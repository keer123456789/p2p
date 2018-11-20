package p2p

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/DSiSc/craft/log"
	"github.com/DSiSc/p2p/common"
	"github.com/DSiSc/p2p/message"
	"github.com/DSiSc/p2p/version"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	MAX_BUF_LEN    = 1024 * 256 //the maximum buffer to receive message
	WRITE_DEADLINE = 5          //deadline of conn write
)

// The peer state
type PeerStatus int

const (
	INIT        = PeerStatus(iota) //initial
	HAND                           //send verion to peer
	HAND_SHAKE                     //haven`t send verion to peer and receive peer`s version
	HAND_SHAKED                    //send verion to peer and receive peer`s version
	ESTABLISH                      //receive peer`s verack
	STOPPED
)

// Peer represent the peer
type Peer struct {
	version    uint32
	outBound   bool
	persistent bool
	addr       *common.NetAddress
	state      uint64   //current state of this peer
	conn       net.Conn //connection to this peer
	sendChan   chan *internalMsg
	recvChan   chan<- *internalMsg
	quitChan   chan interface{}
	lock       sync.RWMutex
	status     PeerStatus
}

// NewInboundPeer new inbound peer instance
func NewInboundPeer(addr *common.NetAddress, msgChan chan<- *internalMsg, conn net.Conn) *Peer {
	return newPeer(addr, false, false, msgChan, conn)
}

// NewInboundPeer new outbound peer instance
func NewOutboundPeer(addr *common.NetAddress, persistent bool, msgChan chan<- *internalMsg) *Peer {
	return newPeer(addr, true, persistent, msgChan, nil)
}

// create a peer instance.
func newPeer(addr *common.NetAddress, outBound, persistent bool, msgChan chan<- *internalMsg, conn net.Conn) *Peer {
	return &Peer{
		addr:       addr,
		outBound:   outBound,
		persistent: persistent,
		status:     INIT,
		sendChan:   make(chan *internalMsg),
		recvChan:   msgChan,
		quitChan:   make(chan interface{}),
		conn:       conn,
	}
}

// Start connect to peer and send message to each other
func (peer *Peer) Start() error {
	log.Info("Start peer %s", peer.GetAddr().ToString())
	peer.lock.Lock()
	defer peer.lock.Unlock()
	if peer.status != INIT {
		return errors.New("peer has been started")
	}

	if peer.outBound {
		err := peer.initConn()
		if err != nil {
			return err
		}
		err = peer.sendVersionMsg()
		if err != nil {
			log.Error("failed to send version message")
			return err
		}
		peer.status = HAND
	} else {
		if peer.conn == nil {
			return errors.New("have no established connection")
		}
	}
	go peer.recvHandler()
	go peer.sendHandler()
	return nil
}

// Stop stop peer.
func (peer *Peer) Stop() {
	log.Info("Stop peer %s", peer.GetAddr().ToString())

	peer.lock.Lock()
	defer peer.lock.Unlock()
	if peer.conn != nil {
		peer.conn.Close()
	}
	close(peer.quitChan)
	peer.status = STOPPED
}

// initConnection init the connection to peer.
func (peer *Peer) initConn() error {
	dialAddr := peer.addr.IP + ":" + strconv.Itoa(int(peer.addr.Port))
	conn, err := net.Dial("tcp", dialAddr)
	if err != nil {
		log.Error("failed to dial to peer %s, as : %v", peer.GetAddr(), err)
		return fmt.Errorf("failed to dial to peer %s, as : %v", peer.GetAddr().ToString(), err)
	}
	peer.conn = conn
	return nil
}

// send version message to remote
func (peer *Peer) sendVersionMsg() error {
	vmsg := &message.Version{
		Version: version.Version,
	}
	err := peer.connSendMessage(vmsg)
	if err != nil {
		log.Error("failed to send version message")
		return err
	}
	return nil
}

// read versin mesage from remote
func (peer *Peer) handleVersionMsg(vmsg *message.Version) error {
	peer.lock.Lock()
	if peer.status != INIT && peer.status != HAND {
		return fmt.Errorf("unknown status to received version,%d,%s\n", peer.status, peer.addr.ToString())
	}
	var msg message.Message
	if peer.status == INIT {
		peer.status = HAND_SHAKE
		msg = &message.Version{
			Version: version.Version,
		}
	} else {
		peer.status = HAND_SHAKED
		msg = &message.VersionAck{
			Version: version.Version,
		}
	}
	peer.lock.Unlock()
	return peer.connSendMessage(msg)
}

// read versin mesage from remote
func (peer *Peer) handleVersionAckMsg(vackmsg *message.VersionAck) error {
	peer.lock.Lock()
	defer peer.lock.Unlock()
	if peer.status != HAND_SHAKE && peer.status != HAND_SHAKED {
		return fmt.Errorf("unknown status to received verAck,state:%d,%s\n", peer.status, peer.addr.ToString())
	}
	peer.status = ESTABLISH
	if peer.status == HAND_SHAKE {
		vmsg := &message.VersionAck{
			Version: version.Version,
		}
		err := peer.connSendMessage(vmsg)
		if err != nil {
			return err
		}
	}
	addReqMsg := &message.AddrReq{}
	go peer.connSendMessage(addReqMsg)
	return nil
}

// message receive handler
func (peer *Peer) recvHandler() {
	reader := bufio.NewReaderSize(peer.conn, MAX_BUF_LEN)
	for {
		// read new message from connection
		msg, err := message.ReadMessage(reader)
		if err != nil {
			log.Error("[p2p]error read from %s, as: %v", peer.GetAddr().ToString(), err)
			peer.disconnectNotify(err)
			return
		}
		switch msg.(type) {
		case *message.Version:
			err := peer.handleVersionMsg(msg.(*message.Version))
			if err != nil {
				reject := &message.RejectMsg{
					Reason: fmt.Sprintf("error: %v", err),
				}
				peer.connSendMessage(reject)
				peer.disconnectNotify(err)
				return
			}
		case *message.VersionAck:
			err := peer.handleVersionAckMsg(msg.(*message.VersionAck))
			if err != nil {
				reject := &message.RejectMsg{
					Reason: fmt.Sprintf("error: %v", err),
				}
				peer.connSendMessage(reject)
				peer.disconnectNotify(err)
				return
			}
		case *message.RejectMsg:
			rejectMsg := msg.(*message.RejectMsg)
			log.Error("receive a reject message from remote, reject reason: %s", rejectMsg.Reason)
			peer.disconnectNotify(errors.New(rejectMsg.Reason))
		default:
			peerStatus := peer.GetStatus()
			if peerStatus != ESTABLISH {
				err := fmt.Errorf("peer %s cannot receive %v type message in %v status", peer.GetAddr().ToString(), msg.MsgType(), peerStatus)
				log.Error("peer %s cannot receive %v type message in [%v] status", peer.GetAddr().ToString(), msg.MsgType(), peerStatus)
				reject := &message.RejectMsg{
					Reason: fmt.Sprintf("error: %v", err),
				}
				peer.connSendMessage(reject)
				peer.disconnectNotify(err)
				return
			}
			imsg := &internalMsg{
				peer.addr,
				nil,
				msg,
				nil,
			}
			peer.recvChan <- imsg
		}
	}
}

// read message from connection
func (peer *Peer) connReadMessage() (message.Message, error) {
	reader := bufio.NewReaderSize(peer.conn, MAX_BUF_LEN)
	return message.ReadMessage(reader)
}

// message send handler
func (peer *Peer) sendHandler() {
	for {
		select {
		case msg := <-peer.sendChan:
			err := peer.connSendMessage(msg.payload)
			if msg.respTo != nil {
				if err != nil {
					msg.respTo <- err
				} else {
					msg.respTo <- nilError
				}
			}
			if err != nil {
				peer.disconnectNotify(err)
			}
		case <-peer.quitChan:
			return
		}
	}
}

// send message to connection.
func (peer *Peer) connSendMessage(msg message.Message) error {
	buf, err := message.EncodeMessage(msg)
	if err != nil {
		log.Error("failed to encode message %v, as %v", msg, err)
		return err
	}

	nCount := len(buf)
	peer.conn.SetWriteDeadline(time.Now().Add(time.Duration(nCount*WRITE_DEADLINE) * time.Second))
	_, err = peer.conn.Write(buf)
	if err != nil {
		log.Error("failed to send raw message to remote, as: %v", err)
		return err
	}
	return nil
}

// IsPersistent return true if this peer is a persistent peer
func (peer *Peer) IsPersistent() bool {
	peer.lock.RLock()
	defer peer.lock.RUnlock()
	return peer.persistent
}

// GetAddr get peer's address
func (peer *Peer) GetAddr() *common.NetAddress {
	peer.lock.RLock()
	defer peer.lock.RUnlock()
	return peer.addr
}

// CurrentState get current state of this peer.
func (peer *Peer) CurrentState() uint64 {
	peer.lock.RLock()
	defer peer.lock.RUnlock()
	return peer.state
}

// Channel get peer's input channel
func (peer *Peer) Channel() chan<- *internalMsg {
	return peer.sendChan
}

// GetStatus get peer's status
func (peer *Peer) GetStatus() PeerStatus {
	peer.lock.RLock()
	defer peer.lock.RUnlock()
	return peer.status
}

// SetState update peer's state
func (peer *Peer) SetState(state uint64) {
	peer.lock.Lock()
	defer peer.lock.Unlock()
	peer.state = state
}

// SetState update peer's state
func (peer *Peer) GetState() uint64 {
	peer.lock.RLock()
	defer peer.lock.RUnlock()
	return peer.state
}

//disconnectNotify push disconnect msg to channel
func (peer *Peer) disconnectNotify(err error) {
	log.Debug("[p2p]call disconnectNotify for %s, as: %v", peer.GetAddr(), err)
	disconnectMsg := &peerDisconnecMsg{
		err,
	}
	msg := &internalMsg{
		peer.addr,
		nil,
		disconnectMsg,
		nil,
	}
	peer.recvChan <- msg
}

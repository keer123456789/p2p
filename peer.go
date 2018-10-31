package p2p

import (
	"bufio"
	"fmt"
	"github.com/DSiSc/craft/log"
	"github.com/DSiSc/p2p/common"
	"github.com/DSiSc/p2p/message"
	"github.com/DSiSc/p2p/version"
	"github.com/pkg/errors"
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
	INIT      = PeerStatus(iota) //initial
	ESTABLISH                    //receive peer`s verack
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
	peer.lock.Lock()
	defer peer.lock.Unlock()
	if peer.status != INIT {
		return errors.New("peer has started")
	}

	if peer.outBound {
		err := peer.initConn()
		if err != nil {
			return err
		}
		err = peer.negotiateOutBound()
		if err != nil {
			return err
		}
	} else {
		if peer.conn == nil {
			return errors.New("have no established connection")
		}
		err := peer.negotiateInBound()
		if err != nil {
			return err
		}
	}

	err := peer.connSendMessage(&message.VersionAck{})
	if err != nil {
		return err
	}

	peer.status = ESTABLISH
	go peer.recvHandler()
	go peer.sendHandler()
	return nil
}

// Stop stop peer.
func (peer *Peer) Stop() {
	if peer.conn != nil {
		peer.conn.Close()
	}
	peer.setStatus(STOPPED)
	close(peer.quitChan)
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

// negotiate outbound peer
func (peer *Peer) negotiateOutBound() error {
	err := peer.sendVersionMsg()
	if err != nil {
		log.Error("failed to send version message")
		return err
	}
	return peer.readVersionMsg()
}

// negotiate inbound peer
func (peer *Peer) negotiateInBound() error {
	err := peer.readVersionMsg()
	if err != nil {
		log.Error("failed to read version message")
		return err
	}
	return peer.sendVersionMsg()
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
func (peer *Peer) readVersionMsg() error {
	msg, err := peer.connReadMessage()
	if err != nil {
		log.Error("failed to receive remote version message")
		return err
	}
	rvmsg, ok := msg.(*message.Version)
	if !ok {
		log.Error("a version message must precede all others")
		return errors.New("a version message must precede all others")
	}
	if !version.Accept(rvmsg.Version) {
		log.Error("unacceptable version peer")
		return errors.New("unacceptable version peer")
	}
	return nil
}

// message receive handler
func (peer *Peer) recvHandler() {
	for {
		// read new message from connection
		msg, err := peer.connReadMessage()
		if err != nil {
			log.Error("[p2p]error read from %s, as: %v", peer.GetAddr().ToString(), err)
			peer.disconnectNotify(err)
			return
		}
		switch msg.(type) {
		case *message.Version:
			reject := &message.RejectMsg{
				Reason: "duplicate version message",
			}
			peer.internalSendMessage(reject, nil, nil)
			peer.disconnectNotify(fmt.Errorf("received duplicate version message"))
			return
		case *message.VersionAck:
			if peer.GetStatus() != ESTABLISH {
				peer.setStatus(ESTABLISH)
			} else {
				reject := &message.RejectMsg{
					Reason: "duplicate version ack message",
				}
				peer.internalSendMessage(reject, nil, nil)
				peer.disconnectNotify(fmt.Errorf("received duplicate version ack message"))
				return
			}
		case *message.RejectMsg:
			rejectMsg := msg.(*message.RejectMsg)
			log.Error("receive a reject message from remote, reject reason: %s", rejectMsg.Reason)
			peer.disconnectNotify(errors.New(rejectMsg.Reason))
		default:
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
				msg.respTo <- err
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

// send message internally
func (peer *Peer) internalSendMessage(msg message.Message, to *common.NetAddress, respTo chan interface{}) {
	imsg := &internalMsg{
		peer.addr,
		nil,
		msg,
		respTo,
	}
	peer.sendChan <- imsg
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
	peer.lock.Lock()
	defer peer.lock.Unlock()
	return peer.status
}

// set peer's status
func (peer *Peer) setStatus(status PeerStatus) {
	peer.lock.Lock()
	defer peer.lock.Unlock()
	peer.status = status
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
	log.Debug("[p2p]call disconnectNotify for %s", peer.GetAddr())
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

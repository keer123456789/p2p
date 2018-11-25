package p2p

import (
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

// Peer represent the peer
type Peer struct {
	version      uint32
	outBound     bool
	persistent   bool
	serverAddr   *common.NetAddress
	addr         *common.NetAddress
	state        uint64    //current state of this peer
	conn         *PeerConn //connection to this peer
	internalChan chan message.Message
	sendChan     chan *internalMsg
	recvChan     chan<- *internalMsg
	quitChan     chan interface{}
	lock         sync.RWMutex
	isRunning    bool
}

// NewInboundPeer new inbound peer instance
func NewInboundPeer(serverAddr, addr *common.NetAddress, msgChan chan<- *internalMsg, conn net.Conn) *Peer {
	return newPeer(serverAddr, addr, false, false, msgChan, conn)
}

// NewInboundPeer new outbound peer instance
func NewOutboundPeer(serverAddr, addr *common.NetAddress, persistent bool, msgChan chan<- *internalMsg) *Peer {
	return newPeer(serverAddr, addr, true, persistent, msgChan, nil)
}

// create a peer instance.
func newPeer(serverAddr, addr *common.NetAddress, outBound, persistent bool, msgChan chan<- *internalMsg, conn net.Conn) *Peer {
	peer := &Peer{
		serverAddr:   serverAddr,
		addr:         addr,
		outBound:     outBound,
		persistent:   persistent,
		internalChan: make(chan message.Message),
		sendChan:     make(chan *internalMsg),
		recvChan:     msgChan,
		quitChan:     make(chan interface{}),
	}
	if !outBound && conn != nil {
		peer.conn = NewPeerConn(conn, peer.internalChan)
	}
	return peer
}

// Start connect to peer and send message to each other
func (peer *Peer) Start() error {
	peer.lock.Lock()
	defer peer.lock.Unlock()
	if peer.isRunning {
		log.Error("peer %s has been started", peer.addr.ToString())
		return fmt.Errorf("peer %s has been started", peer.addr.ToString())
	}

	if peer.outBound {
		log.Info("Start outbound peer %s", peer.addr.ToString())
		err := peer.initConn()
		if err != nil {
			return err
		}
		peer.conn.Start()
		err = peer.handShakeWithOutBoundPeer()
		if err != nil {
			peer.conn.Stop()
			return err
		}
	} else {
		log.Info("Start inbound peer %s", peer.addr.ToString())
		if peer.conn == nil {
			return errors.New("have no established connection")
		}
		peer.conn.Start()
		err := peer.handShakeWithInBoundPeer()
		if err != nil {
			peer.conn.Stop()
			return err
		}
	}
	go peer.recvHandler()
	go peer.sendHandler()
	return nil
}

// start handshake with outbound peer.
func (peer *Peer) handShakeWithOutBoundPeer() error {
	//send version message
	err := peer.sendVersionMessage()
	if err != nil {
		return err
	}

	// read version message
	err = peer.readVersionMessage()
	if err != nil {
		return err
	}

	// send version ack message
	err = peer.sendVersionAckMessage()
	if err != nil {
		return err
	}

	// read version ack message
	return peer.readVersionAckMessage()
}

// start handshake with inbound peer.
func (peer *Peer) handShakeWithInBoundPeer() error {
	// read version message
	err := peer.readVersionMessage()
	if err != nil {
		return err
	}

	//send version message
	err = peer.sendVersionMessage()
	if err != nil {
		return err
	}

	// read version ack message
	err = peer.readVersionAckMessage()
	if err != nil {
		return err
	}

	// send version ack message
	return peer.sendVersionAckMessage()
}

// send version message to this peer.
func (peer *Peer) sendVersionMessage() error {
	vmsg := &message.Version{
		Version: version.Version,
		PortMe:  peer.serverAddr.Port,
	}
	return peer.conn.SendMessage(vmsg)
}

// send version ack message to this peer.
func (peer *Peer) sendVersionAckMessage() error {
	vackmsg := &message.VersionAck{}
	return peer.conn.SendMessage(vackmsg)
}

// read version message
func (peer *Peer) readVersionMessage() error {
	msg, err := peer.readMessageWithType(message.VERSION_TYPE)
	if err != nil {
		return err
	}
	if !peer.outBound {
		vmsg := msg.(*message.Version)
		peer.addr.Port = vmsg.PortMe
	}
	return nil
}

// read version ack message
func (peer *Peer) readVersionAckMessage() error {
	_, err := peer.readMessageWithType(message.VERACK_TYPE)
	if err != nil {
		return err
	}
	return nil
}

// read specified type message from peer.
func (peer *Peer) readMessageWithType(msgType message.MessageType) (message.Message, error) {
	timer := time.NewTicker(5 * time.Second)
	select {
	case msg := <-peer.internalChan:
		if msg.MsgType() == msgType {
			return msg, nil
		} else {
			log.Error("error type message received from peer %s, expected: %v, actual: %v", peer.addr.ToString(), msgType, msg.MsgType())
			return nil, fmt.Errorf("error type message received from peer %s, expected: %v, actual: %v", peer.addr.ToString(), msgType, msg.MsgType())
		}
	case <-timer.C:
		log.Error("read %v type message from peer %s time out", msgType, peer.addr.ToString())
		return nil, fmt.Errorf("read %v type message from peer %s time out", msgType, peer.addr.ToString())
	}
}

// Stop stop peer.
func (peer *Peer) Stop() {
	log.Info("Stop peer %s", peer.GetAddr().ToString())

	peer.lock.Lock()
	defer peer.lock.Unlock()
	if peer.conn != nil {
		peer.conn.Stop()
	}
	close(peer.quitChan)
	peer.isRunning = false
}

// initConnection init the connection to peer.
func (peer *Peer) initConn() error {
	log.Debug("start init the connection to peer %s", peer.addr.ToString())
	dialAddr := peer.addr.IP + ":" + strconv.Itoa(int(peer.addr.Port))
	conn, err := net.Dial("tcp", dialAddr)
	if err != nil {
		log.Error("failed to dial to peer %s, as : %v", peer.addr.ToString(), err)
		return fmt.Errorf("failed to dial to peer %s, as : %v", peer.addr.ToString(), err)
	}
	peer.conn = NewPeerConn(conn, peer.internalChan)
	return nil
}

// message receive handler
func (peer *Peer) recvHandler() {
	for {
		var msg message.Message
		select {
		case msg = <-peer.internalChan:
			log.Debug("receive %v type message from peer %s", msg.MsgType(), peer.GetAddr().ToString())
		case <-peer.quitChan:
			return
		}

		switch msg.(type) {
		case *message.Version:
			reject := &message.RejectMsg{
				Reason: "invalid message, as version messages can only be sent once ",
			}
			peer.conn.SendMessage(reject)
			peer.disconnectNotify(errors.New("receive an invalid message from remote"))
			return
		case *message.VersionAck:
			reject := &message.RejectMsg{
				Reason: "invalid message, as version ack messages can only be sent once ",
			}
			peer.conn.SendMessage(reject)
			peer.disconnectNotify(errors.New("receive an invalid message from remote"))
			return
		case *message.RejectMsg:
			rejectMsg := msg.(*message.RejectMsg)
			log.Error("receive a reject message from remote, reject reason: %s", rejectMsg.Reason)
			peer.disconnectNotify(errors.New(rejectMsg.Reason))
			return
		default:
			imsg := &internalMsg{
				from:    peer.addr,
				to:      peer.serverAddr,
				payload: msg,
			}
			peer.recvChan <- imsg
			log.Debug("peer %s send %v type message to message channel", peer.GetAddr().ToString(), msg.MsgType())
		}
	}
}

// message send handler
func (peer *Peer) sendHandler() {
	for {
		select {
		case msg := <-peer.sendChan:
			err := peer.conn.SendMessage(msg.payload)
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

// Channel get peer's send channel
func (peer *Peer) Channel() chan<- *internalMsg {
	return peer.sendChan
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
	log.Debug("[p2p]call disconnectNotify for %s, as: %v", peer.GetAddr().ToString(), err)
	disconnectMsg := &peerDisconnecMsg{
		err,
	}
	msg := &internalMsg{
		from:    peer.addr,
		to:      peer.serverAddr,
		payload: disconnectMsg,
	}
	peer.recvChan <- msg
}

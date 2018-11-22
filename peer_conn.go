package p2p

import (
	"bufio"
	"github.com/DSiSc/craft/log"
	"github.com/DSiSc/p2p/message"
	"net"
	"sync"
	"time"
)

// PeerConn is the abstract of the net.Conn to this peer.
type PeerConn struct {
	conn     net.Conn //connection to this peer
	recvChan chan message.Message
	quitChan chan interface{}
	lock     sync.RWMutex
}

// NewPeerConn create a PeerConn instance
func NewPeerConn(conn net.Conn, recvChan chan message.Message) *PeerConn {
	return &PeerConn{
		conn:     conn,
		recvChan: recvChan,
		quitChan: make(chan interface{}),
	}
}

// Start start PeerConn
// will start receive and send handler to handle the message from/to net.Conn
func (peerConn *PeerConn) Start() {
	go peerConn.recvHandler()
}

// Stop stop PeerConn
func (peerConn *PeerConn) Stop() {
	peerConn.conn.Close()
	close(peerConn.quitChan)
}

// message receive handler
func (peerConn *PeerConn) recvHandler() {
	reader := bufio.NewReaderSize(peerConn.conn, MAX_BUF_LEN)
	for {
		// read new message from connection
		msg, err := message.ReadMessage(reader)
		if err != nil {
			log.Error("failed to read message from remote %s, as: %v", peerConn.conn.RemoteAddr().String(), err)
			peerConn.disconnectNotify(err)
			return
		}
		peerConn.recvChan <- msg
	}
}

// SendMessage message to this PeerConn.
func (peerConn *PeerConn) SendMessage(msg message.Message) error {
	log.Debug("send %v type message to remote %s", msg.MsgType(), peerConn.conn.RemoteAddr().String())
	buf, err := message.EncodeMessage(msg)
	if err != nil {
		log.Error("failed to encode message %v, as %v", msg, err)
		return err
	}

	nCount := len(buf)
	peerConn.conn.SetWriteDeadline(time.Now().Add(time.Duration(nCount*WRITE_DEADLINE) * time.Second))
	_, err = peerConn.conn.Write(buf)
	if err != nil {
		log.Error("failed to send raw message to remote %s, as: %v", peerConn.conn.RemoteAddr().String(), err)
		return err
	}
	return nil
}

//disconnectNotify push disconnect msg to channel
func (peerConn *PeerConn) disconnectNotify(err error) {
	log.Debug("call disconnectNotify for %s, as: %v", peerConn.conn.RemoteAddr().String(), err)
	disconnectMsg := &peerDisconnecMsg{
		err,
	}
	peerConn.recvChan <- disconnectMsg
}

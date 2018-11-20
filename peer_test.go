package p2p

import (
	"errors"
	"github.com/DSiSc/craft/log"
	"github.com/DSiSc/monkey"
	"github.com/DSiSc/p2p/common"
	"github.com/DSiSc/p2p/message"
	"github.com/DSiSc/p2p/version"
	"github.com/stretchr/testify/assert"
	"net"
	"sync"
	"testing"
	"time"
)

func mockAddress() *common.NetAddress {
	addr := common.NetAddress{
		Protocol: "tcp",
		IP:       "192.168.1.1",
		Port:     8080,
	}
	return &addr
}

func TestNewInboundPeer(t *testing.T) {
	assert := assert.New(t)
	msgChan := make(chan *internalMsg)
	peer := NewInboundPeer(mockAddress(), msgChan, &testConn{})
	assert.NotNil(peer)
	assert.False(peer.outBound)
}

func TestNewOutboundPeer(t *testing.T) {
	assert := assert.New(t)
	msgChan := make(chan *internalMsg)
	peer := NewOutboundPeer(mockAddress(), true, msgChan)
	assert.NotNil(peer)
	assert.True(peer.persistent)
	assert.True(peer.outBound)
}

func TestPeer_Start(t *testing.T) {
	defer monkey.UnpatchAll()

	assert := assert.New(t)

	// mock version message
	msgByte, _ := message.EncodeMessage(&message.Version{
		Version: version.Version,
	})
	connBytes := msgByte

	// mock version ack
	msgByte, _ = message.EncodeMessage(&message.VersionAck{
		Version: version.Version,
	})
	connBytes = append(connBytes, msgByte...)

	// mock address message
	msgByte, _ = message.EncodeMessage(&message.Addr{
		NetAddresses: make([]*common.NetAddress, 0),
	})

	netConn := newTestConn(connBytes)

	// start inbound peer
	msgChan := make(chan *internalMsg)
	peer := NewInboundPeer(mockAddress(), msgChan, netConn)
	assert.NotNil(peer)
	err := peer.Start()
	assert.Nil(err)

	timer := time.NewTicker(2 * time.Second)
	select {
	case <-msgChan:
	case <-timer.C:
		assert.Nil(errors.New("failed to receive heart beat message"))
	}

	// test stop peer
	peer.Stop()
	assert.Equal(STOPPED, peer.GetStatus())
	select {
	case <-peer.quitChan:
	default:
		assert.Nil(errors.New("failed to stop peer"))
	}
}

func TestPeer_Start1(t *testing.T) {
	defer monkey.UnpatchAll()
	assert := assert.New(t)
	// mock version message
	msgByte, _ := message.EncodeMessage(&message.Version{
		Version: version.Version,
	})
	connBytes := msgByte

	// mock version ack
	msgByte, _ = message.EncodeMessage(&message.VersionAck{
		Version: version.Version,
	})
	connBytes = append(connBytes, msgByte...)

	// mock address message
	msgByte, _ = message.EncodeMessage(&message.Addr{
		NetAddresses: make([]*common.NetAddress, 0),
	})
	netConn := newTestConn(connBytes)

	// mock dial to remote server
	monkey.Patch(net.Dial, func(network, address string) (net.Conn, error) {
		return netConn, nil
	})

	// start outbound peer
	msgChan := make(chan *internalMsg)
	peer := NewOutboundPeer(mockAddress(), false, msgChan)
	assert.NotNil(peer)
	err := peer.Start()
	assert.Nil(err)

	timer := time.NewTicker(5 * time.Second)
	select {
	case <-msgChan:
	case <-timer.C:
		assert.Nil(errors.New("failed to receive heart beat message"))
	}

	// test stop peer
	peer.Stop()
	assert.Equal(STOPPED, peer.GetStatus())
	select {
	case <-peer.quitChan:
	default:
		assert.Nil(errors.New("failed to stop peer"))
	}
}

func TestPeer_IsPersistent(t *testing.T) {
	assert := assert.New(t)
	msgChan := make(chan *internalMsg)
	peer := NewOutboundPeer(mockAddress(), true, msgChan)
	assert.NotNil(peer)
	assert.True(peer.IsPersistent())
}

func TestPeer_GetAddr(t *testing.T) {
	assert := assert.New(t)
	msgChan := make(chan *internalMsg)
	addr := mockAddress()
	peer := NewOutboundPeer(addr, true, msgChan)
	assert.NotNil(peer)
	assert.True(addr.Equal(peer.GetAddr()))
}

func TestPeer_CurrentState(t *testing.T) {
	assert := assert.New(t)
	msgChan := make(chan *internalMsg)
	peer := NewOutboundPeer(mockAddress(), true, msgChan)
	assert.NotNil(peer)
	assert.Equal(uint64(0), peer.CurrentState())
}

func TestPeer_Channel(t *testing.T) {
	assert := assert.New(t)

	// mock version message
	msgByte, _ := message.EncodeMessage(&message.Version{
		Version: version.Version,
	})
	connBytes := msgByte

	// mock version ack
	msgByte, _ = message.EncodeMessage(&message.VersionAck{
		Version: version.Version,
	})
	connBytes = append(connBytes, msgByte...)

	// mock address message
	msgByte, _ = message.EncodeMessage(&message.Addr{
		NetAddresses: make([]*common.NetAddress, 0),
	})
	connBytes = append(connBytes, msgByte...)
	netConn := newTestConn(connBytes)

	// start inbound peer
	msgChan := make(chan *internalMsg)
	peer := NewInboundPeer(mockAddress(), msgChan, netConn)
	assert.NotNil(peer)
	err := peer.Start()
	assert.Nil(err)

	// send message
	respChan := make(chan interface{})
	sendMsg := &internalMsg{
		from: nil,
		to:   peer.GetAddr(),
		payload: &message.PingMsg{
			State: 1,
		},
		respTo: respChan,
	}
	peer.Channel() <- sendMsg
	time.Sleep(time.Second)
	select {
	case err := <-respChan:
		if err != nilError {
			assert.Nil(err)
		}
	default:
		assert.Nil(errors.New("failed to send message"))
	}

	// test stop peer
	peer.Stop()
	assert.Equal(STOPPED, peer.GetStatus())
	select {
	case <-peer.quitChan:
	default:
		assert.Nil(errors.New("failed to stop peer"))
	}
}

func TestPeer_GetStatus(t *testing.T) {
	assert := assert.New(t)
	msgChan := make(chan *internalMsg)
	peer := NewInboundPeer(mockAddress(), msgChan, &testConn{})
	assert.NotNil(peer)
	assert.Equal(INIT, peer.GetStatus())
}

func TestPeer_SetState(t *testing.T) {
	assert := assert.New(t)
	msgChan := make(chan *internalMsg)
	peer := NewInboundPeer(mockAddress(), msgChan, &testConn{})
	assert.NotNil(peer)

	peer.SetState(64)
	assert.Equal(uint64(64), peer.GetState())
}

type testConn struct {
	mockMsg      []byte
	internalChan chan []byte
	lock         sync.RWMutex
}

func newTestConn(msgs []byte) *testConn {
	return &testConn{
		mockMsg:      msgs,
		internalChan: make(chan []byte),
	}
}

func (this *testConn) Read(b []byte) (n int, err error) {
	time.Sleep(100 * time.Millisecond)
	this.lock.Lock()
	if len(this.mockMsg) == 0 {
		msgByte, _ := message.EncodeMessage(&message.PongMsg{
			State: 1,
		})
		this.mockMsg = msgByte
	}
	if len(b) >= len(this.mockMsg) {
		n = len(this.mockMsg)
		copy(b, this.mockMsg[:])
		this.mockMsg = make([]byte, 0)
	} else {
		n = len(b)
		copy(b, this.mockMsg[:n])
		this.mockMsg = this.mockMsg[n+1:]
	}
	this.lock.Unlock()
	return
}

func (this *testConn) Write(b []byte) (n int, err error) {
	log.Info("Connection write")
	return 0, nil
}

func (this *testConn) Close() error {
	return nil
}

func (this *testConn) LocalAddr() net.Addr {
	return nil
}

func (this *testConn) RemoteAddr() net.Addr {
	return nil
}

func (this *testConn) SetDeadline(t time.Time) error {
	return nil
}

func (this *testConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (this *testConn) SetWriteDeadline(t time.Time) error {
	return nil
}

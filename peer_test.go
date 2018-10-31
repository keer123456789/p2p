package p2p

import (
	"github.com/DSiSc/monkey"
	"github.com/DSiSc/p2p/common"
	"github.com/DSiSc/p2p/message"
	"github.com/DSiSc/p2p/version"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"net"
	"reflect"
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
	assert := assert.New(t)
	netConn := newTestConn()
	// mock connection read message
	monkey.PatchInstanceMethod(reflect.TypeOf(netConn), "Read", func(conn *testConn, b []byte) (n int, err error) {
		msg := <-conn.internalChan
		copy(b, msg[:])
		return len(msg), nil
	})
	// mock version message
	msgByte, _ := message.EncodeMessage(&message.Version{
		Version: version.Version,
	})
	go func() {
		netConn.internalChan <- msgByte
	}()

	// start inbound peer
	msgChan := make(chan *internalMsg)
	peer := NewInboundPeer(mockAddress(), msgChan, netConn)
	assert.NotNil(peer)
	err := peer.Start()
	assert.Nil(err)

	// test stop peer
	peer.Stop()
	select {
	case <-peer.quitChan:
	default:
		assert.Error(errors.New("failed to stop peer"))
	}
}

func TestPeer_Start1(t *testing.T) {
	assert := assert.New(t)
	netConn := newTestConn()
	// mock connection read message
	monkey.PatchInstanceMethod(reflect.TypeOf(netConn), "Read", func(conn *testConn, b []byte) (n int, err error) {
		msg := <-conn.internalChan
		copy(b, msg[:])
		return len(msg), nil
	})
	// mock dial to remote server
	monkey.Patch(net.Dial, func(network, address string) (net.Conn, error) {
		return netConn, nil
	})

	// mock version message
	msgByte, _ := message.EncodeMessage(&message.Version{
		Version: version.Version,
	})
	go func() {
		netConn.internalChan <- msgByte
	}()

	// start outbound peer
	msgChan := make(chan *internalMsg)
	peer := NewOutboundPeer(mockAddress(), false, msgChan)
	assert.NotNil(peer)
	err := peer.Start()
	assert.Nil(err)

	// test stop peer
	peer.Stop()
	select {
	case <-peer.quitChan:
	default:
		assert.Error(errors.New("failed to stop peer"))
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
	netConn := newTestConn()
	// mock connection read message
	monkey.PatchInstanceMethod(reflect.TypeOf(netConn), "Read", func(conn *testConn, b []byte) (n int, err error) {
		msg := <-conn.internalChan
		copy(b, msg[:])
		return len(msg), nil
	})

	// mock version message
	go func() {
		msgByte, _ := message.EncodeMessage(&message.Version{
			Version: version.Version,
		})
		netConn.internalChan <- msgByte
	}()

	// start inbound peer
	msgChan := make(chan *internalMsg)
	peer := NewInboundPeer(mockAddress(), msgChan, netConn)
	assert.NotNil(peer)
	err := peer.Start()
	assert.Nil(err)

	// send message to send channel
	respChan := make(chan interface{})
	sendMsg := &internalMsg{
		from: nil,
		to:   peer.GetAddr(),
		payload: &message.PingMsg{
			State: 1,
		},
		respTo: respChan,
	}
	go func() {
		peer.Channel() <- sendMsg
	}()

	// check send response
	timer := time.NewTicker(time.Second)
	select {
	case err := <-respChan:
		if err != nil {
			assert.Error(err.(error))
		}
	case <-timer.C:
		assert.Error(errors.New("Send time out"))
	}

	// mock connection send message
	connChan := make(chan interface{})
	monkey.PatchInstanceMethod(reflect.TypeOf(netConn), "Write", func(conn *testConn, b []byte) (n int, err error) {
		connChan <- "Success"
		return len(b), nil
	})
	// check connection send result
	select {
	case result := <-connChan:
		assert.Equal("Seccess", result)
	case <-timer.C:
		assert.Error(errors.New("failed to send message to connection"))
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
	internalChan chan []byte
}

func newTestConn() *testConn {
	return &testConn{
		internalChan: make(chan []byte),
	}
}

func (this *testConn) Read(b []byte) (n int, err error) {
	return 0, nil
}

func (this *testConn) Write(b []byte) (n int, err error) {
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

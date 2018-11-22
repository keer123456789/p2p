package p2p

import (
	"errors"
	"fmt"
	"github.com/DSiSc/monkey"
	"github.com/DSiSc/p2p/common"
	"github.com/DSiSc/p2p/message"
	"github.com/DSiSc/p2p/version"
	"github.com/stretchr/testify/assert"
	"net"
	"reflect"
	"testing"
	"time"
)

func mockServerAddress() *common.NetAddress {
	addr := common.NetAddress{
		Protocol: "tcp",
		IP:       "192.168.1.100",
		Port:     8080,
	}
	return &addr
}

func mockAddress() *common.NetAddress {
	addr := common.NetAddress{
		Protocol: "tcp",
		IP:       "192.168.1.101",
		Port:     8080,
	}
	return &addr
}

func mockPeerConn() *PeerConn {
	peerConn := NewPeerConn(nil, make(chan message.Message))
	monkey.PatchInstanceMethod(reflect.TypeOf(peerConn), "Start", func(peerConn *PeerConn) {})
	monkey.PatchInstanceMethod(reflect.TypeOf(peerConn), "Stop", func(peerConn *PeerConn) {})
	monkey.PatchInstanceMethod(reflect.TypeOf(peerConn), "SendMessage", func(peerConn *PeerConn, msg message.Message) error { return nil })
	return peerConn
}

func TestNewInboundPeer(t *testing.T) {
	assert := assert.New(t)
	msgChan := make(chan *internalMsg)
	peer := NewInboundPeer(mockServerAddress(), mockAddress(), msgChan, &testConn{})
	assert.NotNil(peer)
	assert.False(peer.outBound)
}

func TestNewOutboundPeer(t *testing.T) {
	assert := assert.New(t)
	msgChan := make(chan *internalMsg)
	peer := NewOutboundPeer(mockServerAddress(), mockAddress(), true, msgChan)
	assert.NotNil(peer)
	assert.True(peer.persistent)
	assert.True(peer.outBound)
}

func TestPeer_Start(t *testing.T) {
	defer monkey.UnpatchAll()

	assert := assert.New(t)

	msgs := []message.Message{
		&message.Version{
			Version:    version.Version,
			NetAddress: mockAddress(),
		},
		&message.VersionAck{},
		&message.Addr{
			NetAddresses: make([]*common.NetAddress, 0),
		},
	}

	peerConn := mockPeerConn()
	monkey.Patch(NewPeerConn, func(conn net.Conn, recvChan chan message.Message) *PeerConn { return peerConn })
	// start inbound peer
	msgChan := make(chan *internalMsg)
	peer := NewInboundPeer(mockServerAddress(), mockAddress(), msgChan, newTestConn())
	assert.NotNil(peer)
	// mock receive message from peerConn
	go func(msgs []message.Message) {
		for _, msg := range msgs {
			peer.internalChan <- msg
		}
	}(msgs)
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
	select {
	case <-peer.quitChan:
	default:
		assert.Nil(errors.New("failed to stop peer"))
	}
}

func TestPeer_Start1(t *testing.T) {
	defer monkey.UnpatchAll()

	assert := assert.New(t)

	msgs := []message.Message{
		&message.Version{
			Version:    version.Version,
			NetAddress: mockAddress(),
		},
		&message.VersionAck{},
		&message.Addr{
			NetAddresses: make([]*common.NetAddress, 0),
		},
	}
	monkey.Patch(net.Dial, func(network, address string) (net.Conn, error) { return newTestConn(), nil })
	peerConn := mockPeerConn()
	monkey.Patch(NewPeerConn, func(conn net.Conn, recvChan chan message.Message) *PeerConn { return peerConn })
	// start outbound peer
	msgChan := make(chan *internalMsg)
	peer := NewOutboundPeer(mockServerAddress(), mockAddress(), false, msgChan)

	// mock receive message from peerConn
	go func(msgs []message.Message) {
		for _, msg := range msgs {
			peer.internalChan <- msg
		}
	}(msgs)

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
	select {
	case <-peer.quitChan:
	default:
		assert.Nil(errors.New("failed to stop peer"))
	}
}

func TestPeer_IsPersistent(t *testing.T) {
	assert := assert.New(t)
	msgChan := make(chan *internalMsg)
	peer := NewOutboundPeer(mockServerAddress(), mockAddress(), true, msgChan)
	assert.NotNil(peer)
	assert.True(peer.IsPersistent())
}

func TestPeer_GetAddr(t *testing.T) {
	assert := assert.New(t)
	msgChan := make(chan *internalMsg)
	addr := mockAddress()
	peer := NewOutboundPeer(mockServerAddress(), addr, true, msgChan)
	assert.NotNil(peer)
	assert.True(addr.Equal(peer.GetAddr()))
}

func TestPeer_CurrentState(t *testing.T) {
	assert := assert.New(t)
	msgChan := make(chan *internalMsg)
	peer := NewOutboundPeer(mockServerAddress(), mockAddress(), true, msgChan)
	assert.NotNil(peer)
	assert.Equal(uint64(0), peer.CurrentState())
}

func TestPeer_Channel(t *testing.T) {
	defer monkey.UnpatchAll()

	assert := assert.New(t)
	msgs := []message.Message{
		&message.Version{
			Version:    version.Version,
			NetAddress: mockAddress(),
		},
		&message.VersionAck{},
		&message.Addr{
			NetAddresses: make([]*common.NetAddress, 0),
		},
	}

	// mock peer connection
	peerConn := mockPeerConn()
	monkey.Patch(NewPeerConn, func(conn net.Conn, recvChan chan message.Message) *PeerConn { return peerConn })

	// start inbound peer
	msgChan := make(chan *internalMsg)
	peer := NewInboundPeer(mockServerAddress(), mockAddress(), msgChan, newTestConn())
	assert.NotNil(peer)
	// mock receive message from peerConn
	go func(msgs []message.Message) {
		for _, msg := range msgs {
			peer.internalChan <- msg
		}
	}(msgs)
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
	select {
	case <-peer.quitChan:
	default:
		assert.Nil(errors.New("failed to stop peer"))
	}
}

func TestPeer_SetState(t *testing.T) {
	assert := assert.New(t)
	msgChan := make(chan *internalMsg)
	peer := NewInboundPeer(mockServerAddress(), mockAddress(), msgChan, &testConn{})
	assert.NotNil(peer)

	peer.SetState(64)
	assert.Equal(uint64(64), peer.GetState())
}

type testConn struct {
}

func newTestConn() *testConn {
	return &testConn{}
}

func (this *testConn) Read(b []byte) (n int, err error) {
	fmt.Println("Read")
	return 0, nil
}

func (this *testConn) Write(b []byte) (n int, err error) {
	fmt.Println("Write")
	return 0, nil
}

func (this *testConn) Close() error {
	return nil
}

func (this *testConn) LocalAddr() net.Addr {
	return nil
}

func (this *testConn) RemoteAddr() net.Addr {
	fmt.Println("RemoteAddr")
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

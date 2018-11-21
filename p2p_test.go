package p2p

import (
	"errors"
	"github.com/DSiSc/monkey"
	"github.com/DSiSc/p2p/common"
	"github.com/DSiSc/p2p/config"
	"github.com/DSiSc/p2p/message"
	"github.com/stretchr/testify/assert"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"
)

func mockConfig() *config.P2PConfig {
	return &config.P2PConfig{
		AddrBookFilePath: "",
		ListenAddress:    "tcp://0.0.0.0:8080",
		MaxConnOutBound:  60,
		MaxConnInBound:   20,
		PersistentPeers:  "",
	}
}

func mockPeer(addr *common.NetAddress, outBound, persistent bool, msgChan chan<- *internalMsg, conn net.Conn) *Peer {
	peer := newPeer(addr, outBound, persistent, msgChan, conn)
	monkey.PatchInstanceMethod(reflect.TypeOf(peer), "Start", func(peer *Peer) error {
		return nil
	})
	monkey.PatchInstanceMethod(reflect.TypeOf(peer), "Stop", func(peer *Peer) {
	})
	return peer
}

func TestNewP2P(t *testing.T) {
	assert := assert.New(t)
	conf := mockConfig()
	p2p, err := NewP2P(conf)
	assert.Nil(err)
	assert.NotNil(p2p)
}

func TestP2P_Start(t *testing.T) {
	defer monkey.UnpatchAll()
	assert := assert.New(t)
	conf := mockConfig()
	p2p, err := NewP2P(conf)
	assert.Nil(err)

	// mock listen
	monkey.Patch(net.Listen, func(network, address string) (net.Listener, error) {
		return newTestListener(), nil
	})
	err = p2p.Start()
	assert.Nil(err)
	p2p.Stop()
}

func TestP2P_Stop(t *testing.T) {
	defer monkey.UnpatchAll()
	assert := assert.New(t)
	conf := mockConfig()
	p2p, err := NewP2P(conf)
	assert.Nil(err)

	// mock listen
	monkey.Patch(net.Listen, func(network, address string) (net.Listener, error) {
		return newTestListener(), nil
	})
	err = p2p.Start()
	assert.Nil(err)
	p2p.Stop()
	select {
	case <-p2p.quitChan:
	default:
		assert.Error(errors.New("failed to stop the peer."))
	}
}

func TestP2P_BroadCast(t *testing.T) {
	defer monkey.UnpatchAll()
	assert := assert.New(t)
	conf := mockConfig()
	conf.PersistentPeers = "tcp://192.168.1.1:8080"
	p2p, err := NewP2P(conf)
	assert.Nil(err)
	msg := &message.PingMsg{
		State: 1,
	}

	//mock peer
	addr, _ := common.ParseNetAddress(conf.PersistentPeers)
	peer := mockPeer(addr, true, false, p2p.internalChan, nil)
	monkey.Patch(NewOutboundPeer, func(addr *common.NetAddress, persistent bool, msgChan chan<- *internalMsg) *Peer {
		return peer
	})

	err = p2p.Start()
	assert.Nil(err)

	timer := time.NewTicker(time.Second)
OUT:
	for {
		select {
		case <-timer.C:
			if len(p2p.GetPeers()) > 0 {
				break OUT
			}
		}
	}
	p2p.BroadCast(msg)
	// read message from peer's send channel
	timeoutTricker := time.NewTicker(5 * time.Second)
	var wg sync.WaitGroup
	for _, peer := range p2p.GetPeers() {
		wg.Add(1)
		go func(p *Peer) {
			for {
				select {
				case pmsg := <-p.sendChan:
					switch pmsg.payload.(type) {
					case *message.PingMsg:
						assert.Equal(msg, pmsg.payload)
						wg.Done()
						return
					}
				case <-timeoutTricker.C:
					assert.Nil(errors.New("read sent message failed"))
				}
			}
		}(peer)
	}
	wg.Wait()
	peer.Stop()
}

func TestP2P_SendMsg(t *testing.T) {
	defer monkey.UnpatchAll()
	assert := assert.New(t)
	conf := mockConfig()
	conf.PersistentPeers = "tcp://192.168.1.1:8080"
	p2p, err := NewP2P(conf)
	assert.Nil(err)
	//mock peer
	addr, _ := common.ParseNetAddress(conf.PersistentPeers)
	mockPeer := mockPeer(addr, true, false, p2p.internalChan, nil)
	monkey.Patch(NewOutboundPeer, func(addr *common.NetAddress, persistent bool, msgChan chan<- *internalMsg) *Peer {
		return mockPeer
	})

	// mock listen
	monkey.Patch(net.Listen, func(network, address string) (net.Listener, error) {
		return newTestListener(), nil
	})
	err = p2p.Start()
	assert.Nil(err)

	timeoutTricker := time.NewTicker(5 * time.Second)
	timer := time.NewTicker(time.Second)
OUT:
	for {
		select {
		case <-timer.C:
			if len(p2p.GetPeers()) > 0 {
				break OUT
			}
		case <-timeoutTricker.C:
			assert.Nil(errors.New("failed to connect persistent peer"))
			break OUT
		}
	}
	msg := &message.BlockReq{}
	peer := p2p.GetPeers()[0]
	go func() {
		err := p2p.sendMsg(peer, msg)
		assert.Nil(err)
	}()
	// read message from peer's send channel
OUT1:
	for {
		select {
		case pmsg := <-peer.sendChan:
			switch pmsg.payload.(type) {
			case *message.BlockReq:
				break OUT1
			default:
				continue
			}
		case <-timeoutTricker.C:
			assert.Nil(errors.New("read sent message failed"))
		}
	}
	p2p.Stop()
}

func TestP2P_GetOutBountPeersCount(t *testing.T) {
	defer monkey.UnpatchAll()
	assert := assert.New(t)
	conf := mockConfig()
	conf.PersistentPeers = "tcp://192.168.1.1:8080"

	p2p, err := NewP2P(conf)
	assert.Nil(err)
	assert.Equal(0, p2p.GetOutBountPeersCount())

	//mock peer
	addr, _ := common.ParseNetAddress(conf.PersistentPeers)
	peer := mockPeer(addr, true, false, p2p.internalChan, nil)
	monkey.Patch(NewOutboundPeer, func(addr *common.NetAddress, persistent bool, msgChan chan<- *internalMsg) *Peer {
		return peer
	})

	// mock listen
	monkey.Patch(net.Listen, func(network, address string) (net.Listener, error) {
		return newTestListener(), nil
	})
	err = p2p.Start()
	assert.Nil(err)
	timer := time.NewTicker(time.Second)
OUT:
	for {
		select {
		case <-timer.C:
			if len(p2p.GetPeers()) > 0 {
				break OUT
			}
		}
	}
	assert.Equal(1, p2p.GetOutBountPeersCount())
	p2p.Stop()
}

func TestP2P_GetPeerByAddress(t *testing.T) {
	defer monkey.UnpatchAll()
	assert := assert.New(t)
	conf := mockConfig()
	conf.PersistentPeers = "tcp://192.168.1.1:8080"
	p2p, err := NewP2P(conf)
	assert.Nil(err)

	//mock peer
	addr, _ := common.ParseNetAddress(conf.PersistentPeers)
	peer := mockPeer(addr, true, false, p2p.internalChan, nil)
	monkey.Patch(NewOutboundPeer, func(addr *common.NetAddress, persistent bool, msgChan chan<- *internalMsg) *Peer {
		return peer
	})

	// mock listen
	monkey.Patch(net.Listen, func(network, address string) (net.Listener, error) {
		return newTestListener(), nil
	})

	err = p2p.Start()
	assert.Nil(err)
	timer := time.NewTicker(time.Second)
OUT:
	for {
		select {
		case <-timer.C:
			if len(p2p.GetPeers()) > 0 {
				break OUT
			}
		}
	}
	p2p.Stop()
}

func TestP2P_GetPeers(t *testing.T) {
	defer monkey.UnpatchAll()
	assert := assert.New(t)
	conf := mockConfig()
	conf.PersistentPeers = "tcp://192.168.1.1:8080"
	p2p, err := NewP2P(conf)
	// mock peer
	addr, _ := common.ParseNetAddress(conf.PersistentPeers)
	peer := mockPeer(addr, true, false, p2p.internalChan, nil)
	monkey.Patch(NewInboundPeer, func(addr *common.NetAddress, msgChan chan<- *internalMsg, conn net.Conn) *Peer {
		return peer
	})
	monkey.Patch(NewOutboundPeer, func(addr *common.NetAddress, persistent bool, msgChan chan<- *internalMsg) *Peer {
		return peer
	})

	// mock listen
	monkey.Patch(net.Listen, func(network, address string) (net.Listener, error) {
		return newTestListener(), nil
	})

	assert.Nil(err)
	err = p2p.Start()
	assert.Nil(err)
	timer := time.NewTicker(time.Second)
OUT:
	for {
		select {
		case <-timer.C:
			if len(p2p.GetPeers()) > 0 {
				break OUT
			}
		}
	}
	assert.Equal(1, len(p2p.GetPeers()))
}

func TestP2P_Gather(t *testing.T) {
	defer monkey.UnpatchAll()
	assert := assert.New(t)
	conf := mockConfig()
	conf.PersistentPeers = "tcp://192.168.1.1:8080"
	p2p, err := NewP2P(conf)
	assert.Nil(err)

	//mock peer
	addr, _ := common.ParseNetAddress(conf.PersistentPeers)
	mockPeer := mockPeer(addr, true, false, p2p.internalChan, nil)
	monkey.Patch(NewOutboundPeer, func(addr *common.NetAddress, persistent bool, msgChan chan<- *internalMsg) *Peer {
		return mockPeer
	})

	// mock listen
	monkey.Patch(net.Listen, func(network, address string) (net.Listener, error) {
		return newTestListener(), nil
	})
	err = p2p.Start()
	assert.Nil(err)

	time.Sleep(time.Second)
	if len(p2p.GetPeers()) <= 0 {
		assert.Nil(errors.New("failed to connect persistent peer"))
	}

	// retrieve message from send channel
	go func() {
		for {
			select {
			case msg := <-p2p.GetPeers()[0].sendChan:
				switch msg.payload.MsgType() {
				case message.GET_BLOCKS_TYPE:
					p2p.internalChan <- &internalMsg{
						from:    mockPeer.GetAddr(),
						payload: &message.Block{},
					}
				}
			}
		}
	}()
	p2p.Gather(func(peerState uint64) bool {
		return true
	}, &message.BlockReq{})
	timer := time.NewTicker(time.Second)
	select {
	case msg := <-p2p.MessageChan():
		if msg.MsgType() != message.BLOCK_TYPE {
			assert.Nil(errors.New("failed to gather block from p2p"))
		}
	case <-timer.C:
		assert.Nil(errors.New("failed to connect persistent peer"))
	}
	p2p.Stop()
}

type testListener struct {
	connChan chan net.Conn
}

func newTestListener() *testListener {
	return &testListener{
		connChan: make(chan net.Conn),
	}
}

func (this *testListener) Accept() (conn net.Conn, err error) {
	defer func() {
		// recover from panic if one occured.
		if recover() != nil {
			err = errors.New("listener have stopped")
		}
	}()
	conn = <-this.connChan
	return
}

func (this *testListener) Close() error {
	close(this.connChan)
	return nil
}

func (this *testListener) Addr() net.Addr {
	return nil
}

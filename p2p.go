package p2p

import (
	"fmt"
	"github.com/DSiSc/craft/log"
	"github.com/DSiSc/p2p/common"
	"github.com/DSiSc/p2p/config"
	"github.com/DSiSc/p2p/message"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	service_VERSION             = 1
	persistentPeerRetryInterval = time.Minute
	stallTickInterval           = 15 * time.Second
	stallResponseTimeout        = 30 * time.Second
	heartBeatInterval           = 10 * time.Second
)

// P2P is p2p service implementation.
type P2P struct {
	config        *config.P2PConfig
	listener      net.Listener // net listener
	msgChan       chan *internalMsg
	stallChan     chan *internalMsg
	quitChan      chan struct{}
	addrManager   *AddressManager
	state         uint64 // service's state
	outbountPeers map[string]*Peer
	inboundPeers  map[string]*Peer
	lock          sync.RWMutex
}

// NewP2P create a p2p service instance
func NewP2P(config *config.P2PConfig) (*P2P, error) {
	addrManger := NewAddressManager(config.AddrBookFilePath)
	return &P2P{
		config:        config,
		addrManager:   addrManger,
		msgChan:       make(chan *internalMsg),
		stallChan:     make(chan *internalMsg),
		quitChan:      make(chan struct{}),
		outbountPeers: make(map[string]*Peer),
		inboundPeers:  make(map[string]*Peer),
	}, nil
}

// Start start p2p service
func (service *P2P) Start() error {
	service.addrManager.Start()

	netAddr, err := common.ParseNetAddress(service.config.ListenAddress)
	if err != nil {
		log.Error("invalid listen address")
		return err
	}
	service.addrManager.AddOurAddress(netAddr)
	listener, err := net.Listen(netAddr.Protocol, netAddr.IP+":"+strconv.Itoa(int(netAddr.Port)))
	if err != nil {
		log.Error("failed to create listener with address: %s, as: %v", netAddr.ToString(), err)
		return err
	}
	service.listener = listener

	go service.startListen(listener) // listen to accept new connection
	go service.recvHandler()         // message receive handler
	go service.stallHandler()        // message response timeout handler
	go service.connectPeers()        // connect to network peers
	go service.addressHandler()      // request address from neighbor peers
	go service.heartBeatHandler()    // start heartbeat handler

	return nil
}

// Stop stop p2p service
func (service *P2P) Stop() {
	peers := service.GetPeers()
	for _, peer := range peers {
		service.stopPeer(peer.addr)
	}
	close(service.quitChan)
	service.addrManager.Stop()
	service.listener.Close()

}

// recvHandler listen to accept connection from inbound peer.
func (service *P2P) startListen(listener net.Listener) {
	for {
		// listen to accept new connection
		conn, err := listener.Accept()
		if err != nil || conn == nil {
			log.Error("encounter error when accepting the new connection: %v", err)
			break
		}

		// parse inbound connection's address
		log.Debug("accept a new connection from %v", conn.RemoteAddr())
		addr, err := common.ParseNetAddress(conn.RemoteAddr().String())
		if err != nil {
			log.Error("unrecognized peer address: %v", err)
			conn.Close()
			continue
		}
		service.addrManager.AddAddress(addr)

		// check num of the inbound peer
		if len(service.inboundPeers) > service.config.MaxConnInBound {
			conn.Close()
			continue
		}

		// add to inbound peer and start the handshake.
		peer := NewInboundPeer(addr, service.msgChan, conn)
		err = service.addInBoundPeer(peer)
		if err != nil {
			log.Error("failed to add peer to inbound peer list, as: %v", err)
			conn.Close()
			continue
		}
		err = peer.Start()
		if err != nil {
			log.Error("failed to start peer %s, as: %v", peer.GetAddr().ToString(), err)
			conn.Close()
			continue
		}
	}
}

// add inbound peer
func (service *P2P) addInBoundPeer(peer *Peer) error {
	return service.addPeer(true, peer)
}

// add outbound peer
func (service *P2P) addOutBoundPeer(peer *Peer) error {
	return service.addPeer(false, peer)
}

// add peer
func (service *P2P) addPeer(inbound bool, peer *Peer) error {
	service.lock.Lock()
	defer service.lock.Unlock()
	if service.inboundPeers[peer.GetAddr().ToString()] != nil || service.outbountPeers[peer.GetAddr().ToString()] != nil {
		return fmt.Errorf("peer %s already in our connected peer list", peer.GetAddr().ToString())
	}
	if inbound {
		service.inboundPeers[peer.GetAddr().ToString()] = peer
	} else {
		service.outbountPeers[peer.GetAddr().ToString()] = peer
	}
	return nil
}

// handle stall detection of the message response
func (service *P2P) stallHandler() {
	stallTicker := time.NewTicker(stallTickInterval)
	pendingResponses := make(map[*common.NetAddress]map[message.MessageType]time.Time)
	for {
		select {
		case msg := <-service.stallChan:
			if msg == nil {
				continue
			}
			if service.isOutMsg(msg) {
				addPendingRespMsg(pendingResponses, msg)
			} else {
				removePendingRespMsg(pendingResponses, msg)
			}
		case <-stallTicker.C:
			now := time.Now()
			timeOutAddrs := make([]*common.NetAddress, 0)
			for addr, pendings := range pendingResponses {
				for msgType, deadline := range pendings {
					if now.Before(deadline) {
						continue
					}
					log.Error("Peer %s receive %v type message's response timeout", addr.ToString(), msgType)
					timeOutAddrs = append(timeOutAddrs, addr)
					service.stopPeer(addr)
					break
				}
			}
			for _, timeOutAddr := range timeOutAddrs {
				delete(pendingResponses, timeOutAddr)
			}
		case <-service.quitChan:
			return
		}
	}
}

// check whether msg is out message.
func (service *P2P) isOutMsg(msg *internalMsg) bool {
	if msg.from == nil {
		return false
	}
	return service.addrManager.OurAddress().Equal(msg.from)
}

// add a message to pending response queue
func addPendingRespMsg(pendingQueue map[*common.NetAddress]map[message.MessageType]time.Time, msg *internalMsg) {
	deadline := time.Now().Add(stallResponseTimeout)
	if pendingQueue[msg.from] == nil {
		pendingQueue[msg.from] = make(map[message.MessageType]time.Time)
	}
	pendingQueue[msg.from][msg.payload.ResponseMsgType()] = deadline
}

// remove message when receiving corresponding response.
func removePendingRespMsg(pendingQueue map[*common.NetAddress]map[message.MessageType]time.Time, msg *internalMsg) {
	if pendingQueue[msg.from] != nil {
		delete(pendingQueue[msg.from], msg.payload.MsgType())
	}
}

// connectPeers connect to peers in p2p network
func (service *P2P) connectPeers() {
	service.connectPersistentPeers()
	service.connectDnsSeeds()
	service.connectNormalPeers()
}

// connect to persistent peers
func (service *P2P) connectPersistentPeers() {
	if service.config.PersistentPeers != "" {
		peerAddres := strings.Split(service.config.PersistentPeers, ",")
		for _, peerAddr := range peerAddres {
			netAddr, err := common.ParseNetAddress(peerAddr)
			if err != nil {
				log.Warn("invalid persistent peer address")
				continue
			}
			if service.addrManager.OurAddress().Equal(netAddr) {
				continue
			}

			service.addrManager.AddAddress(netAddr) //record address
			peer := NewOutboundPeer(netAddr, true, service.msgChan)
			err = service.addOutBoundPeer(peer)
			if err != nil {
				log.Error("failed to add peer %s to outbound list, as: %v", peer.GetAddr().ToString(), err)
			}
			go service.connectPeer(peer)
		}
	}
}

// connect to dns seeds
func (service *P2P) connectDnsSeeds() {
	//TODO
}

// connect to dns seeds
func (service *P2P) connectNormalPeers() {
	// random select peer to connect
	ticker := time.NewTicker(30 * time.Second)
	attemptTimes := 30 * (service.config.MaxConnOutBound - service.GetOutBountPeersCount())
	for {
		// connect to peer
		for i := 0; i <= attemptTimes; i++ {
			if service.GetOutBountPeersCount() >= service.config.MaxConnOutBound || service.addrManager.GetAddressCount() <= service.GetOutBountPeersCount() {
				break
			}
			addr, err := service.addrManager.GetAddress()
			if err != nil {
				break
			}
			if service.containsPeer(addr) {
				continue
			}
			peer := NewOutboundPeer(addr, false, service.msgChan)
			err = service.addOutBoundPeer(peer)
			if err != nil {
				log.Error("failed to add peer %s to outbound list, as: %v", peer.GetAddr().ToString(), err)
			}
			go service.connectPeer(peer)
		}

		//wait for time out
		select {
		case <-ticker.C:
		case <-service.quitChan:
			return
		}
	}
}

// check whether peer with this address have existed in the neighbor list
func (service *P2P) containsPeer(addr *common.NetAddress) bool {
	service.lock.RLock()
	defer service.lock.RUnlock()
	return service.outbountPeers[addr.ToString()] != nil || service.inboundPeers[addr.ToString()] != nil
}

// connect to a peer
func (service *P2P) connectPeer(peer *Peer) {
	ticker := time.NewTicker(persistentPeerRetryInterval)
RETRY:
	err := peer.Start()
	if err != nil {
		log.Error("failed to start peer %s, as: %v", peer.GetAddr().ToString(), err)
		if peer.IsPersistent() {
			select {
			case <-ticker.C:
				goto RETRY
			case <-service.quitChan:
				return
			}
		}
	}
}

// stop the peer with specified address
func (service *P2P) stopPeer(addr *common.NetAddress) {
	service.lock.Lock()
	defer service.lock.Unlock()
	if service.inboundPeers[addr.ToString()] != nil {
		service.inboundPeers[addr.ToString()].Stop()
		delete(service.inboundPeers, addr.ToString())
	}
	if service.outbountPeers[addr.ToString()] != nil {
		service.outbountPeers[addr.ToString()].Stop()
		delete(service.inboundPeers, addr.ToString())
	}
}

// addresses handler(request more addresses from neighbor peers)
func (service *P2P) addressHandler() {
	ticker := time.NewTicker(30 * time.Second)
	for {
		// get more address
		if service.addrManager.NeedMoreAddrs() {
			peers := service.GetPeers()
			if peers != nil && len(peers) > 0 {
				addReq := &message.AddrReq{}
				service.SendMsg(peers[rand.Intn(len(peers))], addReq)
			}
		}

		select {
		case <-ticker.C:
		case <-service.quitChan:
			return
		}
	}
}

// receive handler (receive message from neighbor peers)
func (service *P2P) recvHandler() {
	for {
		select {
		case msg := <-service.msgChan:
			service.stallChan <- msg
			switch msg.payload.(type) {
			case *peerDisconnecMsg:
				service.stopPeer(msg.from)
			case *message.PingMsg:
				pingMsg := &message.PingMsg{
					State: service.getLocalState(),
				}
				peer := service.GetPeerByAddress(msg.from)
				service.SendMsg(peer, pingMsg)
			case *message.PongMsg:
				peer := service.GetPeerByAddress(msg.from)
				peer.SetState(msg.payload.(*message.PongMsg).State)
			default:
				//TODO receive a normal message
			}
		case <-service.quitChan:
			return
		}
	}
}

// send hear beat message periodically
func (service *P2P) heartBeatHandler() {
	timer := time.NewTicker(heartBeatInterval)
	for {
		select {
		case <-timer.C:
			pingMsg := &message.PingMsg{
				State: 1,
			}
			service.BroadCast(pingMsg)
		case <-service.quitChan:
			return
		}
	}
}

// BroadCast broad cast message to all neighbor peers
func (service *P2P) BroadCast(msg message.Message) {
	service.lock.RLock()
	defer service.lock.RUnlock()
	for _, peer := range service.outbountPeers {
		go service.SendMsg(peer, msg)
	}

	for _, peer := range service.inboundPeers {
		go service.SendMsg(peer, msg)
	}
}

// SendMsg send message to a peer.
func (service *P2P) SendMsg(peer *Peer, msg message.Message) error {
	message := &internalMsg{
		service.addrManager.OurAddress(),
		peer.addr,
		msg,
		nil,
	}
	peer.Channel() <- message
	service.registerPendingResp(message)
	return nil
}

// register need response message to pending response queue
func (service *P2P) registerPendingResp(msg *internalMsg) {
	//check whether message need response
	if msg.payload.ResponseMsgType() != message.NIL {
		service.stallChan <- msg
	}
}

// GetOutBountPeersCount get out bount peer count
func (service *P2P) GetOutBountPeersCount() int {
	service.lock.RLock()
	defer service.lock.RUnlock()
	return len(service.outbountPeers)
}

// GetPeers get service's inbound peers and outbound peers
func (service *P2P) GetPeers() []*Peer {
	service.lock.RLock()
	defer service.lock.RUnlock()
	peers := make([]*Peer, 0)
	for _, peer := range service.inboundPeers {
		peers = append(peers, peer)
	}
	for _, peer := range service.outbountPeers {
		peers = append(peers, peer)
	}
	return peers
}

// GetPeerByAddress get a peer by net address
func (service *P2P) GetPeerByAddress(addr *common.NetAddress) *Peer {
	service.lock.RLock()
	defer service.lock.RUnlock()
	if service.inboundPeers[addr.ToString()] != nil {
		return service.inboundPeers[addr.ToString()]
	}
	if service.outbountPeers[addr.ToString()] != nil {
		return service.outbountPeers[addr.ToString()]
	}
	return nil
}

// get local state
func (service *P2P) getLocalState() uint64 {
	//TODO get local state
	return 1
}

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/DSiSc/blockchain"
	"github.com/DSiSc/blockchain/config"
	"github.com/DSiSc/craft/log"
	"github.com/DSiSc/craft/types"
	"github.com/DSiSc/p2p"
	"github.com/DSiSc/p2p/common"
	p2pconf "github.com/DSiSc/p2p/config"
	"github.com/DSiSc/p2p/message"
	"github.com/DSiSc/p2p/tools"
	"math/rand"
	"net/http"
	"os"
	"syscall"
	"time"
)

func sysSignalProcess() {
	sysSignalProcess := NewSignalSet()
	sysSignalProcess.RegisterSysSignal(syscall.SIGINT, func(os.Signal, interface{}) {
		log.Warn("handle signal SIGINT.")
		os.Exit(1)
	})
	sysSignalProcess.RegisterSysSignal(syscall.SIGTERM, func(os.Signal, interface{}) {
		log.Warn("handle signal SIGTERM.")
		os.Exit(1)
	})
	go sysSignalProcess.CatchSysSignal()
}

func main() {
	chainConf := config.BlockChainConfig{
		PluginName: blockchain.PLUGIN_MEMDB,
	}
	blockchain.InitBlockChain(chainConf, &tools.P2PTestEventCenter{})
	var addrBookPath, listenAddress, persistentPeers, localAddrStr, displayServer string
	var maxConnOutBound, maxConnInBound int
	var traceMaster bool
	flagSet := flag.NewFlagSet("broadcast", flag.ExitOnError)
	flagSet.StringVar(&addrBookPath, "path", "./address_book.json", "Address book file path")
	flagSet.StringVar(&listenAddress, "listen", "tcp://0.0.0.0:8888", "Listen address")
	flagSet.StringVar(&persistentPeers, "peers", "", "Persistent peers")
	flagSet.IntVar(&maxConnOutBound, "out", 20, "Max num of outbound peer")
	flagSet.IntVar(&maxConnInBound, "in", 60, "Max num of out inbound peer")
	flagSet.StringVar(&localAddrStr, "local_addr", "", "local address to identify this peer")
	flagSet.StringVar(&displayServer, "display_server", "", "trace info display server address")
	flagSet.BoolVar(&traceMaster, "master", false, "trace master")
	flagSet.Usage = func() {
		fmt.Println(`Justitia blockchain p2p test tool.

Usage:
	broadcast [-path ./address_book.json] [-listen tcp://0.0.0.0:8080] [-peers tcp://192.168.1.100:1080] [-out 20] [-in 60]]

Examples:
	broadcast -peers tcp://192.168.1.100:1080`)
		fmt.Println("Flags:")
		flagSet.PrintDefaults()
	}
	flagSet.Parse(os.Args[1:])

	// init p2p config
	conf := &p2pconf.P2PConfig{
		AddrBookFilePath: addrBookPath,
		ListenAddress:    listenAddress,
		PersistentPeers:  persistentPeers,
		MaxConnOutBound:  maxConnOutBound,
		MaxConnInBound:   maxConnInBound,
	}
	localAddr, err := common.ParseNetAddress(localAddrStr)
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"invalid local_addr, as: %v", err,
		)
		os.Exit(1)
	}

	// create p2p
	p2p, err := p2p.NewP2P(conf, tools.NewP2PTestEventCenter())
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"failed to create p2p, as: %v", err,
		)
		os.Exit(1)
	}

	// start p2p
	err = p2p.Start()
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"failed to start p2p, as: %v", err,
		)
		os.Exit(1)
	}

	// start trace handler
	taceHandler := NewTraceMsgHandler(localAddr, p2p, displayServer)
	taceHandler.Start()

	sysSignalProcess()

	// send trace message periodically
	timer := time.NewTicker(30 * time.Second)
	for {
		if traceMaster {
			tmsg := newTraceMsg(localAddr)
			p2p.BroadCast(tmsg)
			go reportTraceRoutes(tmsg, displayServer)
			go reportNeighbors(localAddrStr, p2p.GetPeers(), displayServer)
		} else {
			go reportNeighbors(localAddrStr, p2p.GetPeers(), displayServer)
		}
		<-timer.C
	}

}

func newTraceMsg(localAddr *common.NetAddress) *message.TraceMsg {
	id := randomHash()
	routes := make([]*common.NetAddress, 0)
	routes = append(routes, localAddr)
	return &message.TraceMsg{
		ID:     id,
		Routes: routes,
	}
}

func randomHash() (hash types.Hash) {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, rand.Int63())
	hasher := sha256.New()
	hasher.Write(buf.Bytes())
	copy(hash[:], hasher.Sum(nil))
	return
}

func reportTraceRoutes(tmsg *message.TraceMsg, displayServer string) {
	stringRoutes := make([]string, 0)
	for _, route := range tmsg.Routes {
		stringRoutes = append(stringRoutes, route.ToString())
	}
	tmsgByte, err := json.Marshal(stringRoutes)
	if err != nil {
		log.Error("failed to encode trace message routes, as: %v", err)
	}
	_, err = http.Post("http://"+displayServer+"/trace_msg/0x"+fmt.Sprintf("%x", tmsg.ID), "application/json", bytes.NewReader(tmsgByte))
	if err != nil {
		log.Error("failed to send trace message to display server, as:%v", err)
	}
}

func reportNeighbors(localAddr string, peers []*p2p.Peer, displayServer string) {
	stringPeers := make([]string, 0)
	for _, peer := range peers {
		stringPeers = append(stringPeers, peer.GetAddr().ToString())
	}
	pmsgByte, err := json.Marshal(stringPeers)
	if err != nil {
		log.Error("failed to encode peer neighbors, as: %v", err)
	}
	_, err = http.Post("http://"+displayServer+"/neighbor/"+localAddr, "application/json", bytes.NewReader(pmsgByte))
	if err != nil {
		log.Error("failed to send neighbor info to display server, as:%v", err)
	}
}

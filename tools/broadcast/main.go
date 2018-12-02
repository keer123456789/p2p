package main

import (
	"flag"
	"fmt"
	"github.com/DSiSc/craft/log"
	"github.com/DSiSc/p2p"
	p2pconf "github.com/DSiSc/p2p/config"
	"net"
	"os"
	"syscall"
)

func sysSignalProcess(p *p2p.P2P) {
	sysSignalProcess := NewSignalSet()
	sysSignalProcess.RegisterSysSignal(syscall.SIGINT, func(os.Signal, interface{}) {
		log.Warn("handle signal SIGINT.")
		//p.Stop()
		os.Exit(1)
	})
	sysSignalProcess.RegisterSysSignal(syscall.SIGTERM, func(os.Signal, interface{}) {
		log.Warn("handle signal SIGTERM.")
		//p.Stop()
		os.Exit(1)
	})
	go sysSignalProcess.CatchSysSignal()
}

func main() {
	listener, _ := net.Listen("tcp", "0.0.0.0:8080")
	fmt.Println(listener.Addr().String())
	var addrBookPath, listenAddress, persistentPeers string
	var maxConnOutBound, maxConnInBound int
	flagSet := flag.NewFlagSet("broadcast", flag.ExitOnError)
	flagSet.StringVar(&addrBookPath, "path", "./address_book.json", "Address book file path")
	flagSet.StringVar(&listenAddress, "listen", "tcp://0.0.0.0:8080", "Listen address")
	flagSet.StringVar(&persistentPeers, "peers", "", "Persistent peers")
	flagSet.IntVar(&maxConnOutBound, "out", 20, "Max num of outbound peer")
	flagSet.IntVar(&maxConnInBound, "in", 60, "Max num of out inbound peer")
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

	// create p2p
	p2p, err := p2p.NewP2P(conf)
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
	sysSignalProcess(p2p)
	exitChan := make(chan interface{})
	select {
	case <-exitChan:
	}
}

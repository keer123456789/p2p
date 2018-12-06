package main

import (
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	var listenAddr string
	var refreshInterval int
	flagSet := flag.NewFlagSet("statitics", flag.ExitOnError)
	flagSet.StringVar(&listenAddr, "listen", "0.0.0.0:8080", "statitics listen address, default 0.0.0.0:8080")
	flagSet.IntVar(&refreshInterval, "refresh", 120, "p2p topo refreshHandler interval (s)")
	flagSet.Usage = func() {
		fmt.Println(`Justitia blockchain p2p test statitics server.
Usage:
	statitics [-listen 0.0.0.0:8080] [-refresh 120]

Examples:
	statitics -listen 0.0.0.0:8080`)
		fmt.Println("Flags:")
		flagSet.PrintDefaults()
	}
	// init display server
	displayServer := NewDisplayServer(time.Duration(refreshInterval) * time.Second)
	displayServer.Start()

	// init http server
	router := mux.NewRouter()
	router.HandleFunc("/neighbor/{addr}", displayServer.ReportNeighbors).Methods("POST")
	router.HandleFunc("/trace_msg/{msgId}", displayServer.ReportTraceMsg).Methods("POST")
	router.HandleFunc("/topos", displayServer.GetNetTopo).Methods("GET")
	router.HandleFunc("/routes", displayServer.GetMessageRoute).Methods("GET")
	log.Fatal(http.ListenAndServe(listenAddr, router))

	// stop display server
	displayServer.Stop()
	os.Exit(0)
}

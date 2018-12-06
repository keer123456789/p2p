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
	flagSet.IntVar(&refreshInterval, "refresh", 120, "p2p topo refresh interval (s)")
	flagSet.Usage = func() {
		fmt.Println(`Justitia blockchain p2p test statitics server.
Usage:
	statitics [-listen 0.0.0.0:8080] [-refresh 60]

Examples:
	statitics -listen 0.0.0.0:8080`)
		fmt.Println("Flags:")
		flagSet.PrintDefaults()
	}

	displayServer := NewDisplayServer()

	// refresh topo/routes periodically
	go func(refreshInterval int, displayServer *DisplayServer) {
		for {
			timer := time.NewTicker(time.Duration(refreshInterval) * time.Second)
			select {
			case <-timer.C:
				displayServer.Refresh()
			}
		}
	}(refreshInterval, displayServer)

	router := mux.NewRouter()
	router.HandleFunc("/neighbor/{addr}", displayServer.ReportNeighbors).Methods("POST")
	router.HandleFunc("/trace_msg/{msgId}", displayServer.ReportTraceMsg).Methods("POST")
	router.HandleFunc("/topos", displayServer.GetNetTopo).Methods("GET")
	router.HandleFunc("/routes", displayServer.GetMessageRoute).Methods("GET")
	log.Fatal(http.ListenAndServe(listenAddr, router))
	os.Exit(0)
}

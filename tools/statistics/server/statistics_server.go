package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/DSiSc/craft/log"
	"github.com/DSiSc/p2p/tools/common"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"sync"
	"time"
)

const SUCCESS = "SUCCESS"

func main() {
	var listenAddr string
	var refreshInterval int
	flagSet := flag.NewFlagSet("statistics-server", flag.ExitOnError)
	flagSet.StringVar(&listenAddr, "listen", "0.0.0.0:8080", "statistics listen address, default 0.0.0.0:8080")
	flagSet.IntVar(&refreshInterval, "refreshHandler", 120, "p2p topo refreshHandler interval (s)")
	flagSet.Usage = func() {
		fmt.Println(`Justitia blockchain p2p test statistics server.
Usage:
	statistics [-listen 0.0.0.0:8080] [-refreshHandler 60]

Examples:
	statistics -listen 0.0.0.0:8080`)
		fmt.Println("Flags:")
		flagSet.PrintDefaults()
	}
	// init display server
	displayServer := NewDisplayServer(time.Duration(refreshInterval) * time.Second)
	displayServer.Start()

	// init http server
	router := mux.NewRouter()
	router.HandleFunc("/neighbor/{addr}", displayServer.ReportNeighbors).Methods("POST")
	router.HandleFunc("/topos", displayServer.GetNetTopo).Methods("GET")
	router.HandleFunc("/msgs/{msgId}", displayServer.ReportMsg).Methods("POST")
	router.HandleFunc("/msgs/{msgId}", displayServer.GetReportMessage).Methods("GET")
	router.HandleFunc("/msgs", displayServer.GetReportMessages).Methods("GET")
	err := http.ListenAndServe(listenAddr, router)
	if err != nil {
		log.Error("Failed to start statics server %v", err)
	}

	// stop display server
	displayServer.Stop()
	os.Exit(0)
}

// DisplayServer is used to collect p2p net info, and display to admin
type DisplayServer struct {
	topos           map[string][]*common.Neighbor
	msgReport       map[string]map[string]*common.ReportMsg
	timeoutCache    map[string]time.Time
	timeoutInterval time.Duration
	quitChan        chan interface{}
	lock            sync.RWMutex
}

// NewDisplayServer create a display server instance
func NewDisplayServer(timeoutInterval time.Duration) *DisplayServer {
	return &DisplayServer{
		topos:           make(map[string][]*common.Neighbor),
		msgReport:       make(map[string]map[string]*common.ReportMsg),
		timeoutCache:    make(map[string]time.Time),
		timeoutInterval: timeoutInterval,
		quitChan:        make(chan interface{}),
	}
}

// ReportNeighbors is used to receive peer's neighbors report
func (this *DisplayServer) ReportNeighbors(w http.ResponseWriter, r *http.Request) {
	// parse param
	params := mux.Vars(r)
	addr := params["addr"]
	var neighbors []*common.Neighbor
	err := json.NewDecoder(r.Body).Decode(&neighbors)
	if err != nil {
		log.Error("failed to decode neighbors body, as: %v", err)
		responseWithError(err, w)
	}

	// record topo
	this.lock.Lock()
	this.topos[addr] = neighbors
	this.timeoutCache[addr] = time.Now().Add(this.timeoutInterval)
	this.lock.Unlock()
	response(SUCCESS, w)
}

// GetNetTopo is used to get current net topo info
func (this *DisplayServer) GetNetTopo(w http.ResponseWriter, r *http.Request) {
	this.lock.RLock()
	response(this.topos, w)
	this.lock.RUnlock()
}

// ReportMsg is used to record p2p message detail
func (this *DisplayServer) ReportMsg(w http.ResponseWriter, r *http.Request) {
	// parse param
	params := mux.Vars(r)
	msgId := params["msgId"]
	repMsg := new(common.ReportMsg)
	err := json.NewDecoder(r.Body).Decode(repMsg)
	if err != nil {
		log.Error("failed to decode report message body, as: %v", err)
		responseWithError(err, w)
	}

	// record routes
	this.lock.Lock()
	if this.msgReport[msgId] == nil {
		this.msgReport[msgId] = make(map[string]*common.ReportMsg)
	}

	// this peer have not received this message
	if this.msgReport[msgId][repMsg.ReportPeer] == nil {
		repMsg.Time = time.Now()
		this.msgReport[msgId][repMsg.ReportPeer] = repMsg
	}
	this.timeoutCache[msgId] = time.Now().Add(this.timeoutInterval)
	this.lock.Unlock()
	response(SUCCESS, w)
}

// GetReportMessage is used to get a message route info
func (this *DisplayServer) GetReportMessage(w http.ResponseWriter, r *http.Request) {
	// parse param
	params := mux.Vars(r)
	msgId := params["msgId"]
	this.lock.RLock()
	response(this.msgReport[msgId], w)
	this.lock.RUnlock()
}

// GetReportMessages is used to get all report message route info
func (this *DisplayServer) GetReportMessages(w http.ResponseWriter, r *http.Request) {
	// parse param
	this.lock.RLock()
	response(this.msgReport, w)
	this.lock.RUnlock()
}

// Start start server
func (this *DisplayServer) Start() {
	go this.refreshHandler()
}

// Stop stop server
func (this *DisplayServer) Stop() {
	close(this.quitChan)
}

// clean timeout message
func (this *DisplayServer) refreshHandler() {
	refreshInterval := time.NewTicker(30 * time.Second)
	for {
		select {
		case <-refreshInterval.C:
			this.lock.RLock()
			now := time.Now()
			for key, deadline := range this.timeoutCache {
				if now.Before(deadline) {
					continue
				}
				delete(this.topos, key)
				delete(this.msgReport, key)
				delete(this.timeoutCache, key)
			}
			this.lock.RUnlock()
		case <-this.quitChan:
			return
		}
	}
}

// response client with related error info
func responseWithError(err error, w http.ResponseWriter) {
	response(common.NewResponse(err), w)
}

// response client request
func response(data interface{}, w http.ResponseWriter) {
	json.NewEncoder(w).Encode(data)
}

package main

import (
	"encoding/json"
	"github.com/DSiSc/craft/log"
	"github.com/gorilla/mux"
	"net/http"
	"sync"
	"time"
)

const SUCCESS = "SUCCESS"

type DisplayServer struct {
	topos           map[string][]string
	msgRoutes       map[string][]map[string]string
	timeoutCache    map[string]time.Time
	timeoutInterval time.Duration
	quitChan        chan interface{}
	lock            sync.RWMutex
}

func NewDisplayServer(timeoutInterval time.Duration) *DisplayServer {
	return &DisplayServer{
		topos:           make(map[string][]string),
		msgRoutes:       make(map[string][]map[string]string),
		timeoutCache:    make(map[string]time.Time),
		timeoutInterval: timeoutInterval,
		quitChan:        make(chan interface{}),
	}
}

func (this *DisplayServer) ReportNeighbors(w http.ResponseWriter, r *http.Request) {
	// parse param
	params := mux.Vars(r)
	addr := params["addr"]
	var neighbors []string
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

func (this *DisplayServer) ReportTraceMsg(w http.ResponseWriter, r *http.Request) {
	// parse param
	params := mux.Vars(r)
	msgId := params["msgId"]
	var routes []string
	err := json.NewDecoder(r.Body).Decode(&routes)
	if err != nil {
		log.Error("failed to decode trace routes body, as: %v", err)
		responseWithError(err, w)
	}

	// record routes
	this.lock.Lock()
	if this.msgRoutes[msgId] == nil {
		this.msgRoutes[msgId] = make([]map[string]string, 0)
	}
	for index, addr := range routes {
		if len(this.msgRoutes[msgId]) <= index {
			this.msgRoutes[msgId] = append(this.msgRoutes[msgId], make(map[string]string))
		}
		var previousPeer string
		if index > 0 {
			previousPeer = routes[index-1]
		} else {
			previousPeer = ""
		}
		if this.msgRoutes[msgId][index][addr] == "" {
			this.msgRoutes[msgId][index][addr] = previousPeer
		}
	}
	this.timeoutCache[msgId] = time.Now().Add(this.timeoutInterval)
	this.lock.Unlock()
	response(SUCCESS, w)
}

func (this *DisplayServer) Start() {
	go this.refreshHandler()
}

func (this *DisplayServer) Stop() {
	close(this.quitChan)
}

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
				delete(this.msgRoutes, key)
				delete(this.timeoutCache, key)
			}
			this.lock.RUnlock()
		case <-this.quitChan:
			return
		}
	}
}

func (this *DisplayServer) GetNetTopo(w http.ResponseWriter, r *http.Request) {
	this.lock.RLock()
	response(this.topos, w)
	this.lock.RUnlock()
}

func (this *DisplayServer) GetMessageRoute(w http.ResponseWriter, r *http.Request) {
	this.lock.RLock()
	response(this.msgRoutes, w)
	this.lock.RUnlock()
}

func responseWithError(err error, w http.ResponseWriter) {
	response(NewResponse(err), w)
}

func response(data interface{}, w http.ResponseWriter) {
	json.NewEncoder(w).Encode(data)
}

type Response struct {
	Error string `json:"error"`
}

func NewResponse(err error) *Response {
	return &Response{
		Error: err.Error(),
	}
}

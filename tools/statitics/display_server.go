package main

import (
	"encoding/json"
	"github.com/DSiSc/craft/log"
	"github.com/gorilla/mux"
	"net/http"
	"strings"
	"sync"
)

const SUCCESS = "SUCCESS"

type DisplayServer struct {
	topos     map[string]map[string]bool
	msgRoutes map[string][]map[string]string
	lock      sync.RWMutex
}

func NewDisplayServer() *DisplayServer {
	return &DisplayServer{
		topos:     make(map[string]map[string]bool),
		msgRoutes: make(map[string][]map[string]string),
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
	if this.topos[addr] == nil {
		this.topos[addr] = make(map[string]bool)
	}
	for _, neighbor := range neighbors {
		neighborAddr := neighbor
		if strings.HasPrefix(neighborAddr, "tcp://") {
			neighborAddr = neighborAddr[6:]
		}
		if this.topos[neighborAddr] == nil {
			this.topos[neighborAddr] = make(map[string]bool)
		}
		this.topos[addr][neighborAddr] = true
		this.topos[neighborAddr][addr] = true
	}
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
	this.lock.Unlock()
	response(SUCCESS, w)
}

func (this *DisplayServer) Refresh() {
	this.lock.RLock()
	this.topos = make(map[string]map[string]bool)
	this.msgRoutes = make(map[string][]map[string]string)
	this.lock.RUnlock()
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

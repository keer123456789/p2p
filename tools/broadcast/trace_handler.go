package main

import (
	"github.com/DSiSc/craft/log"
	"github.com/DSiSc/p2p"
	"github.com/DSiSc/p2p/common"
	"github.com/DSiSc/p2p/message"
)

type TraceMsgHandler struct {
	localAddr     *common.NetAddress
	p2p           p2p.P2PAPI
	displayServer string
	quitChan      chan interface{}
}

func NewTraceMsgHandler(localAddr *common.NetAddress, p2p p2p.P2PAPI, displayServer string) *TraceMsgHandler {
	return &TraceMsgHandler{
		localAddr:     localAddr,
		p2p:           p2p,
		quitChan:      make(chan interface{}),
		displayServer: displayServer,
	}
}

func (this *TraceMsgHandler) Start() {
	go this.recvHandler()
}

func (this *TraceMsgHandler) Stop() {
	close(this.quitChan)
}

func (this *TraceMsgHandler) recvHandler() {
	msgChan := this.p2p.MessageChan()
	for {
		select {
		case msg := <-msgChan:
			switch msg.Payload.MsgType() {
			case message.TRACE_TYPE:
				tmsg := msg.Payload.(*message.TraceMsg)
				log.Info("receive a trace message %x", tmsg.ID)
				tmsg.Routes = append(tmsg.Routes, this.localAddr)
				this.p2p.BroadCast(tmsg)
				go reportTraceRoutes(tmsg, this.displayServer)
			}
		case <-this.quitChan:
			return
		}
	}

}

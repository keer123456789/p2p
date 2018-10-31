package p2p

import (
	"github.com/DSiSc/p2p/common"
	"github.com/DSiSc/p2p/message"
)

const (
	internalMsgRespSuccess = iota
)

// internal message type
type internalMsg struct {
	from    *common.NetAddress
	to      *common.NetAddress
	payload message.Message
	respTo  chan interface{}
}

// peer disconect message ping message
type peerDisconnecMsg struct {
	err error
}

func (this *peerDisconnecMsg) MsgType() message.MessageType {
	return message.PEER_DISCONNECT
}

func (this *peerDisconnecMsg) ResponseMsgType() message.MessageType {
	return message.NIL
}

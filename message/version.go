package message

import "github.com/DSiSc/p2p/common"

// Version version message
type Version struct {
	Version    string             `json:"version"`
	NetAddress *common.NetAddress `json:"net_address"`
}

func (this *Version) MsgType() MessageType {
	return VERSION_TYPE
}

func (this *Version) ResponseMsgType() MessageType {
	return VERACK_TYPE
}

// Version ack message
type VersionAck struct {
}

func (this *VersionAck) MsgType() MessageType {
	return VERACK_TYPE
}

func (this *VersionAck) ResponseMsgType() MessageType {
	return NIL
}

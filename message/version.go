package message

// Version version message
type Version struct {
	Version string `json:"version"`
}

func (this *Version) MsgType() MessageType {
	return VERSION_TYPE
}

func (this *Version) ResponseMsgType() MessageType {
	return VERACK_TYPE
}

// Version ack message
type VersionAck struct {
	Version uint32 `json:"version"`
}

func (this *VersionAck) MsgType() MessageType {
	return VERACK_TYPE
}

func (this *VersionAck) ResponseMsgType() MessageType {
	return NIL
}

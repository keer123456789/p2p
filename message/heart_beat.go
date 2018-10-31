package message

// PingMsg ping message
type PingMsg struct {
	State uint64
}

func (this *PingMsg) MsgType() MessageType {
	return PING_TYPE
}

func (this *PingMsg) ResponseMsgType() MessageType {
	return PONG_TYPE
}

// PongMsg pong message
type PongMsg struct {
	State uint64
}

func (this *PongMsg) MsgType() MessageType {
	return PONG_TYPE
}

func (this *PongMsg) ResponseMsgType() MessageType {
	return NIL
}

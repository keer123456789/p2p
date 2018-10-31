package message

// RejectMsg reject message
type RejectMsg struct {
	Reason string
}

func (this *RejectMsg) MsgType() MessageType {
	return REJECT_TYPE
}

func (this *RejectMsg) ResponseMsgType() MessageType {
	return NIL
}

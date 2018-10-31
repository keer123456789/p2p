package message

import "github.com/DSiSc/craft/types"

// Transaction message
type Transaction struct {
	Tx *types.Transaction `json:"tx"`
}

func (this *Transaction) MsgType() MessageType {
	return TX_TYPE
}

func (this *Transaction) ResponseMsgType() MessageType {
	return NIL
}

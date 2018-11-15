package message

import "github.com/DSiSc/craft/types"

// BlockHeaderReq block request message
type BlockHeaderReq struct {
	Len       uint8      `json:"len"`
	HashStart types.Hash `json:"hash_start"`
	HashStop  types.Hash `json:"hash_stop"`
}

func (this *BlockHeaderReq) MsgType() MessageType {
	return GET_HEADERS_TYPE
}

func (this *BlockHeaderReq) ResponseMsgType() MessageType {
	return HEADERS_TYPE
}

// BlockHeaders block header message
type BlockHeaders struct {
	Headers []*types.Header `json:"headers"`
}

func (this *BlockHeaders) MsgType() MessageType {
	return HEADERS_TYPE
}

func (this *BlockHeaders) ResponseMsgType() MessageType {
	return NIL
}
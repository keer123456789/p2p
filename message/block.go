package message

import "github.com/DSiSc/craft/types"

// BlockReq block request message
type BlockReq struct {
	HeaderHashCount uint8      `json:"header_hash_count"`
	HashStart       types.Hash `json:"hash_start"`
	HashStop        types.Hash `json:"hash_stop"`
}

func (this *BlockReq) MsgType() MessageType {
	return GET_BLOCKS_TYPE
}

func (this *BlockReq) ResponseMsgType() MessageType {
	return BLOCK_TYPE
}

// Block block message
type Block struct {
	Blocks []*types.Block `json:"blocks"`
}

func (this *Block) MsgType() MessageType {
	return BLOCK_TYPE
}

func (this *Block) ResponseMsgType() MessageType {
	return NIL
}

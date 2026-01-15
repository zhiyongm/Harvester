package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
	"time"
)

type BlockHeader struct {
	ParentBlockHash []byte
	StateRoot       []byte
	TxRoot          []byte
	Number          uint64
	Time            time.Time
	Miner           uint64
	TotalTxCount    uint64

	ForeSightBloom []byte
}

func (bh *BlockHeader) Encode() []byte {
	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff)
	err := enc.Encode(bh)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}

func DecodeBH(b []byte) *BlockHeader {
	var blockHeader BlockHeader

	decoder := gob.NewDecoder(bytes.NewReader(b))
	err := decoder.Decode(&blockHeader)
	if err != nil {
		log.Panic(err)
	}

	return &blockHeader
}

func (bh *BlockHeader) Hash() []byte {
	hash := sha256.Sum256(bh.Encode())
	return hash[:]

}

func (bh *BlockHeader) PrintBlockHeader() string {
	vals := []interface{}{
		hex.EncodeToString(bh.ParentBlockHash),
		hex.EncodeToString(bh.StateRoot),
		hex.EncodeToString(bh.TxRoot),
		bh.Number,
		bh.Time,
	}
	res := fmt.Sprintf("%v\n", vals)
	return res
}

type Block struct {
	Header *BlockHeader
	Body   []*Transaction
	Hash   []byte
}

func NewBlock(bh *BlockHeader, bb []*Transaction) *Block {
	return &Block{Header: bh, Body: bb}
}

func (b *Block) PrintBlock() string {
	vals := []interface{}{
		b.Header.Number,
		hex.EncodeToString(b.Hash),
		len(b.Body),
	}
	res := fmt.Sprintf("%v\n", vals)
	return res

}

func (b *Block) Encode() []byte {
	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff)
	err := enc.Encode(b)
	if err != nil {

		log.Panic(err)
	}
	return buff.Bytes()
}

func DecodeB(b []byte) *Block {
	var block Block

	decoder := gob.NewDecoder(bytes.NewReader(b))
	err := decoder.Decode(&block)
	if err != nil {
		log.Panic(err)
	}

	return &block
}

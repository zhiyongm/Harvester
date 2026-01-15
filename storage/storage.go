package storage

import (
	"blockcooker/core"
	"errors"
	"log"

	"github.com/ethereum/go-ethereum/ethdb"
)

var (
	BLK_HEADER_PFX = []byte("H")

	BLK_PFX = []byte("K")
)

type Storage struct {
	DB ethdb.Database
}

func NewStorage(statedb ethdb.Database) *Storage {
	s := &Storage{}

	s.DB = statedb

	return s
}

func (s *Storage) UpdateNewestBlock(newestbhash []byte) {

	err := s.DB.Put([]byte("OnlyNewestBlock"), newestbhash)
	if err != nil {
		log.Panic()
	}

}

func (s *Storage) AddBlockHeader(blockhash []byte, bh *core.BlockHeader) {

	err := s.DB.Put(append(BLK_HEADER_PFX, blockhash...), bh.Encode())

	if err != nil {
		log.Panic()
	}
}

func (s *Storage) AddBlock(b *core.Block) {
	savedBlockEncoded := b.Encode()

	err := s.DB.Put(append(BLK_PFX, b.Hash...), savedBlockEncoded)
	if err != nil {
		log.Panic(err)
	}

	s.AddBlockHeader(b.Hash, b.Header)
	s.UpdateNewestBlock(b.Hash)

}

func (s *Storage) GetBlockHeader(bhash []byte) (*core.BlockHeader, error) {
	var res *core.BlockHeader

	bh_encoded, err := s.DB.Get(append(BLK_HEADER_PFX, bhash...))
	if err != nil {
		return nil, errors.New("the block is not existed")
	}
	res = core.DecodeBH(bh_encoded)

	return res, err
}

func (s *Storage) GetBlock(bhash []byte) (*core.Block, error) {
	var res *core.Block

	b_encoded, err := s.DB.Get(append(BLK_PFX, bhash...))
	if err != nil {
		return nil, errors.New("the block is not existed")
	}
	res = core.DecodeB(b_encoded)

	return res, err
}

func (s *Storage) GetNewestBlockHash() ([]byte, error) {
	var nhb []byte
	nhb, err := s.DB.Get([]byte("OnlyNewestBlock"))

	if nhb == nil {
		return nil, errors.New("cannot find the newest block hash")
	}

	return nhb, err
}

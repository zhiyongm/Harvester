package bloom

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/bits-and-blooms/bloom/v3"
)

type BloomFilter struct {
	filter *bloom.BloomFilter
}

func NewCustomBloomFilter(capacity uint, falsePositiveRate float64) *BloomFilter {
	b := bloom.NewWithEstimates(capacity, falsePositiveRate)

	return &BloomFilter{
		filter: b,
	}
}

func (bf *BloomFilter) AddElement(address *common.Address) {
	bf.filter.Add(address.Bytes())
}

func (bf *BloomFilter) CheckExistence(address common.Address) bool {
	return bf.filter.Test([]byte(address.Bytes()))
}

func (bf *BloomFilter) Encode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(bf.filter)
	if err != nil {
		return nil, fmt.Errorf("编码布隆过滤器失败: %w", err)
	}
	return buf.Bytes(), nil

}

func DecodeNew(data []byte) (*BloomFilter, error) {
	b := &bloom.BloomFilter{}

	var buf bytes.Buffer
	buf.Write(data)
	dec := gob.NewDecoder(&buf)
	err := dec.Decode(b)

	if err != nil {
		return nil, fmt.Errorf("解码布隆过滤器失败: %w", err)
	}

	return &BloomFilter{
		filter: b,
	}, nil

}

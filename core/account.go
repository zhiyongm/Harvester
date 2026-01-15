package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"log"
	"math/big"
)

type AccountState struct {
	Nonce       uint64
	Balance     *big.Int
	StorageRoot []byte
	CodeHash    []byte
}

func (as *AccountState) Deduct(val *big.Int) bool {
	as.Balance.Sub(as.Balance, val)
	as.Nonce++
	return true
}

func (s *AccountState) Deposit(value *big.Int) {
	s.Balance.Add(s.Balance, value)
	s.Nonce++

}

func (as *AccountState) Encode() []byte {
	var buff bytes.Buffer
	encoder := gob.NewEncoder(&buff)
	err := encoder.Encode(as)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}

func DecodeAS(b []byte) *AccountState {
	var as AccountState

	decoder := gob.NewDecoder(bytes.NewReader(b))
	err := decoder.Decode(&as)
	if err != nil {
		log.Panic(err)
	}
	return &as
}

func (as *AccountState) Hash() []byte {
	h := sha256.Sum256(as.Encode())
	return h[:]
}

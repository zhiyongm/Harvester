package utils

import (
	"crypto/ecdsa"
	"math/rand"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type Address = string

const charset = "0123456789abcdef"

func RandomString(n int) string {
	sb := strings.Builder{}
	sb.Grow(n)
	for i := 0; i < n; i++ {
		sb.WriteByte(charset[rand.Intn(len(charset))])
	}
	return sb.String()
}

func RandomBytes(n int) []byte {
	sb := strings.Builder{}
	sb.Grow(n)
	for i := 0; i < n; i++ {
		sb.WriteByte(charset[rand.Intn(len(charset))])
	}
	return []byte(sb.String())
}
func RandomAddress() common.Address {

	return common.HexToAddress(RandomString(40))
}
func RandomHash() common.Hash {
	return common.HexToHash(RandomString(64))
}

func UInt64ToBytes(num uint64) []byte {
	buf := make([]byte, 8)
	for i := uint(0); i < 8; i++ {
		buf[i] = byte(num >> (i * 8))
	}
	return buf
}

func BytesToUInt64(buf []byte) uint64 {
	var num uint64
	for i := uint(0); i < 8; i++ {
		num |= uint64(buf[i]) << (i * 8)
	}
	return num
}
func GenerateEthereumAddress() common.Address {
	privateKey, _ := crypto.GenerateKey()

	publicKey := privateKey.Public()
	publicKeyECDSA, _ := publicKey.(*ecdsa.PublicKey)

	address := crypto.PubkeyToAddress(*publicKeyECDSA)

	return address
}

func DeduplicateAddresses(addresses []*common.Address) []*common.Address {
	seen := make(map[common.Address]struct{})

	result := make([]*common.Address, 0)

	for _, addr := range addresses {
		if _, exists := seen[*addr]; !exists {
			seen[*addr] = struct{}{}
			result = append(result, addr)
		}
	}

	return result
}

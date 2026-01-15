package trie

import (
	"bytes"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

func newTestFullNode(v []byte) []interface{} {
	fullNodeData := []interface{}{}
	for i := 0; i < 16; i++ {
		k := bytes.Repeat([]byte{byte(i + 1)}, 32)
		fullNodeData = append(fullNodeData, k)
	}
	fullNodeData = append(fullNodeData, v)
	return fullNodeData
}

func TestDecodeNestedNode(t *testing.T) {
	fullNodeData := newTestFullNode([]byte("fullnode"))

	data := [][]byte{}
	for i := 0; i < 16; i++ {
		data = append(data, nil)
	}
	data = append(data, []byte("subnode"))
	fullNodeData[15] = data

	buf := bytes.NewBuffer([]byte{})
	rlp.Encode(buf, fullNodeData)

	if _, err := decodeNode([]byte("testdecode"), buf.Bytes()); err != nil {
		t.Fatalf("decode nested full node err: %v", err)
	}
}

func TestDecodeFullNodeWrongSizeChild(t *testing.T) {
	fullNodeData := newTestFullNode([]byte("wrongsizechild"))
	fullNodeData[0] = []byte("00")
	buf := bytes.NewBuffer([]byte{})
	rlp.Encode(buf, fullNodeData)

	_, err := decodeNode([]byte("testdecode"), buf.Bytes())
	if _, ok := err.(*decodeError); !ok {
		t.Fatalf("decodeNode returned wrong err: %v", err)
	}
}

func TestDecodeFullNodeWrongNestedFullNode(t *testing.T) {
	fullNodeData := newTestFullNode([]byte("fullnode"))

	data := [][]byte{}
	for i := 0; i < 16; i++ {
		data = append(data, []byte("123456"))
	}
	data = append(data, []byte("subnode"))
	fullNodeData[15] = data

	buf := bytes.NewBuffer([]byte{})
	rlp.Encode(buf, fullNodeData)

	_, err := decodeNode([]byte("testdecode"), buf.Bytes())
	if _, ok := err.(*decodeError); !ok {
		t.Fatalf("decodeNode returned wrong err: %v", err)
	}
}

func TestDecodeFullNode(t *testing.T) {
	fullNodeData := newTestFullNode([]byte("decodefullnode"))
	buf := bytes.NewBuffer([]byte{})
	rlp.Encode(buf, fullNodeData)

	_, err := decodeNode([]byte("testdecode"), buf.Bytes())
	if err != nil {
		t.Fatalf("decode full node err: %v", err)
	}
}

func BenchmarkEncodeShortNode(b *testing.B) {
	node := &shortNode{
		Key: []byte{0x1, 0x2},
		Val: hashNode(randBytes(32)),
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		nodeToBytes(node)
	}
}

func BenchmarkEncodeFullNode(b *testing.B) {
	node := &fullNode{}
	for i := 0; i < 16; i++ {
		node.Children[i] = hashNode(randBytes(32))
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		nodeToBytes(node)
	}
}

func BenchmarkDecodeShortNode(b *testing.B) {
	node := &shortNode{
		Key: []byte{0x1, 0x2},
		Val: hashNode(randBytes(32)),
	}
	blob := nodeToBytes(node)
	hash := crypto.Keccak256(blob)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mustDecodeNode(hash, blob)
	}
}

func BenchmarkDecodeShortNodeUnsafe(b *testing.B) {
	node := &shortNode{
		Key: []byte{0x1, 0x2},
		Val: hashNode(randBytes(32)),
	}
	blob := nodeToBytes(node)
	hash := crypto.Keccak256(blob)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mustDecodeNodeUnsafe(hash, blob)
	}
}

func BenchmarkDecodeFullNode(b *testing.B) {
	node := &fullNode{}
	for i := 0; i < 16; i++ {
		node.Children[i] = hashNode(randBytes(32))
	}
	blob := nodeToBytes(node)
	hash := crypto.Keccak256(blob)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mustDecodeNode(hash, blob)
	}
}

func BenchmarkDecodeFullNodeUnsafe(b *testing.B) {
	node := &fullNode{}
	for i := 0; i < 16; i++ {
		node.Children[i] = hashNode(randBytes(32))
	}
	blob := nodeToBytes(node)
	hash := crypto.Keccak256(blob)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mustDecodeNodeUnsafe(hash, blob)
	}
}

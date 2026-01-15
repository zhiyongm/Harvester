package trie

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

type Reader interface {
	Node(owner common.Hash, path []byte, hash common.Hash) (node, error)

	NodeBlob(owner common.Hash, path []byte, hash common.Hash) ([]byte, error)
}

type NodeReader interface {
	GetReader(root common.Hash) Reader
}

type trieReader struct {
	owner  common.Hash
	reader Reader
	banned map[string]struct{}
}

func newTrieReader(stateRoot, owner common.Hash, db NodeReader) (*trieReader, error) {
	reader := db.GetReader(stateRoot)
	if reader == nil {
		return nil, fmt.Errorf("state not found #%x", stateRoot)
	}
	return &trieReader{owner: owner, reader: reader}, nil
}

func newEmptyReader() *trieReader {
	return &trieReader{}
}

func (r *trieReader) node(path []byte, hash common.Hash) (node, error) {
	if r.banned != nil {
		if _, ok := r.banned[string(path)]; ok {
			return nil, &MissingNodeError{Owner: r.owner, NodeHash: hash, Path: path}
		}
	}
	if r.reader == nil {
		return nil, &MissingNodeError{Owner: r.owner, NodeHash: hash, Path: path}
	}
	node, err := r.reader.Node(r.owner, path, hash)
	if err != nil || node == nil {
		return nil, &MissingNodeError{Owner: r.owner, NodeHash: hash, Path: path, err: err}
	}
	return node, nil
}

func (r *trieReader) nodeBlob(path []byte, hash common.Hash) ([]byte, error) {
	if r.banned != nil {
		if _, ok := r.banned[string(path)]; ok {
			return nil, &MissingNodeError{Owner: r.owner, NodeHash: hash, Path: path}
		}
	}
	if r.reader == nil {
		return nil, &MissingNodeError{Owner: r.owner, NodeHash: hash, Path: path}
	}
	blob, err := r.reader.NodeBlob(r.owner, path, hash)
	if err != nil || len(blob) == 0 {
		return nil, &MissingNodeError{Owner: r.owner, NodeHash: hash, Path: path, err: err}
	}
	return blob, nil
}

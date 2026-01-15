package trie

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

type MissingNodeError struct {
	Owner    common.Hash
	NodeHash common.Hash
	Path     []byte
	err      error
}

func (err *MissingNodeError) Unwrap() error {
	return err.err
}

func (err *MissingNodeError) Error() string {
	if err.Owner == (common.Hash{}) {
		return fmt.Sprintf("missing trie node %x (path %x) %v", err.NodeHash, err.Path, err.err)
	}
	return fmt.Sprintf("missing trie node %x (owner %x) (path %x) %v", err.NodeHash, err.Owner, err.Path, err.err)
}

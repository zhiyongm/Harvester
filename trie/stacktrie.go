package trie

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"errors"
	"io"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

var ErrCommitDisabled = errors.New("no database for committing")

var stPool = sync.Pool{
	New: func() interface{} {
		return NewStackTrie(nil)
	},
}

type NodeWriteFunc = func(owner common.Hash, path []byte, hash common.Hash, blob []byte)

func stackTrieFromPool(writeFn NodeWriteFunc, owner common.Hash) *StackTrie {
	st := stPool.Get().(*StackTrie)
	st.owner = owner
	st.writeFn = writeFn
	return st
}

func returnToPool(st *StackTrie) {
	st.Reset()
	stPool.Put(st)
}

type StackTrie struct {
	owner    common.Hash
	nodeType uint8
	val      []byte
	key      []byte
	children [16]*StackTrie
	writeFn  NodeWriteFunc
}

func NewStackTrie(writeFn NodeWriteFunc) *StackTrie {
	return &StackTrie{
		nodeType: emptyNode,
		writeFn:  writeFn,
	}
}

func NewStackTrieWithOwner(writeFn NodeWriteFunc, owner common.Hash) *StackTrie {
	return &StackTrie{
		owner:    owner,
		nodeType: emptyNode,
		writeFn:  writeFn,
	}
}

func NewFromBinary(data []byte, writeFn NodeWriteFunc) (*StackTrie, error) {
	var st StackTrie
	if err := st.UnmarshalBinary(data); err != nil {
		return nil, err
	}
	if writeFn != nil {
		st.setWriter(writeFn)
	}
	return &st, nil
}

func (st *StackTrie) MarshalBinary() (data []byte, err error) {
	var (
		b bytes.Buffer
		w = bufio.NewWriter(&b)
	)
	if err := gob.NewEncoder(w).Encode(struct {
		Owner    common.Hash
		NodeType uint8
		Val      []byte
		Key      []byte
	}{
		st.owner,
		st.nodeType,
		st.val,
		st.key,
	}); err != nil {
		return nil, err
	}
	for _, child := range st.children {
		if child == nil {
			w.WriteByte(0)
			continue
		}
		w.WriteByte(1)
		if childData, err := child.MarshalBinary(); err != nil {
			return nil, err
		} else {
			w.Write(childData)
		}
	}
	w.Flush()
	return b.Bytes(), nil
}

func (st *StackTrie) UnmarshalBinary(data []byte) error {
	r := bytes.NewReader(data)
	return st.unmarshalBinary(r)
}

func (st *StackTrie) unmarshalBinary(r io.Reader) error {
	var dec struct {
		Owner    common.Hash
		NodeType uint8
		Val      []byte
		Key      []byte
	}
	if err := gob.NewDecoder(r).Decode(&dec); err != nil {
		return err
	}
	st.owner = dec.Owner
	st.nodeType = dec.NodeType
	st.val = dec.Val
	st.key = dec.Key

	var hasChild = make([]byte, 1)
	for i := range st.children {
		if _, err := r.Read(hasChild); err != nil {
			return err
		} else if hasChild[0] == 0 {
			continue
		}
		var child StackTrie
		if err := child.unmarshalBinary(r); err != nil {
			return err
		}
		st.children[i] = &child
	}
	return nil
}

func (st *StackTrie) setWriter(writeFn NodeWriteFunc) {
	st.writeFn = writeFn
	for _, child := range st.children {
		if child != nil {
			child.setWriter(writeFn)
		}
	}
}

func newLeaf(owner common.Hash, key, val []byte, writeFn NodeWriteFunc) *StackTrie {
	st := stackTrieFromPool(writeFn, owner)
	st.nodeType = leafNode
	st.key = append(st.key, key...)
	st.val = val
	return st
}

func newExt(owner common.Hash, key []byte, child *StackTrie, writeFn NodeWriteFunc) *StackTrie {
	st := stackTrieFromPool(writeFn, owner)
	st.nodeType = extNode
	st.key = append(st.key, key...)
	st.children[0] = child
	return st
}

const (
	emptyNode = iota
	branchNode
	extNode
	leafNode
	hashedNode
)

func (st *StackTrie) Update(key, value []byte) error {
	k := keybytesToHex(key)
	if len(value) == 0 {
		panic("deletion not supported")
	}
	st.insert(k[:len(k)-1], value, nil)
	return nil
}

func (st *StackTrie) MustUpdate(key, value []byte) {
	if err := st.Update(key, value); err != nil {
		log.Error("Unhandled trie error in StackTrie.Update", "err", err)
	}
}

func (st *StackTrie) Reset() {
	st.owner = common.Hash{}
	st.writeFn = nil
	st.key = st.key[:0]
	st.val = nil
	for i := range st.children {
		st.children[i] = nil
	}
	st.nodeType = emptyNode
}

func (st *StackTrie) getDiffIndex(key []byte) int {
	for idx, nibble := range st.key {
		if nibble != key[idx] {
			return idx
		}
	}
	return len(st.key)
}

func (st *StackTrie) insert(key, value []byte, prefix []byte) {
	switch st.nodeType {
	case branchNode:
		idx := int(key[0])

		for i := idx - 1; i >= 0; i-- {
			if st.children[i] != nil {
				if st.children[i].nodeType != hashedNode {
					st.children[i].hash(append(prefix, byte(i)))
				}
				break
			}
		}

		if st.children[idx] == nil {
			st.children[idx] = newLeaf(st.owner, key[1:], value, st.writeFn)
		} else {
			st.children[idx].insert(key[1:], value, append(prefix, key[0]))
		}

	case extNode:
		diffidx := st.getDiffIndex(key)

		if diffidx == len(st.key) {
			st.children[0].insert(key[diffidx:], value, append(prefix, key[:diffidx]...))
			return
		}
		var n *StackTrie
		if diffidx < len(st.key)-1 {
			n = newExt(st.owner, st.key[diffidx+1:], st.children[0], st.writeFn)
			n.hash(append(prefix, st.key[:diffidx+1]...))
		} else {
			n = st.children[0]
			n.hash(append(prefix, st.key...))
		}
		var p *StackTrie
		if diffidx == 0 {
			st.children[0] = nil
			p = st
			st.nodeType = branchNode
		} else {
			st.children[0] = stackTrieFromPool(st.writeFn, st.owner)
			st.children[0].nodeType = branchNode
			p = st.children[0]
		}
		o := newLeaf(st.owner, key[diffidx+1:], value, st.writeFn)

		origIdx := st.key[diffidx]
		newIdx := key[diffidx]
		p.children[origIdx] = n
		p.children[newIdx] = o
		st.key = st.key[:diffidx]

	case leafNode:
		diffidx := st.getDiffIndex(key)

		if diffidx >= len(st.key) {
			panic("Trying to insert into existing key")
		}

		var p *StackTrie
		if diffidx == 0 {
			st.nodeType = branchNode
			p = st
			st.children[0] = nil
		} else {
			st.nodeType = extNode
			st.children[0] = NewStackTrieWithOwner(st.writeFn, st.owner)
			st.children[0].nodeType = branchNode
			p = st.children[0]
		}

		origIdx := st.key[diffidx]
		p.children[origIdx] = newLeaf(st.owner, st.key[diffidx+1:], st.val, st.writeFn)
		p.children[origIdx].hash(append(prefix, st.key[:diffidx+1]...))

		newIdx := key[diffidx]
		p.children[newIdx] = newLeaf(st.owner, key[diffidx+1:], value, st.writeFn)

		st.key = st.key[:diffidx]
		st.val = nil

	case emptyNode:
		st.nodeType = leafNode
		st.key = key
		st.val = value

	case hashedNode:
		panic("trying to insert into hash")

	default:
		panic("invalid type")
	}
}

func (st *StackTrie) hash(path []byte) {
	h := newHasher(false)
	defer returnHasherToPool(h)

	st.hashRec(h, path)
}

func (st *StackTrie) hashRec(hasher *hasher, path []byte) {
	var encodedNode []byte

	switch st.nodeType {
	case hashedNode:
		return

	case emptyNode:
		st.val = types.EmptyRootHash.Bytes()
		st.key = st.key[:0]
		st.nodeType = hashedNode
		return

	case branchNode:
		var nodes rawFullNode
		for i, child := range st.children {
			if child == nil {
				nodes[i] = nilValueNode
				continue
			}
			child.hashRec(hasher, append(path, byte(i)))
			if len(child.val) < 32 {
				nodes[i] = rawNode(child.val)
			} else {
				nodes[i] = hashNode(child.val)
			}

			st.children[i] = nil
			returnToPool(child)
		}

		nodes.encode(hasher.encbuf)
		encodedNode = hasher.encodedBytes()

	case extNode:
		st.children[0].hashRec(hasher, append(path, st.key...))

		n := rawShortNode{Key: hexToCompact(st.key)}
		if len(st.children[0].val) < 32 {
			n.Val = rawNode(st.children[0].val)
		} else {
			n.Val = hashNode(st.children[0].val)
		}

		n.encode(hasher.encbuf)
		encodedNode = hasher.encodedBytes()

		returnToPool(st.children[0])
		st.children[0] = nil

	case leafNode:
		st.key = append(st.key, byte(16))
		n := rawShortNode{Key: hexToCompact(st.key), Val: valueNode(st.val)}

		n.encode(hasher.encbuf)
		encodedNode = hasher.encodedBytes()

	default:
		panic("invalid node type")
	}

	st.nodeType = hashedNode
	st.key = st.key[:0]
	if len(encodedNode) < 32 {
		st.val = common.CopyBytes(encodedNode)
		return
	}

	st.val = hasher.hashData(encodedNode)
	if st.writeFn != nil {
		st.writeFn(st.owner, path, common.BytesToHash(st.val), encodedNode)
	}
}

func (st *StackTrie) Hash() (h common.Hash) {
	hasher := newHasher(false)
	defer returnHasherToPool(hasher)

	st.hashRec(hasher, nil)
	if len(st.val) == 32 {
		copy(h[:], st.val)
		return h
	}
	hasher.sha.Reset()
	hasher.sha.Write(st.val)
	hasher.sha.Read(h[:])
	return h
}

func (st *StackTrie) Commit() (h common.Hash, err error) {
	if st.writeFn == nil {
		return common.Hash{}, ErrCommitDisabled
	}
	hasher := newHasher(false)
	defer returnHasherToPool(hasher)

	st.hashRec(hasher, nil)
	if len(st.val) == 32 {
		copy(h[:], st.val)
		return h, nil
	}
	hasher.sha.Reset()
	hasher.sha.Write(st.val)
	hasher.sha.Read(h[:])

	st.writeFn(st.owner, nil, h, st.val)
	return h, nil
}

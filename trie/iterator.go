package trie

import (
	"bytes"
	"container/heap"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type NodeResolver func(owner common.Hash, path []byte, hash common.Hash) []byte

type Iterator struct {
	nodeIt NodeIterator

	Key   []byte
	Value []byte
	Err   error
}

func NewIterator(it NodeIterator) *Iterator {
	return &Iterator{
		nodeIt: it,
	}
}

func (it *Iterator) Next() bool {
	for it.nodeIt.Next(true) {
		if it.nodeIt.Leaf() {
			it.Key = it.nodeIt.LeafKey()
			it.Value = it.nodeIt.LeafBlob()
			return true
		}
	}
	it.Key = nil
	it.Value = nil
	it.Err = it.nodeIt.Error()
	return false
}

func (it *Iterator) Prove() [][]byte {
	return it.nodeIt.LeafProof()
}

type NodeIterator interface {
	Next(bool) bool

	Error() error

	Hash() common.Hash

	Parent() common.Hash

	Path() []byte

	NodeBlob() []byte

	Leaf() bool

	LeafKey() []byte

	LeafBlob() []byte

	LeafProof() [][]byte

	AddResolver(NodeResolver)
}

type nodeIteratorState struct {
	hash    common.Hash
	node    node
	parent  common.Hash
	index   int
	pathlen int
}

type nodeIterator struct {
	trie  *Trie
	stack []*nodeIteratorState
	path  []byte
	err   error

	resolver NodeResolver
}

var errIteratorEnd = errors.New("end of iteration")

type seekError struct {
	key []byte
	err error
}

func (e seekError) Error() string {
	return "seek error: " + e.err.Error()
}

func newNodeIterator(trie *Trie, start []byte) NodeIterator {
	if trie.Hash() == types.EmptyRootHash {
		return &nodeIterator{
			trie: trie,
			err:  errIteratorEnd,
		}
	}
	it := &nodeIterator{trie: trie}
	it.err = it.seek(start)
	return it
}

func (it *nodeIterator) AddResolver(resolver NodeResolver) {
	it.resolver = resolver
}

func (it *nodeIterator) Hash() common.Hash {
	if len(it.stack) == 0 {
		return common.Hash{}
	}
	return it.stack[len(it.stack)-1].hash
}

func (it *nodeIterator) Parent() common.Hash {
	if len(it.stack) == 0 {
		return common.Hash{}
	}
	return it.stack[len(it.stack)-1].parent
}

func (it *nodeIterator) Leaf() bool {
	return hasTerm(it.path)
}

func (it *nodeIterator) LeafKey() []byte {
	if len(it.stack) > 0 {
		if _, ok := it.stack[len(it.stack)-1].node.(valueNode); ok {
			return hexToKeybytes(it.path)
		}
	}
	panic("not at leaf")
}

func (it *nodeIterator) LeafBlob() []byte {
	if len(it.stack) > 0 {
		if node, ok := it.stack[len(it.stack)-1].node.(valueNode); ok {
			return node
		}
	}
	panic("not at leaf")
}

func (it *nodeIterator) LeafProof() [][]byte {
	if len(it.stack) > 0 {
		if _, ok := it.stack[len(it.stack)-1].node.(valueNode); ok {
			hasher := newHasher(false)
			defer returnHasherToPool(hasher)
			proofs := make([][]byte, 0, len(it.stack))

			for i, item := range it.stack[:len(it.stack)-1] {
				node, hashed := hasher.proofHash(item.node)
				if _, ok := hashed.(hashNode); ok || i == 0 {
					proofs = append(proofs, nodeToBytes(node))
				}
			}
			return proofs
		}
	}
	panic("not at leaf")
}

func (it *nodeIterator) Path() []byte {
	return it.path
}

func (it *nodeIterator) NodeBlob() []byte {
	if it.Hash() == (common.Hash{}) {
		return nil
	}
	blob, err := it.resolveBlob(it.Hash().Bytes(), it.Path())
	if err != nil {
		it.err = err
		return nil
	}
	return blob
}

func (it *nodeIterator) Error() error {
	if it.err == errIteratorEnd {
		return nil
	}
	if seek, ok := it.err.(seekError); ok {
		return seek.err
	}
	return it.err
}

func (it *nodeIterator) Next(descend bool) bool {
	if it.err == errIteratorEnd {
		return false
	}
	if seek, ok := it.err.(seekError); ok {
		if it.err = it.seek(seek.key); it.err != nil {
			return false
		}
	}
	state, parentIndex, path, err := it.peek(descend)
	it.err = err
	if it.err != nil {
		return false
	}
	it.push(state, parentIndex, path)
	return true
}

func (it *nodeIterator) seek(prefix []byte) error {
	key := keybytesToHex(prefix)
	key = key[:len(key)-1]
	for {
		state, parentIndex, path, err := it.peekSeek(key)
		if err == errIteratorEnd {
			return errIteratorEnd
		} else if err != nil {
			return seekError{prefix, err}
		} else if bytes.Compare(path, key) >= 0 {
			return nil
		}
		it.push(state, parentIndex, path)
	}
}

func (it *nodeIterator) init() (*nodeIteratorState, error) {
	root := it.trie.Hash()
	state := &nodeIteratorState{node: it.trie.root, index: -1}
	if root != types.EmptyRootHash {
		state.hash = root
	}
	return state, state.resolve(it, nil)
}

func (it *nodeIterator) peek(descend bool) (*nodeIteratorState, *int, []byte, error) {
	if len(it.stack) == 0 {
		state, err := it.init()
		return state, nil, nil, err
	}
	if !descend {
		it.pop()
	}

	for len(it.stack) > 0 {
		parent := it.stack[len(it.stack)-1]
		ancestor := parent.hash
		if (ancestor == common.Hash{}) {
			ancestor = parent.parent
		}
		state, path, ok := it.nextChild(parent, ancestor)
		if ok {
			if err := state.resolve(it, path); err != nil {
				return parent, &parent.index, path, err
			}
			return state, &parent.index, path, nil
		}
		it.pop()
	}
	return nil, nil, nil, errIteratorEnd
}

func (it *nodeIterator) peekSeek(seekKey []byte) (*nodeIteratorState, *int, []byte, error) {
	if len(it.stack) == 0 {
		state, err := it.init()
		return state, nil, nil, err
	}
	if !bytes.HasPrefix(seekKey, it.path) {
		it.pop()
	}

	for len(it.stack) > 0 {
		parent := it.stack[len(it.stack)-1]
		ancestor := parent.hash
		if (ancestor == common.Hash{}) {
			ancestor = parent.parent
		}
		state, path, ok := it.nextChildAt(parent, ancestor, seekKey)
		if ok {
			if err := state.resolve(it, path); err != nil {
				return parent, &parent.index, path, err
			}
			return state, &parent.index, path, nil
		}
		it.pop()
	}
	return nil, nil, nil, errIteratorEnd
}

func (it *nodeIterator) resolveHash(hash hashNode, path []byte) (node, error) {
	if it.resolver != nil {
		if blob := it.resolver(it.trie.owner, path, common.BytesToHash(hash)); len(blob) > 0 {
			if resolved, err := decodeNode(hash, blob); err == nil {
				return resolved, nil
			}
		}
	}
	return it.trie.reader.node(path, common.BytesToHash(hash))
}

func (it *nodeIterator) resolveBlob(hash hashNode, path []byte) ([]byte, error) {
	if it.resolver != nil {
		if blob := it.resolver(it.trie.owner, path, common.BytesToHash(hash)); len(blob) > 0 {
			return blob, nil
		}
	}
	return it.trie.reader.nodeBlob(path, common.BytesToHash(hash))
}

func (st *nodeIteratorState) resolve(it *nodeIterator, path []byte) error {
	if hash, ok := st.node.(hashNode); ok {
		resolved, err := it.resolveHash(hash, path)
		if err != nil {
			return err
		}
		st.node = resolved
		st.hash = common.BytesToHash(hash)
	}
	return nil
}

func findChild(n *fullNode, index int, path []byte, ancestor common.Hash) (node, *nodeIteratorState, []byte, int) {
	var (
		child     node
		state     *nodeIteratorState
		childPath []byte
	)
	for ; index < len(n.Children); index++ {
		if n.Children[index] != nil {
			child = n.Children[index]
			hash, _ := child.cache()
			state = &nodeIteratorState{
				hash:    common.BytesToHash(hash),
				node:    child,
				parent:  ancestor,
				index:   -1,
				pathlen: len(path),
			}
			childPath = append(childPath, path...)
			childPath = append(childPath, byte(index))
			return child, state, childPath, index
		}
	}
	return nil, nil, nil, 0
}

func (it *nodeIterator) nextChild(parent *nodeIteratorState, ancestor common.Hash) (*nodeIteratorState, []byte, bool) {
	switch node := parent.node.(type) {
	case *fullNode:
		if child, state, path, index := findChild(node, parent.index+1, it.path, ancestor); child != nil {
			parent.index = index - 1
			return state, path, true
		}
	case *shortNode:
		if parent.index < 0 {
			hash, _ := node.Val.cache()
			state := &nodeIteratorState{
				hash:    common.BytesToHash(hash),
				node:    node.Val,
				parent:  ancestor,
				index:   -1,
				pathlen: len(it.path),
			}
			path := append(it.path, node.Key...)
			return state, path, true
		}
	}
	return parent, it.path, false
}

func (it *nodeIterator) nextChildAt(parent *nodeIteratorState, ancestor common.Hash, key []byte) (*nodeIteratorState, []byte, bool) {
	switch n := parent.node.(type) {
	case *fullNode:
		child, state, path, index := findChild(n, parent.index+1, it.path, ancestor)
		if child == nil {
			return parent, it.path, false
		}
		if bytes.Compare(path, key) >= 0 {
			parent.index = index - 1
			return state, path, true
		}
		for {
			nextChild, nextState, nextPath, nextIndex := findChild(n, index+1, it.path, ancestor)
			if nextChild == nil || bytes.Compare(nextPath, key) >= 0 {
				parent.index = index - 1
				return state, path, true
			}
			state, path, index = nextState, nextPath, nextIndex
		}
	case *shortNode:
		if parent.index < 0 {
			hash, _ := n.Val.cache()
			state := &nodeIteratorState{
				hash:    common.BytesToHash(hash),
				node:    n.Val,
				parent:  ancestor,
				index:   -1,
				pathlen: len(it.path),
			}
			path := append(it.path, n.Key...)
			return state, path, true
		}
	}
	return parent, it.path, false
}

func (it *nodeIterator) push(state *nodeIteratorState, parentIndex *int, path []byte) {
	it.path = path
	it.stack = append(it.stack, state)
	if parentIndex != nil {
		*parentIndex++
	}
}

func (it *nodeIterator) pop() {
	last := it.stack[len(it.stack)-1]
	it.path = it.path[:last.pathlen]
	it.stack[len(it.stack)-1] = nil
	it.stack = it.stack[:len(it.stack)-1]
}

func compareNodes(a, b NodeIterator) int {
	if cmp := bytes.Compare(a.Path(), b.Path()); cmp != 0 {
		return cmp
	}
	if a.Leaf() && !b.Leaf() {
		return -1
	} else if b.Leaf() && !a.Leaf() {
		return 1
	}
	if cmp := bytes.Compare(a.Hash().Bytes(), b.Hash().Bytes()); cmp != 0 {
		return cmp
	}
	if a.Leaf() && b.Leaf() {
		return bytes.Compare(a.LeafBlob(), b.LeafBlob())
	}
	return 0
}

type differenceIterator struct {
	a, b  NodeIterator
	eof   bool
	count int
}

func NewDifferenceIterator(a, b NodeIterator) (NodeIterator, *int) {
	a.Next(true)
	it := &differenceIterator{
		a: a,
		b: b,
	}
	return it, &it.count
}

func (it *differenceIterator) Hash() common.Hash {
	return it.b.Hash()
}

func (it *differenceIterator) Parent() common.Hash {
	return it.b.Parent()
}

func (it *differenceIterator) Leaf() bool {
	return it.b.Leaf()
}

func (it *differenceIterator) LeafKey() []byte {
	return it.b.LeafKey()
}

func (it *differenceIterator) LeafBlob() []byte {
	return it.b.LeafBlob()
}

func (it *differenceIterator) LeafProof() [][]byte {
	return it.b.LeafProof()
}

func (it *differenceIterator) Path() []byte {
	return it.b.Path()
}

func (it *differenceIterator) NodeBlob() []byte {
	return it.b.NodeBlob()
}

func (it *differenceIterator) AddResolver(resolver NodeResolver) {
	panic("not implemented")
}

func (it *differenceIterator) Next(bool) bool {
	if !it.b.Next(true) {
		return false
	}
	it.count++

	if it.eof {
		return true
	}

	for {
		switch compareNodes(it.a, it.b) {
		case -1:
			if !it.a.Next(true) {
				it.eof = true
				return true
			}
			it.count++
		case 1:
			return true
		case 0:
			hasHash := it.a.Hash() == common.Hash{}
			if !it.b.Next(hasHash) {
				return false
			}
			it.count++
			if !it.a.Next(hasHash) {
				it.eof = true
				return true
			}
			it.count++
		}
	}
}

func (it *differenceIterator) Error() error {
	if err := it.a.Error(); err != nil {
		return err
	}
	return it.b.Error()
}

type nodeIteratorHeap []NodeIterator

func (h nodeIteratorHeap) Len() int            { return len(h) }
func (h nodeIteratorHeap) Less(i, j int) bool  { return compareNodes(h[i], h[j]) < 0 }
func (h nodeIteratorHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *nodeIteratorHeap) Push(x interface{}) { *h = append(*h, x.(NodeIterator)) }
func (h *nodeIteratorHeap) Pop() interface{} {
	n := len(*h)
	x := (*h)[n-1]
	*h = (*h)[0 : n-1]
	return x
}

type unionIterator struct {
	items *nodeIteratorHeap
	count int
}

func NewUnionIterator(iters []NodeIterator) (NodeIterator, *int) {
	h := make(nodeIteratorHeap, len(iters))
	copy(h, iters)
	heap.Init(&h)

	ui := &unionIterator{items: &h}
	return ui, &ui.count
}

func (it *unionIterator) Hash() common.Hash {
	return (*it.items)[0].Hash()
}

func (it *unionIterator) Parent() common.Hash {
	return (*it.items)[0].Parent()
}

func (it *unionIterator) Leaf() bool {
	return (*it.items)[0].Leaf()
}

func (it *unionIterator) LeafKey() []byte {
	return (*it.items)[0].LeafKey()
}

func (it *unionIterator) LeafBlob() []byte {
	return (*it.items)[0].LeafBlob()
}

func (it *unionIterator) LeafProof() [][]byte {
	return (*it.items)[0].LeafProof()
}

func (it *unionIterator) Path() []byte {
	return (*it.items)[0].Path()
}

func (it *unionIterator) NodeBlob() []byte {
	return (*it.items)[0].NodeBlob()
}

func (it *unionIterator) AddResolver(resolver NodeResolver) {
	panic("not implemented")
}

func (it *unionIterator) Next(descend bool) bool {
	if len(*it.items) == 0 {
		return false
	}

	least := heap.Pop(it.items).(NodeIterator)

	for len(*it.items) > 0 && ((!descend && bytes.HasPrefix((*it.items)[0].Path(), least.Path())) || compareNodes(least, (*it.items)[0]) == 0) {
		skipped := heap.Pop(it.items).(NodeIterator)
		if skipped.Next(skipped.Hash() == common.Hash{}) {
			it.count++
			heap.Push(it.items, skipped)
		}
	}
	if least.Next(descend) {
		it.count++
		heap.Push(it.items, least)
	}
	return len(*it.items) > 0
}

func (it *unionIterator) Error() error {
	for i := 0; i < len(*it.items); i++ {
		if err := (*it.items)[i].Error(); err != nil {
			return err
		}
	}
	return nil
}

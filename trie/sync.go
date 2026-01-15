package trie

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/prque"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

var ErrNotRequested = errors.New("not requested")

var ErrAlreadyProcessed = errors.New("already processed")

const maxFetchesPerDepth = 16384

type SyncPath [][]byte

func NewSyncPath(path []byte) SyncPath {
	if len(path) < 64 {
		return SyncPath{hexToCompact(path)}
	}
	return SyncPath{hexToKeybytes(path[:64]), hexToCompact(path[64:])}
}

type LeafCallback func(keys [][]byte, path []byte, leaf []byte, parent common.Hash, parentPath []byte) error

type nodeRequest struct {
	hash common.Hash
	path []byte
	data []byte

	parent   *nodeRequest
	deps     int
	callback LeafCallback
}

type codeRequest struct {
	hash    common.Hash
	path    []byte
	data    []byte
	parents []*nodeRequest
}

type NodeSyncResult struct {
	Path string
	Data []byte
}

type CodeSyncResult struct {
	Hash common.Hash
	Data []byte
}

type syncMemBatch struct {
	nodes  map[string][]byte
	hashes map[string]common.Hash
	codes  map[common.Hash][]byte
	size   uint64
}

func newSyncMemBatch() *syncMemBatch {
	return &syncMemBatch{
		nodes:  make(map[string][]byte),
		hashes: make(map[string]common.Hash),
		codes:  make(map[common.Hash][]byte),
	}
}

func (batch *syncMemBatch) hasNode(path []byte) bool {
	_, ok := batch.nodes[string(path)]
	return ok
}

func (batch *syncMemBatch) hasCode(hash common.Hash) bool {
	_, ok := batch.codes[hash]
	return ok
}

type Sync struct {
	scheme   string
	database ethdb.KeyValueReader
	membatch *syncMemBatch
	nodeReqs map[string]*nodeRequest
	codeReqs map[common.Hash]*codeRequest
	queue    *prque.Prque[int64, any]
	fetches  map[int]int
}

func NewSync(root common.Hash, database ethdb.KeyValueReader, callback LeafCallback, scheme string) *Sync {
	ts := &Sync{
		scheme:   scheme,
		database: database,
		membatch: newSyncMemBatch(),
		nodeReqs: make(map[string]*nodeRequest),
		codeReqs: make(map[common.Hash]*codeRequest),
		queue:    prque.New[int64, any](nil),
		fetches:  make(map[int]int),
	}
	ts.AddSubTrie(root, nil, common.Hash{}, nil, callback)
	return ts
}

func (s *Sync) AddSubTrie(root common.Hash, path []byte, parent common.Hash, parentPath []byte, callback LeafCallback) {
	if root == types.EmptyRootHash {
		return
	}
	if s.membatch.hasNode(path) {
		return
	}
	owner, inner := ResolvePath(path)
	if rawdb.HasTrieNode(s.database, owner, inner, root, s.scheme) {
		return
	}
	req := &nodeRequest{
		hash:     root,
		path:     path,
		callback: callback,
	}
	if parent != (common.Hash{}) {
		ancestor := s.nodeReqs[string(parentPath)]
		if ancestor == nil {
			panic(fmt.Sprintf("sub-trie ancestor not found: %x", parent))
		}
		ancestor.deps++
		req.parent = ancestor
	}
	s.scheduleNodeRequest(req)
}

func (s *Sync) AddCodeEntry(hash common.Hash, path []byte, parent common.Hash, parentPath []byte) {
	if hash == types.EmptyCodeHash {
		return
	}
	if s.membatch.hasCode(hash) {
		return
	}
	if rawdb.HasCodeWithPrefix(s.database, hash) {
		return
	}
	req := &codeRequest{
		path: path,
		hash: hash,
	}
	if parent != (common.Hash{}) {
		ancestor := s.nodeReqs[string(parentPath)]
		if ancestor == nil {
			panic(fmt.Sprintf("raw-entry ancestor not found: %x", parent))
		}
		ancestor.deps++
		req.parents = append(req.parents, ancestor)
	}
	s.scheduleCodeRequest(req)
}

func (s *Sync) Missing(max int) ([]string, []common.Hash, []common.Hash) {
	var (
		nodePaths  []string
		nodeHashes []common.Hash
		codeHashes []common.Hash
	)
	for !s.queue.Empty() && (max == 0 || len(nodeHashes)+len(codeHashes) < max) {
		item, prio := s.queue.Peek()

		depth := int(prio >> 56)
		if s.fetches[depth] > maxFetchesPerDepth {
			break
		}
		s.queue.Pop()
		s.fetches[depth]++

		switch item := item.(type) {
		case common.Hash:
			codeHashes = append(codeHashes, item)
		case string:
			req, ok := s.nodeReqs[item]
			if !ok {
				log.Error("Missing node request", "path", item)
				continue
			}
			nodePaths = append(nodePaths, item)
			nodeHashes = append(nodeHashes, req.hash)
		}
	}
	return nodePaths, nodeHashes, codeHashes
}

func (s *Sync) ProcessCode(result CodeSyncResult) error {
	req := s.codeReqs[result.Hash]
	if req == nil {
		return ErrNotRequested
	}
	if req.data != nil {
		return ErrAlreadyProcessed
	}
	req.data = result.Data
	return s.commitCodeRequest(req)
}

func (s *Sync) ProcessNode(result NodeSyncResult) error {
	req := s.nodeReqs[result.Path]
	if req == nil {
		return ErrNotRequested
	}
	if req.data != nil {
		return ErrAlreadyProcessed
	}
	node, err := decodeNode(req.hash.Bytes(), result.Data)
	if err != nil {
		return err
	}
	req.data = result.Data

	requests, err := s.children(req, node)
	if err != nil {
		return err
	}
	if len(requests) == 0 && req.deps == 0 {
		s.commitNodeRequest(req)
	} else {
		req.deps += len(requests)
		for _, child := range requests {
			s.scheduleNodeRequest(child)
		}
	}
	return nil
}

func (s *Sync) Commit(dbw ethdb.Batch) error {
	for path, value := range s.membatch.nodes {
		owner, inner := ResolvePath([]byte(path))
		rawdb.WriteTrieNode(dbw, owner, inner, s.membatch.hashes[path], value, s.scheme)
	}
	for hash, value := range s.membatch.codes {
		rawdb.WriteCode(dbw, hash, value)
	}
	s.membatch = newSyncMemBatch()
	return nil
}

func (s *Sync) MemSize() uint64 {
	return s.membatch.size
}

func (s *Sync) Pending() int {
	return len(s.nodeReqs) + len(s.codeReqs)
}

func (s *Sync) scheduleNodeRequest(req *nodeRequest) {
	s.nodeReqs[string(req.path)] = req

	prio := int64(len(req.path)) << 56
	for i := 0; i < 14 && i < len(req.path); i++ {
		prio |= int64(15-req.path[i]) << (52 - i*4)
	}
	s.queue.Push(string(req.path), prio)
}

func (s *Sync) scheduleCodeRequest(req *codeRequest) {
	if old, ok := s.codeReqs[req.hash]; ok {
		old.parents = append(old.parents, req.parents...)
		return
	}
	s.codeReqs[req.hash] = req

	prio := int64(len(req.path)) << 56
	for i := 0; i < 14 && i < len(req.path); i++ {
		prio |= int64(15-req.path[i]) << (52 - i*4)
	}
	s.queue.Push(req.hash, prio)
}

func (s *Sync) children(req *nodeRequest, object node) ([]*nodeRequest, error) {
	type childNode struct {
		path []byte
		node node
	}
	var children []childNode

	switch node := (object).(type) {
	case *shortNode:
		key := node.Key
		if hasTerm(key) {
			key = key[:len(key)-1]
		}
		children = []childNode{{
			node: node.Val,
			path: append(append([]byte(nil), req.path...), key...),
		}}
	case *fullNode:
		for i := 0; i < 17; i++ {
			if node.Children[i] != nil {
				children = append(children, childNode{
					node: node.Children[i],
					path: append(append([]byte(nil), req.path...), byte(i)),
				})
			}
		}
	default:
		panic(fmt.Sprintf("unknown node: %+v", node))
	}
	var (
		missing = make(chan *nodeRequest, len(children))
		pending sync.WaitGroup
	)
	for _, child := range children {
		if req.callback != nil {
			if node, ok := (child.node).(valueNode); ok {
				var paths [][]byte
				if len(child.path) == 2*common.HashLength {
					paths = append(paths, hexToKeybytes(child.path))
				} else if len(child.path) == 4*common.HashLength {
					paths = append(paths, hexToKeybytes(child.path[:2*common.HashLength]))
					paths = append(paths, hexToKeybytes(child.path[2*common.HashLength:]))
				}
				if err := req.callback(paths, child.path, node, req.hash, req.path); err != nil {
					return nil, err
				}
			}
		}
		if node, ok := (child.node).(hashNode); ok {
			if s.membatch.hasNode(child.path) {
				continue
			}
			pending.Add(1)
			go func(child childNode) {
				defer pending.Done()

				var (
					chash        = common.BytesToHash(node)
					owner, inner = ResolvePath(child.path)
				)
				if rawdb.HasTrieNode(s.database, owner, inner, chash, s.scheme) {
					return
				}
				missing <- &nodeRequest{
					path:     child.path,
					hash:     chash,
					parent:   req,
					callback: req.callback,
				}
			}(child)
		}
	}
	pending.Wait()

	requests := make([]*nodeRequest, 0, len(children))
	for done := false; !done; {
		select {
		case miss := <-missing:
			requests = append(requests, miss)
		default:
			done = true
		}
	}
	return requests, nil
}

func (s *Sync) commitNodeRequest(req *nodeRequest) error {
	s.membatch.nodes[string(req.path)] = req.data
	s.membatch.hashes[string(req.path)] = req.hash
	s.membatch.size += common.HashLength + uint64(len(req.data))
	delete(s.nodeReqs, string(req.path))
	s.fetches[len(req.path)]--

	if req.parent != nil {
		req.parent.deps--
		if req.parent.deps == 0 {
			if err := s.commitNodeRequest(req.parent); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Sync) commitCodeRequest(req *codeRequest) error {
	s.membatch.codes[req.hash] = req.data
	s.membatch.size += common.HashLength + uint64(len(req.data))
	delete(s.codeReqs, req.hash)
	s.fetches[len(req.path)]--

	for _, parent := range req.parents {
		parent.deps--
		if parent.deps == 0 {
			if err := s.commitNodeRequest(parent); err != nil {
				return err
			}
		}
	}
	return nil
}

func ResolvePath(path []byte) (common.Hash, []byte) {
	var owner common.Hash
	if len(path) >= 2*common.HashLength {
		owner = common.BytesToHash(hexToKeybytes(path[:2*common.HashLength]))
		path = path[2*common.HashLength:]
	}
	return owner, path
}

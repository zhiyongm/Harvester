package trie

import "github.com/ethereum/go-ethereum/common"

type tracer struct {
	inserts    map[string]struct{}
	deletes    map[string]struct{}
	accessList map[string][]byte
}

func newTracer() *tracer {
	return &tracer{
		inserts:    make(map[string]struct{}),
		deletes:    make(map[string]struct{}),
		accessList: make(map[string][]byte),
	}
}

func (t *tracer) onRead(path []byte, val []byte) {
	t.accessList[string(path)] = val
}

func (t *tracer) onInsert(path []byte) {
	if _, present := t.deletes[string(path)]; present {
		delete(t.deletes, string(path))
		return
	}
	t.inserts[string(path)] = struct{}{}
}

func (t *tracer) onDelete(path []byte) {
	if _, present := t.inserts[string(path)]; present {
		delete(t.inserts, string(path))
		return
	}
	t.deletes[string(path)] = struct{}{}
}

func (t *tracer) reset() {
	t.inserts = make(map[string]struct{})
	t.deletes = make(map[string]struct{})
	t.accessList = make(map[string][]byte)
}

func (t *tracer) copy() *tracer {
	var (
		inserts    = make(map[string]struct{})
		deletes    = make(map[string]struct{})
		accessList = make(map[string][]byte)
	)
	for path := range t.inserts {
		inserts[path] = struct{}{}
	}
	for path := range t.deletes {
		deletes[path] = struct{}{}
	}
	for path, blob := range t.accessList {
		accessList[path] = common.CopyBytes(blob)
	}
	return &tracer{
		inserts:    inserts,
		deletes:    deletes,
		accessList: accessList,
	}
}

func (t *tracer) markDeletions(set *NodeSet) {
	for path := range t.deletes {
		if _, ok := set.accessList[path]; !ok {
			continue
		}
		set.markDeleted([]byte(path))
	}
}

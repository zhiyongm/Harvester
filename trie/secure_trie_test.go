package trie

import (
	"bytes"
	"fmt"
	"runtime"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/crypto"
)

func newEmptySecure() *StateTrie {
	trie, _ := NewStateTrie(TrieID(common.Hash{}), NewDatabase(rawdb.NewMemoryDatabase()))
	return trie
}

func makeTestStateTrie() (*Database, *StateTrie, map[string][]byte) {
	triedb := NewDatabase(rawdb.NewMemoryDatabase())
	trie, _ := NewStateTrie(TrieID(common.Hash{}), triedb)

	content := make(map[string][]byte)
	for i := byte(0); i < 255; i++ {
		key, val := common.LeftPadBytes([]byte{1, i}, 32), []byte{i}
		content[string(key)] = val
		trie.MustUpdate(key, val)

		key, val = common.LeftPadBytes([]byte{2, i}, 32), []byte{i}
		content[string(key)] = val
		trie.MustUpdate(key, val)

		for j := byte(3); j < 13; j++ {
			key, val = common.LeftPadBytes([]byte{j, i}, 32), []byte{j, i}
			content[string(key)] = val
			trie.MustUpdate(key, val)
		}
	}
	root, nodes := trie.Commit(false)
	if err := triedb.Update(NewWithNodeSet(nodes)); err != nil {
		panic(fmt.Errorf("failed to commit db %v", err))
	}
	trie, _ = NewStateTrie(TrieID(root), triedb)
	return triedb, trie, content
}

func TestSecureDelete(t *testing.T) {
	trie := newEmptySecure()
	vals := []struct{ k, v string }{
		{"do", "verb"},
		{"ether", "wookiedoo"},
		{"horse", "stallion"},
		{"shaman", "horse"},
		{"doge", "coin"},
		{"ether", ""},
		{"dog", "puppy"},
		{"shaman", ""},
	}
	for _, val := range vals {
		if val.v != "" {
			trie.MustUpdate([]byte(val.k), []byte(val.v))
		} else {
			trie.MustDelete([]byte(val.k))
		}
	}
	hash := trie.Hash()
	exp := common.HexToHash("29b235a58c3c25ab83010c327d5932bcf05324b7d6b1185e650798034783ca9d")
	if hash != exp {
		t.Errorf("expected %x got %x", exp, hash)
	}
}

func TestSecureGetKey(t *testing.T) {
	trie := newEmptySecure()
	trie.MustUpdate([]byte("foo"), []byte("bar"))

	key := []byte("foo")
	value := []byte("bar")
	seckey := crypto.Keccak256(key)

	if !bytes.Equal(trie.MustGet(key), value) {
		t.Errorf("Get did not return bar")
	}
	if k := trie.GetKey(seckey); !bytes.Equal(k, key) {
		t.Errorf("GetKey returned %q, want %q", k, key)
	}
}

func TestStateTrieConcurrency(t *testing.T) {
	_, trie, _ := makeTestStateTrie()

	threads := runtime.NumCPU()
	tries := make([]*StateTrie, threads)
	for i := 0; i < threads; i++ {
		tries[i] = trie.Copy()
	}
	pend := new(sync.WaitGroup)
	pend.Add(threads)
	for i := 0; i < threads; i++ {
		go func(index int) {
			defer pend.Done()

			for j := byte(0); j < 255; j++ {
				key, val := common.LeftPadBytes([]byte{byte(index), 1, j}, 32), []byte{j}
				tries[index].MustUpdate(key, val)

				key, val = common.LeftPadBytes([]byte{byte(index), 2, j}, 32), []byte{j}
				tries[index].MustUpdate(key, val)

				for k := byte(3); k < 13; k++ {
					key, val = common.LeftPadBytes([]byte{byte(index), k, j}, 32), []byte{k, j}
					tries[index].MustUpdate(key, val)
				}
			}
			tries[index].Commit(false)
		}(i)
	}
	pend.Wait()
}

package trie

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
)

type preimageStore struct {
	lock          sync.RWMutex
	disk          ethdb.KeyValueStore
	preimages     map[common.Hash][]byte
	preimagesSize common.StorageSize
}

func newPreimageStore(disk ethdb.KeyValueStore) *preimageStore {
	return &preimageStore{
		disk:      disk,
		preimages: make(map[common.Hash][]byte),
	}
}

func (store *preimageStore) insertPreimage(preimages map[common.Hash][]byte) {
	store.lock.Lock()
	defer store.lock.Unlock()

	for hash, preimage := range preimages {
		if _, ok := store.preimages[hash]; ok {
			continue
		}
		store.preimages[hash] = preimage
		store.preimagesSize += common.StorageSize(common.HashLength + len(preimage))
	}
}

func (store *preimageStore) preimage(hash common.Hash) []byte {
	store.lock.RLock()
	preimage := store.preimages[hash]
	store.lock.RUnlock()

	if preimage != nil {
		return preimage
	}
	return rawdb.ReadPreimage(store.disk, hash)
}

func (store *preimageStore) commit(force bool) error {
	store.lock.Lock()
	defer store.lock.Unlock()

	if store.preimagesSize <= 4*1024*1024 && !force {
		return nil
	}
	batch := store.disk.NewBatch()
	rawdb.WritePreimages(batch, store.preimages)
	if err := batch.Write(); err != nil {
		return err
	}
	store.preimages, store.preimagesSize = make(map[common.Hash][]byte), 0
	return nil
}

func (store *preimageStore) size() common.StorageSize {
	store.lock.RLock()
	defer store.lock.RUnlock()

	return store.preimagesSize
}

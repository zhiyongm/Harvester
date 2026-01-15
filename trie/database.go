package trie

import (
	"blockcooker/statistic"
	"blockcooker/utils"
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	memcacheCleanHitMeter   = metrics.NewRegisteredMeter("trie/memcache/clean/hit", nil)
	memcacheCleanMissMeter  = metrics.NewRegisteredMeter("trie/memcache/clean/miss", nil)
	memcacheCleanReadMeter  = metrics.NewRegisteredMeter("trie/memcache/clean/read", nil)
	memcacheCleanWriteMeter = metrics.NewRegisteredMeter("trie/memcache/clean/write", nil)

	memcacheDirtyHitMeter   = metrics.NewRegisteredMeter("trie/memcache/dirty/hit", nil)
	memcacheDirtyMissMeter  = metrics.NewRegisteredMeter("trie/memcache/dirty/miss", nil)
	memcacheDirtyReadMeter  = metrics.NewRegisteredMeter("trie/memcache/dirty/read", nil)
	memcacheDirtyWriteMeter = metrics.NewRegisteredMeter("trie/memcache/dirty/write", nil)

	memcacheFlushTimeTimer  = metrics.NewRegisteredResettingTimer("trie/memcache/flush/time", nil)
	memcacheFlushNodesMeter = metrics.NewRegisteredMeter("trie/memcache/flush/nodes", nil)
	memcacheFlushSizeMeter  = metrics.NewRegisteredMeter("trie/memcache/flush/size", nil)

	memcacheGCTimeTimer  = metrics.NewRegisteredResettingTimer("trie/memcache/gc/time", nil)
	memcacheGCNodesMeter = metrics.NewRegisteredMeter("trie/memcache/gc/nodes", nil)
	memcacheGCSizeMeter  = metrics.NewRegisteredMeter("trie/memcache/gc/size", nil)

	memcacheCommitTimeTimer  = metrics.NewRegisteredResettingTimer("trie/memcache/commit/time", nil)
	memcacheCommitNodesMeter = metrics.NewRegisteredMeter("trie/memcache/commit/nodes", nil)
	memcacheCommitSizeMeter  = metrics.NewRegisteredMeter("trie/memcache/commit/size", nil)
)

type Database struct {
	diskdb ethdb.Database

	cleans *fastcache.Cache

	dirties map[common.Hash]*cachedNode
	oldest  common.Hash
	newest  common.Hash

	gctime  time.Duration
	gcnodes uint64
	gcsize  common.StorageSize

	flushtime  time.Duration
	flushnodes uint64
	flushsize  common.StorageSize

	dirtiesSize  common.StorageSize
	childrenSize common.StorageSize
	preimages    *preimageStore

	lock sync.RWMutex

	ChainConfig *utils.Config
	Statistic   *statistic.ChainStatistic
}

type rawNode []byte

func (n rawNode) cache() (hashNode, bool)   { panic("this should never end up in a live trie") }
func (n rawNode) fstring(ind string) string { panic("this should never end up in a live trie") }

func (n rawNode) EncodeRLP(w io.Writer) error {
	_, err := w.Write(n)
	return err
}

type rawFullNode [17]node

func (n rawFullNode) cache() (hashNode, bool)   { panic("this should never end up in a live trie") }
func (n rawFullNode) fstring(ind string) string { panic("this should never end up in a live trie") }

func (n rawFullNode) EncodeRLP(w io.Writer) error {
	eb := rlp.NewEncoderBuffer(w)
	n.encode(eb)
	return eb.Flush()
}

type rawShortNode struct {
	Key []byte
	Val node
}

func (n rawShortNode) cache() (hashNode, bool)   { panic("this should never end up in a live trie") }
func (n rawShortNode) fstring(ind string) string { panic("this should never end up in a live trie") }

type cachedNode struct {
	node node
	size uint16

	parents  uint32
	children map[common.Hash]uint16

	flushPrev common.Hash
	flushNext common.Hash
}

var cachedNodeSize = int(reflect.TypeOf(cachedNode{}).Size())

const cachedNodeChildrenSize = 48

func (n *cachedNode) rlp() []byte {
	if node, ok := n.node.(rawNode); ok {
		return node
	}
	return nodeToBytes(n.node)
}

func (n *cachedNode) obj(hash common.Hash) node {
	if node, ok := n.node.(rawNode); ok {
		return mustDecodeNodeUnsafe(hash[:], node)
	}
	return expandNode(hash[:], n.node)
}

func (n *cachedNode) forChilds(onChild func(hash common.Hash)) {
	for child := range n.children {
		onChild(child)
	}
	if _, ok := n.node.(rawNode); !ok {
		forGatherChildren(n.node, onChild)
	}
}

func forGatherChildren(n node, onChild func(hash common.Hash)) {
	switch n := n.(type) {
	case *rawShortNode:
		forGatherChildren(n.Val, onChild)
	case rawFullNode:
		for i := 0; i < 16; i++ {
			forGatherChildren(n[i], onChild)
		}
	case hashNode:
		onChild(common.BytesToHash(n))
	case valueNode, nil, rawNode:
	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

func simplifyNode(n node) node {
	switch n := n.(type) {
	case *shortNode:
		return &rawShortNode{Key: n.Key, Val: simplifyNode(n.Val)}

	case *fullNode:
		node := rawFullNode(n.Children)
		for i := 0; i < len(node); i++ {
			if node[i] != nil {
				node[i] = simplifyNode(node[i])
			}
		}
		return node

	case valueNode, hashNode, rawNode:
		return n

	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

func expandNode(hash hashNode, n node) node {
	switch n := n.(type) {
	case *rawShortNode:
		return &shortNode{
			Key: compactToHex(n.Key),
			Val: expandNode(nil, n.Val),
			flags: nodeFlag{
				hash: hash,
			},
		}

	case rawFullNode:
		node := &fullNode{
			flags: nodeFlag{
				hash: hash,
			},
		}
		for i := 0; i < len(node.Children); i++ {
			if n[i] != nil {
				node.Children[i] = expandNode(nil, n[i])
			}
		}
		return node

	case valueNode, hashNode:
		return n

	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

type Config struct {
	Cache     int
	Journal   string
	Preimages bool
}

func NewDatabase(diskdb ethdb.Database) *Database {
	return NewDatabaseWithConfig(diskdb, nil, nil, nil)
}

func NewDatabaseWithConfig(diskdb ethdb.Database, config *Config, chainConfig *utils.Config, statistic *statistic.ChainStatistic) *Database {
	var cleans *fastcache.Cache
	if config != nil && config.Cache > 0 {
		if config.Journal == "" {
			cleans = fastcache.New(config.Cache * 1024 * 1024)
		} else {
			cleans = fastcache.LoadFromFileOrNew(config.Journal, config.Cache*1024*1024)
		}
	}
	var preimage *preimageStore
	if config != nil && config.Preimages {
		preimage = newPreimageStore(diskdb)
	}
	db := &Database{
		diskdb: diskdb,
		cleans: cleans,
		dirties: map[common.Hash]*cachedNode{{}: {
			children: make(map[common.Hash]uint16),
		}},
		preimages:   preimage,
		ChainConfig: chainConfig,
		Statistic:   statistic,
	}

	return db
}

func (db *Database) AddRandomVisitor() {

	go func() {
		buf := make([]byte, 8)
		if _, err := crand.Read(buf); err != nil {
			log.Error("Failed to initialize random seed", "error", err)
			return
		}
		sSeed := int64(binary.LittleEndian.Uint64(buf))
		source := rand.NewSource(sSeed)
		r := rand.New(source)
		var randomBytesChannel chan []byte = make(chan []byte, 10000)
		go func() {
			for {
				b := make([]byte, 1024)
				_, err := r.Read(b)
				if err != nil {
					log.Error("Failed to generate random bytes", "error", err)
				}
				randomBytesChannel <- b
			}
		}()

		for {
			time.Sleep(time.Microsecond)
			b := <-randomBytesChannel
			db.cleans.Set(b, b)
			db.cleans.Get(b, b)
		}

	}()

}

func (db *Database) insert(hash common.Hash, size int, node node) {
	if _, ok := db.dirties[hash]; ok {
		return
	}
	memcacheDirtyWriteMeter.Mark(int64(size))

	entry := &cachedNode{
		node:      node,
		size:      uint16(size),
		flushPrev: db.newest,
	}
	entry.forChilds(func(child common.Hash) {
		if c := db.dirties[child]; c != nil {
			c.parents++
		}
	})
	db.dirties[hash] = entry

	if db.oldest == (common.Hash{}) {
		db.oldest, db.newest = hash, hash
	} else {
		db.dirties[db.newest].flushNext, db.newest = hash, hash
	}
	db.dirtiesSize += common.StorageSize(common.HashLength + entry.size)
}

func (db *Database) node(hash common.Hash) node {
	if db.cleans != nil {
		if enc := db.cleans.Get(nil, hash[:]); enc != nil {
			memcacheCleanHitMeter.Mark(1)
			memcacheCleanReadMeter.Mark(int64(len(enc)))

			return mustDecodeNodeUnsafe(hash[:], enc)
		}
	}
	db.lock.RLock()
	dirty := db.dirties[hash]
	db.lock.RUnlock()

	if dirty != nil {
		memcacheDirtyHitMeter.Mark(1)
		memcacheDirtyReadMeter.Mark(int64(dirty.size))
		return dirty.obj(hash)
	}
	memcacheDirtyMissMeter.Mark(1)

	enc, err := db.diskdb.Get(hash[:])
	db.Statistic.ThisRoundLevelDBGetNum++
	db.Statistic.ThisBlockLevelDBGetNum++
	db.Statistic.TotalLevelDBGetNum++

	if err != nil || enc == nil {
		return nil
	}
	if db.cleans != nil {
		db.cleans.Set(hash[:], enc)
		memcacheCleanMissMeter.Mark(1)
		memcacheCleanWriteMeter.Mark(int64(len(enc)))
	}
	return mustDecodeNodeUnsafe(hash[:], enc)
}

func (db *Database) Node(hash common.Hash) ([]byte, error) {
	if hash == (common.Hash{}) {
		return nil, errors.New("not found")
	}
	if db.cleans != nil {
		if enc := db.cleans.Get(nil, hash[:]); enc != nil {
			memcacheCleanHitMeter.Mark(1)
			memcacheCleanReadMeter.Mark(int64(len(enc)))
			return enc, nil
		}
	}
	db.lock.RLock()
	dirty := db.dirties[hash]
	db.lock.RUnlock()

	if dirty != nil {
		memcacheDirtyHitMeter.Mark(1)
		memcacheDirtyReadMeter.Mark(int64(dirty.size))
		return dirty.rlp(), nil
	}
	memcacheDirtyMissMeter.Mark(1)

	enc := rawdb.ReadLegacyTrieNode(db.diskdb, hash)

	db.Statistic.TempLevelDBCounter++

	if len(enc) != 0 {
		if db.cleans != nil {
			db.cleans.Set(hash[:], enc)
			memcacheCleanMissMeter.Mark(1)
			memcacheCleanWriteMeter.Mark(int64(len(enc)))
		}
		return enc, nil
	}
	return nil, errors.New("not found")
}

func (db *Database) Nodes() []common.Hash {
	db.lock.RLock()
	defer db.lock.RUnlock()

	var hashes = make([]common.Hash, 0, len(db.dirties))
	for hash := range db.dirties {
		if hash != (common.Hash{}) {
			hashes = append(hashes, hash)
		}
	}
	return hashes
}

func (db *Database) Reference(child common.Hash, parent common.Hash) {
	db.lock.Lock()
	defer db.lock.Unlock()

	db.reference(child, parent)
}

func (db *Database) reference(child common.Hash, parent common.Hash) {
	node, ok := db.dirties[child]
	if !ok {
		return
	}
	if db.dirties[parent].children == nil {
		db.dirties[parent].children = make(map[common.Hash]uint16)
		db.childrenSize += cachedNodeChildrenSize
	} else if _, ok = db.dirties[parent].children[child]; ok && parent != (common.Hash{}) {
		return
	}
	node.parents++
	db.dirties[parent].children[child]++
	if db.dirties[parent].children[child] == 1 {
		db.childrenSize += common.HashLength + 2
	}
}

func (db *Database) Dereference(root common.Hash) {
	if root == (common.Hash{}) {
		log.Error("Attempted to dereference the trie cache meta root")
		return
	}
	db.lock.Lock()
	defer db.lock.Unlock()

	nodes, storage, start := len(db.dirties), db.dirtiesSize, time.Now()
	db.dereference(root, common.Hash{})

	db.gcnodes += uint64(nodes - len(db.dirties))
	db.gcsize += storage - db.dirtiesSize
	db.gctime += time.Since(start)

	memcacheGCTimeTimer.Update(time.Since(start))
	memcacheGCSizeMeter.Mark(int64(storage - db.dirtiesSize))
	memcacheGCNodesMeter.Mark(int64(nodes - len(db.dirties)))

	log.Debug("Dereferenced trie from memory database", "nodes", nodes-len(db.dirties), "size", storage-db.dirtiesSize, "time", time.Since(start),
		"gcnodes", db.gcnodes, "gcsize", db.gcsize, "gctime", db.gctime, "livenodes", len(db.dirties), "livesize", db.dirtiesSize)
}

func (db *Database) dereference(child common.Hash, parent common.Hash) {
	node := db.dirties[parent]

	if node.children != nil && node.children[child] > 0 {
		node.children[child]--
		if node.children[child] == 0 {
			delete(node.children, child)
			db.childrenSize -= (common.HashLength + 2)
		}
	}
	node, ok := db.dirties[child]
	if !ok {
		return
	}
	if node.parents > 0 {
		node.parents--
	}
	if node.parents == 0 {
		switch child {
		case db.oldest:
			db.oldest = node.flushNext
			db.dirties[node.flushNext].flushPrev = common.Hash{}
		case db.newest:
			db.newest = node.flushPrev
			db.dirties[node.flushPrev].flushNext = common.Hash{}
		default:
			db.dirties[node.flushPrev].flushNext = node.flushNext
			db.dirties[node.flushNext].flushPrev = node.flushPrev
		}
		node.forChilds(func(hash common.Hash) {
			db.dereference(hash, child)
		})
		delete(db.dirties, child)
		db.dirtiesSize -= common.StorageSize(common.HashLength + int(node.size))
		if node.children != nil {
			db.childrenSize -= cachedNodeChildrenSize
		}
	}
}

func (db *Database) Cap(limit common.StorageSize) error {
	nodes, storage, start := len(db.dirties), db.dirtiesSize, time.Now()
	batch := db.diskdb.NewBatch()

	size := db.dirtiesSize + common.StorageSize((len(db.dirties)-1)*cachedNodeSize)
	size += db.childrenSize - common.StorageSize(len(db.dirties[common.Hash{}].children)*(common.HashLength+2))

	if db.preimages != nil {
		if err := db.preimages.commit(false); err != nil {
			return err
		}
	}
	oldest := db.oldest
	for size > limit && oldest != (common.Hash{}) {
		node := db.dirties[oldest]
		rawdb.WriteLegacyTrieNode(batch, oldest, node.rlp())

		if batch.ValueSize() >= ethdb.IdealBatchSize {
			if err := batch.Write(); err != nil {
				log.Error("Failed to write flush list to disk", "err", err)
				return err
			}
			batch.Reset()
		}
		size -= common.StorageSize(common.HashLength + int(node.size) + cachedNodeSize)
		if node.children != nil {
			size -= common.StorageSize(cachedNodeChildrenSize + len(node.children)*(common.HashLength+2))
		}
		oldest = node.flushNext
	}
	if err := batch.Write(); err != nil {
		log.Error("Failed to write flush list to disk", "err", err)
		return err
	}
	db.lock.Lock()
	defer db.lock.Unlock()

	for db.oldest != oldest {
		node := db.dirties[db.oldest]
		delete(db.dirties, db.oldest)
		db.oldest = node.flushNext

		db.dirtiesSize -= common.StorageSize(common.HashLength + int(node.size))
		if node.children != nil {
			db.childrenSize -= common.StorageSize(cachedNodeChildrenSize + len(node.children)*(common.HashLength+2))
		}
	}
	if db.oldest != (common.Hash{}) {
		db.dirties[db.oldest].flushPrev = common.Hash{}
	}
	db.flushnodes += uint64(nodes - len(db.dirties))
	db.flushsize += storage - db.dirtiesSize
	db.flushtime += time.Since(start)

	memcacheFlushTimeTimer.Update(time.Since(start))
	memcacheFlushSizeMeter.Mark(int64(storage - db.dirtiesSize))
	memcacheFlushNodesMeter.Mark(int64(nodes - len(db.dirties)))

	log.Debug("Persisted nodes from memory database", "nodes", nodes-len(db.dirties), "size", storage-db.dirtiesSize, "time", time.Since(start),
		"flushnodes", db.flushnodes, "flushsize", db.flushsize, "flushtime", db.flushtime, "livenodes", len(db.dirties), "livesize", db.dirtiesSize)

	return nil
}

func (db *Database) Commit(node common.Hash, report bool) error {
	start := time.Now()
	batch := db.diskdb.NewBatch()

	if db.preimages != nil {
		if err := db.preimages.commit(true); err != nil {
			return err
		}
	}
	nodes, storage := len(db.dirties), db.dirtiesSize

	uncacher := &cleaner{db}
	if err := db.commit(node, batch, uncacher); err != nil {
		log.Error("Failed to commit trie from trie database", "err", err)
		return err
	}
	if err := batch.Write(); err != nil {
		log.Error("Failed to write trie to disk", "err", err)
		return err
	}
	db.lock.Lock()
	defer db.lock.Unlock()
	if err := batch.Replay(uncacher); err != nil {
		return err
	}
	batch.Reset()

	memcacheCommitTimeTimer.Update(time.Since(start))
	memcacheCommitSizeMeter.Mark(int64(storage - db.dirtiesSize))
	memcacheCommitNodesMeter.Mark(int64(nodes - len(db.dirties)))

	logger := log.Info
	if !report {
		logger = log.Debug
	}
	logger("Persisted trie from memory database", "nodes", nodes-len(db.dirties)+int(db.flushnodes), "size", storage-db.dirtiesSize+db.flushsize, "time", time.Since(start)+db.flushtime,
		"gcnodes", db.gcnodes, "gcsize", db.gcsize, "gctime", db.gctime, "livenodes", len(db.dirties), "livesize", db.dirtiesSize)

	db.gcnodes, db.gcsize, db.gctime = 0, 0, 0
	db.flushnodes, db.flushsize, db.flushtime = 0, 0, 0

	return nil
}

func (db *Database) commit(hash common.Hash, batch ethdb.Batch, uncacher *cleaner) error {
	node, ok := db.dirties[hash]
	if !ok {
		return nil
	}
	var err error
	node.forChilds(func(child common.Hash) {
		if err == nil {
			err = db.commit(child, batch, uncacher)
		}
	})
	if err != nil {
		return err
	}
	rawdb.WriteLegacyTrieNode(batch, hash, node.rlp())
	if batch.ValueSize() >= ethdb.IdealBatchSize {
		if err := batch.Write(); err != nil {
			return err
		}
		db.lock.Lock()
		err := batch.Replay(uncacher)
		batch.Reset()
		db.lock.Unlock()
		if err != nil {
			return err
		}
	}
	return nil
}

type cleaner struct {
	db *Database
}

func (c *cleaner) Put(key []byte, rlp []byte) error {
	hash := common.BytesToHash(key)

	node, ok := c.db.dirties[hash]
	if !ok {
		return nil
	}
	switch hash {
	case c.db.oldest:
		c.db.oldest = node.flushNext
		c.db.dirties[node.flushNext].flushPrev = common.Hash{}
	case c.db.newest:
		c.db.newest = node.flushPrev
		c.db.dirties[node.flushPrev].flushNext = common.Hash{}
	default:
		c.db.dirties[node.flushPrev].flushNext = node.flushNext
		c.db.dirties[node.flushNext].flushPrev = node.flushPrev
	}
	delete(c.db.dirties, hash)
	c.db.dirtiesSize -= common.StorageSize(common.HashLength + int(node.size))
	if node.children != nil {
		c.db.childrenSize -= common.StorageSize(cachedNodeChildrenSize + len(node.children)*(common.HashLength+2))
	}
	if c.db.cleans != nil {
		c.db.cleans.Set(hash[:], rlp)
		memcacheCleanWriteMeter.Mark(int64(len(rlp)))
	}
	return nil
}

func (c *cleaner) Delete(key []byte) error {
	panic("not implemented")
}

func (db *Database) Update(nodes *MergedNodeSet) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	var order []common.Hash
	for owner := range nodes.sets {
		if owner == (common.Hash{}) {
			continue
		}
		order = append(order, owner)
	}
	if _, ok := nodes.sets[common.Hash{}]; ok {
		order = append(order, common.Hash{})
	}
	for _, owner := range order {
		subset := nodes.sets[owner]
		subset.forEachWithOrder(func(path string, n *memoryNode) {
			if n.isDeleted() {
				return
			}
			db.insert(n.hash, int(n.size), n.node)
		})
	}
	if set, present := nodes.sets[common.Hash{}]; present {
		for _, n := range set.leaves {
			var account types.StateAccount
			if err := rlp.DecodeBytes(n.blob, &account); err != nil {
				return err
			}
			if account.Root != types.EmptyRootHash {
				db.reference(account.Root, n.parent)
			}
		}
	}
	return nil
}

func (db *Database) Size() (common.StorageSize, common.StorageSize) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	var metadataSize = common.StorageSize((len(db.dirties) - 1) * cachedNodeSize)
	var metarootRefs = common.StorageSize(len(db.dirties[common.Hash{}].children) * (common.HashLength + 2))
	var preimageSize common.StorageSize
	if db.preimages != nil {
		preimageSize = db.preimages.size()
	}
	return db.dirtiesSize + db.childrenSize + metadataSize - metarootRefs, preimageSize
}

func (db *Database) GetReader(root common.Hash) Reader {
	return newHashReader(db)
}

type hashReader struct {
	db *Database
}

func newHashReader(db *Database) *hashReader {
	return &hashReader{db: db}
}

func (reader *hashReader) Node(_ common.Hash, _ []byte, hash common.Hash) (node, error) {
	return reader.db.node(hash), nil
}

func (reader *hashReader) NodeBlob(_ common.Hash, _ []byte, hash common.Hash) ([]byte, error) {
	blob, _ := reader.db.Node(hash)
	return blob, nil
}

func (db *Database) saveCache(dir string, threads int) error {
	if db.cleans == nil {
		return nil
	}
	log.Info("Writing clean trie cache to disk", "path", dir, "threads", threads)

	start := time.Now()
	err := db.cleans.SaveToFileConcurrent(dir, threads)
	if err != nil {
		log.Error("Failed to persist clean trie cache", "error", err)
		return err
	}
	log.Info("Persisted the clean trie cache", "path", dir, "elapsed", common.PrettyDuration(time.Since(start)))
	return nil
}

func (db *Database) SaveCache(dir string) error {
	return db.saveCache(dir, runtime.GOMAXPROCS(0))
}

func (db *Database) SaveCachePeriodically(dir string, interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			db.saveCache(dir, 1)
		case <-stopCh:
			return
		}
	}
}

func (db *Database) CommitPreimages() error {
	db.lock.Lock()
	defer db.lock.Unlock()

	if db.preimages == nil {
		return nil
	}
	return db.preimages.commit(true)
}

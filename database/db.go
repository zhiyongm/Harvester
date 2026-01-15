package database

import (
	"bytes"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/bloom"
)

const (
	minCache = 16

	minHandles = 16

	metricsGatheringInterval = 3 * time.Second

	degradationWarnInterval = time.Minute
)

type Database struct {
	fn string
	db *pebble.DB

	quitLock sync.RWMutex
	quitChan chan chan error
	closed   bool

	activeComp    int
	compStartTime time.Time
	compTime      atomic.Int64
	level0Comp    atomic.Uint32
	nonLevel0Comp atomic.Uint32

	writeStalled        atomic.Bool
	writeDelayStartTime time.Time
	writeDelayCount     atomic.Int64
	writeDelayTime      atomic.Int64

	writeOptions *pebble.WriteOptions
}

func (d *Database) onCompactionBegin(info pebble.CompactionInfo) {
	if d.activeComp == 0 {
		d.compStartTime = time.Now()
	}
	l0 := info.Input[0]
	if l0.Level == 0 {
		d.level0Comp.Add(1)
	} else {
		d.nonLevel0Comp.Add(1)
	}
	d.activeComp++
}

func (d *Database) onCompactionEnd(info pebble.CompactionInfo) {
	if d.activeComp == 1 {
		d.compTime.Add(int64(time.Since(d.compStartTime)))
	} else if d.activeComp == 0 {
		panic("should not happen")
	}
	d.activeComp--
}

func (d *Database) onWriteStallBegin(b pebble.WriteStallBeginInfo) {
	d.writeDelayStartTime = time.Now()
	d.writeDelayCount.Add(1)
	d.writeStalled.Store(true)
}

func (d *Database) onWriteStallEnd() {
	d.writeDelayTime.Add(int64(time.Since(d.writeDelayStartTime)))
	d.writeStalled.Store(false)
}

type panicLogger struct{}

func (l panicLogger) Infof(format string, args ...interface{}) {
}

func (l panicLogger) Errorf(format string, args ...interface{}) {
}

func (l panicLogger) Fatalf(format string, args ...interface{}) {
	panic(fmt.Errorf("fatal: "+format, args...))
}

func New(file string, cache int, handles int, namespace string, readonly bool, ephemeral bool) (*Database, error) {
	if cache < minCache {
		cache = minCache
	}
	if handles < minHandles {
		handles = minHandles
	}

	maxMemTableSize := (1<<31)<<(^uint(0)>>63) - 1

	memTableLimit := 2
	memTableSize := cache * 1024 * 1024 / 2 / memTableLimit

	if memTableSize >= maxMemTableSize {
		memTableSize = maxMemTableSize - 1
	}
	db := &Database{
		fn:           file,
		quitChan:     make(chan chan error),
		writeOptions: &pebble.WriteOptions{Sync: !ephemeral},
	}
	opt := &pebble.Options{
		Cache:        pebble.NewCache(int64(cache * 1024 * 1024)),
		MaxOpenFiles: handles,

		MemTableSize:                memTableSize,
		MemTableStopWritesThreshold: memTableLimit,

		MaxConcurrentCompactions: runtime.NumCPU,

		Levels: []pebble.LevelOptions{
			{TargetFileSize: 2 * 1024 * 1024, FilterPolicy: bloom.FilterPolicy(10)},
			{TargetFileSize: 2 * 1024 * 1024, FilterPolicy: bloom.FilterPolicy(10)},
			{TargetFileSize: 2 * 1024 * 1024, FilterPolicy: bloom.FilterPolicy(10)},
			{TargetFileSize: 2 * 1024 * 1024, FilterPolicy: bloom.FilterPolicy(10)},
			{TargetFileSize: 2 * 1024 * 1024, FilterPolicy: bloom.FilterPolicy(10)},
			{TargetFileSize: 2 * 1024 * 1024, FilterPolicy: bloom.FilterPolicy(10)},
			{TargetFileSize: 2 * 1024 * 1024, FilterPolicy: bloom.FilterPolicy(10)},
		},
		ReadOnly: readonly,
	}
	opt.Experimental.ReadSamplingMultiplier = -1

	innerDB, err := pebble.Open(file, opt)
	if err != nil {
		return nil, err
	}
	db.db = innerDB

	return db, nil
}

func (d *Database) Close() error {
	d.quitLock.Lock()
	defer d.quitLock.Unlock()
	if d.closed {
		return nil
	}
	d.closed = true
	if d.quitChan != nil {
		errc := make(chan error)
		d.quitChan <- errc

		d.quitChan = nil
	}
	return d.db.Close()
}

func (d *Database) Has(key []byte) (bool, error) {
	d.quitLock.RLock()
	defer d.quitLock.RUnlock()
	if d.closed {
		return false, pebble.ErrClosed
	}
	_, closer, err := d.db.Get(key)
	if err == pebble.ErrNotFound {
		return false, nil
	} else if err != nil {
		return false, err
	}
	closer.Close()
	return true, nil
}

func (d *Database) Get(key []byte) ([]byte, error) {
	d.quitLock.RLock()
	defer d.quitLock.RUnlock()
	if d.closed {
		return nil, pebble.ErrClosed
	}
	dat, closer, err := d.db.Get(key)
	if err != nil {
		return nil, err
	}
	ret := make([]byte, len(dat))
	copy(ret, dat)
	closer.Close()
	return ret, nil
}

func (d *Database) Put(key []byte, value []byte) error {
	d.quitLock.RLock()
	defer d.quitLock.RUnlock()
	if d.closed {
		return pebble.ErrClosed
	}
	return d.db.Set(key, value, d.writeOptions)
}

func (d *Database) Delete(key []byte) error {
	d.quitLock.RLock()
	defer d.quitLock.RUnlock()
	if d.closed {
		return pebble.ErrClosed
	}
	return d.db.Delete(key, nil)
}

func upperBound(prefix []byte) (limit []byte) {
	for i := len(prefix) - 1; i >= 0; i-- {
		c := prefix[i]
		if c == 0xff {
			continue
		}
		limit = make([]byte, i+1)
		copy(limit, prefix)
		limit[i] = c + 1
		break
	}
	return limit
}

func (d *Database) Stat() (string, error) {
	return d.db.Metrics().String(), nil
}

func (d *Database) Compact(start []byte, limit []byte) error {
	if limit == nil {
		limit = bytes.Repeat([]byte{0xff}, 32)
	}
	return d.db.Compact(start, limit, true)
}

func (d *Database) Path() string {
	return d.fn
}

type batch struct {
	b    *pebble.Batch
	db   *Database
	size int
}

func (b *batch) Put(key, value []byte) error {
	b.b.Set(key, value, nil)
	b.size += len(key) + len(value)
	return nil
}

func (b *batch) Delete(key []byte) error {
	b.b.Delete(key, nil)
	b.size += len(key)
	return nil
}

func (b *batch) ValueSize() int {
	return b.size
}

func (b *batch) Write() error {
	b.db.quitLock.RLock()
	defer b.db.quitLock.RUnlock()
	if b.db.closed {
		return pebble.ErrClosed
	}
	return b.b.Commit(b.db.writeOptions)
}

func (b *batch) Reset() {
	b.b.Reset()
	b.size = 0
}

type pebbleIterator struct {
	iter     *pebble.Iterator
	moved    bool
	released bool
}

func (iter *pebbleIterator) Next() bool {
	if iter.moved {
		iter.moved = false
		return iter.iter.Valid()
	}
	return iter.iter.Next()
}

func (iter *pebbleIterator) Error() error {
	return iter.iter.Error()
}

func (iter *pebbleIterator) Key() []byte {
	return iter.iter.Key()
}

func (iter *pebbleIterator) Value() []byte {
	return iter.iter.Value()
}

func (iter *pebbleIterator) Release() {
	if !iter.released {
		iter.iter.Close()
		iter.released = true
	}
}

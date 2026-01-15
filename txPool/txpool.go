package txPool

import (
	"blockcooker/core"
	"sync"
	"time"
	"unsafe"
)

type TxPool struct {
	TxQueue  []*core.Transaction
	lock     sync.Mutex
	capacity int
	cond     *sync.Cond
}

func NewTxPool(capacity int) *TxPool {
	if capacity <= 0 {
		panic("txpool capacity must be positive")
	}
	pool := &TxPool{
		TxQueue:  make([]*core.Transaction, 0, capacity),
		capacity: capacity,
	}
	pool.cond = sync.NewCond(&pool.lock)
	return pool
}

func (txpool *TxPool) AddTx2Pool(tx *core.Transaction) {
	txpool.lock.Lock()
	defer txpool.lock.Unlock()

	for len(txpool.TxQueue) >= txpool.capacity {
		txpool.cond.Wait()
	}

	if tx.Time.IsZero() {
		tx.Time = time.Now()
	}
	txpool.TxQueue = append(txpool.TxQueue, tx)
}

func (txpool *TxPool) AddTxs2Pool(txs []*core.Transaction) {
	txpool.lock.Lock()
	defer txpool.lock.Unlock()

	for _, tx := range txs {
		for len(txpool.TxQueue) >= txpool.capacity {
			txpool.cond.Wait()
		}
		if tx.Time.IsZero() {
			tx.Time = time.Now()
		}
		txpool.TxQueue = append(txpool.TxQueue, tx)
	}
}

func (txpool *TxPool) AddTxs2Pool_Head(txs []*core.Transaction) {
	txpool.lock.Lock()
	defer txpool.lock.Unlock()

	for i := len(txs) - 1; i >= 0; i-- {
		tx := txs[i]
		for len(txpool.TxQueue) >= txpool.capacity {
			txpool.cond.Wait()
		}
		txpool.TxQueue = append([]*core.Transaction{tx}, txpool.TxQueue...)
	}
}

func (txpool *TxPool) PackTxs(max_txs uint64) []*core.Transaction {
	txpool.lock.Lock()
	defer txpool.lock.Unlock()

	txNum := max_txs
	if uint64(len(txpool.TxQueue)) < txNum {
		txNum = uint64(len(txpool.TxQueue))
	}

	if txNum == 0 {
		return nil
	}

	txs_Packed := txpool.TxQueue[:txNum]
	remainingTxs := make([]*core.Transaction, len(txpool.TxQueue)-int(txNum))
	copy(remainingTxs, txpool.TxQueue[txNum:])
	txpool.TxQueue = remainingTxs

	txpool.cond.Broadcast()

	return txs_Packed
}

func (txpool *TxPool) PackTxsWithBytes(max_bytes int) []*core.Transaction {
	txpool.lock.Lock()
	defer txpool.lock.Unlock()

	if len(txpool.TxQueue) == 0 {
		return nil
	}

	txNum := len(txpool.TxQueue)
	currentSize := 0
	for tx_idx, tx := range txpool.TxQueue {
		currentSize += int(unsafe.Sizeof(*tx))
		if currentSize > max_bytes {
			txNum = tx_idx
			break
		}
	}

	if txNum == 0 {
		return nil
	}

	txs_Packed := txpool.TxQueue[:txNum]
	remainingTxs := make([]*core.Transaction, len(txpool.TxQueue)-txNum)
	copy(remainingTxs, txpool.TxQueue[txNum:])
	txpool.TxQueue = remainingTxs

	txpool.cond.Broadcast()

	return txs_Packed
}

func (txpool *TxPool) GetLocked() {
	txpool.lock.Lock()
}

func (txpool *TxPool) GetUnlocked() {
	txpool.lock.Unlock()
}

func (txpool *TxPool) GetTxQueueLen() int {
	txpool.lock.Lock()
	defer txpool.lock.Unlock()
	return len(txpool.TxQueue)
}

func (txpool *TxPool) PackTxsByNextBlock() []*core.Transaction {
	txpool.lock.Lock()
	defer txpool.lock.Unlock()

	if len(txpool.TxQueue) == 0 {
		return nil
	}

	targetBlockNum := txpool.TxQueue[0].BlockNumber

	cutIdx := 0
	for i, tx := range txpool.TxQueue {
		if tx.BlockNumber != targetBlockNum {
			break
		}
		cutIdx = i + 1
	}

	txsPacked := txpool.TxQueue[:cutIdx]

	remainingTxs := make([]*core.Transaction, len(txpool.TxQueue)-cutIdx)
	copy(remainingTxs, txpool.TxQueue[cutIdx:])
	txpool.TxQueue = remainingTxs

	txpool.cond.Broadcast()

	return txsPacked
}

package behavior

import (
	"log"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type AccountScore struct {
	Addr  common.Address
	Score float64
}

type BlockRecord struct {
	BlockNumber uint64
	AccessedSet map[common.Address]struct{}
	Timestamp   time.Time
}

type QueueStats struct {
	CurrentBlockCount int
	OldestBlock       uint64
	NewestBlock       uint64
	TotalAccounts     int
	UniqueAccounts    int
}

type HarvesterQueue struct {
	sync.RWMutex
	ExpMode string

	maxBlocks int
	DecayK    float64
	queue     []*BlockRecord
}

func NewHarvesterQueue(maxBlocks int, k float64, Expmode string) *HarvesterQueue {
	return &HarvesterQueue{
		maxBlocks: maxBlocks,
		DecayK:    k,
		ExpMode:   Expmode,
		queue:     make([]*BlockRecord, 0, maxBlocks),
	}
}

func (hq *HarvesterQueue) PushBlock(blockNum uint64, accessedAddrs []*common.Address) {
	hq.Lock()
	defer hq.Unlock()

	uniqueSet := make(map[common.Address]struct{})
	for _, addr := range accessedAddrs {
		uniqueSet[*addr] = struct{}{}
	}

	record := &BlockRecord{
		BlockNumber: blockNum,
		AccessedSet: uniqueSet,
		Timestamp:   time.Now(),
	}

	hq.queue = append(hq.queue, record)

	if len(hq.queue) > hq.maxBlocks {
		hq.queue = hq.queue[1:]
	}
}

func (hq *HarvesterQueue) CalculateHotspots() []AccountScore {
	hq.RLock()
	defer hq.RUnlock()

	scoreMap := make(map[common.Address]float64)
	queueLen := len(hq.queue)

	for i, record := range hq.queue {
		distance := float64(queueLen - 1 - i)

		var weight float64
		if hq.ExpMode == "classic" {
			weight = 1
		} else if hq.ExpMode == "tpe" {
			weight = math.Exp(-hq.DecayK * distance)
		} else {
			log.Panic("Unknown ExpMode in HarvesterQueue:", hq.ExpMode)
		}

		for addr := range record.AccessedSet {

			scoreMap[addr] += weight
		}
	}

	result := make([]AccountScore, 0, len(scoreMap))
	for addr, score := range scoreMap {
		result = append(result, AccountScore{
			Addr:  addr,
			Score: score,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Score == result[j].Score {
			return result[i].Addr.Hex() < result[j].Addr.Hex()
		}
		return result[i].Score > result[j].Score
	})

	return result
}

func (hq *HarvesterQueue) GetStats() QueueStats {
	hq.RLock()
	defer hq.RUnlock()

	stats := QueueStats{
		CurrentBlockCount: len(hq.queue),
		TotalAccounts:     0,
		UniqueAccounts:    0,
	}

	if len(hq.queue) > 0 {
		stats.OldestBlock = hq.queue[0].BlockNumber
		stats.NewestBlock = hq.queue[len(hq.queue)-1].BlockNumber
	}

	uniqueMap := make(map[common.Address]struct{})
	for _, record := range hq.queue {
		stats.TotalAccounts += len(record.AccessedSet)
		for addr := range record.AccessedSet {
			uniqueMap[addr] = struct{}{}
		}
	}
	stats.UniqueAccounts = len(uniqueMap)

	return stats
}

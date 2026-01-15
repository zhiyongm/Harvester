package statistic

import "time"

type ChainStatistic struct {
	AccountStateGetNum          uint64
	AccountStateGetTimeConsumed time.Duration

	ThisRoundSnapShotGetNum uint64
	ThisRoundSnapShotHitNum uint64

	ThisBlockSnapShotGetNum uint64
	ThisBlockSnapShotHitNum uint64

	TotalSnapShotGetNum uint64
	TotalSnapShotHitNum uint64

	ThisBlockLevelDBGetNum uint64
	ThisRoundLevelDBGetNum uint64
	TotalLevelDBGetNum     uint64

	KValue float64

	PollutionWorkerNum int

	TempLevelDBCounter uint64

	Queue_pollution_rate float64
}

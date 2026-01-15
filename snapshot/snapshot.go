package snapshot

import (
	"blockcooker/core"

	"github.com/ethereum/go-ethereum/common"
)

type Snapshot struct {
	SnapshotMap map[common.Address]*core.AccountState
	Size        int
}

func (s *Snapshot) GetSize() int {
	return s.Size
}

func (s *Snapshot) UpdateSnapshot(address common.Address, accountState *core.AccountState) {
	if _, exists := s.SnapshotMap[address]; !exists {
		s.Size++
	}
	s.SnapshotMap[address] = accountState
}

func (s *Snapshot) GetAccountState(address common.Address) (*core.AccountState, bool) {

	accountState, exists := s.SnapshotMap[address]
	return accountState, exists
}

func (s *Snapshot) Clear() {
	s.SnapshotMap = make(map[common.Address]*core.AccountState)
	s.Size = 0
}

func NewSnapshot() *Snapshot {
	return &Snapshot{
		SnapshotMap: make(map[common.Address]*core.AccountState),
		Size:        0,
	}
}

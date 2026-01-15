package resultWriter

type ResultEntity struct {
	BlockNumber         uint64 `csv:"block_number"`
	InternalBlockNumber uint64 `csv:"internal_block_number"`
	BlockHash           string `csv:"block_hash"`
	TxNumber            uint64 `csv:"tx_number"`
	ActionType          string `csv:"action_type"`
	ProcessTime         int64  `csv:"process_time"`
	ModifyAccountCount  int    `csv:"modify_account_count"`
	AccountStateGetNum  uint64 `csv:"account_state_get_num"`

	ReceiveTime int64 `csv:"receive_time"`
	MessageSize int   `csv:"message_size"`

	KValue       float64 `csv:"k_value"`
	ExpMode      string  `csv:"exp_mode"`
	Pollution    bool    `csv:"pollution"`
	PollutionNum uint64  `csv:"pollution_num"`

	ThisBlockSnapShotGetNum uint64 `csv:"this_block_snapshot_get_num"`
	ThisBlockSnapShotHitNum uint64 `csv:"this_block_snapshot_hit_num"`

	ThisRoundSnapShotGetNum uint64 `csv:"this_round_snapshot_get_num"`
	ThisRoundSnapShotHitNum uint64 `csv:"this_round_snapshot_hit_num"`

	TotalSnapShotGetNum uint64 `csv:"total_snapshot_get_num"`
	TotalSnapShotHitNum uint64 `csv:"total_snapshot_hit_num"`

	ThisBlockLevelDBGetNum uint64 `csv:"this_block_leveldb_get_num"`
	ThisRoundLevelDBGetNum uint64 `csv:"this_round_leveldb_get_num"`
	TotalLevelDBGetNum     uint64 `csv:"total_leveldb_get_num"`
}

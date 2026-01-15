package utils

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
)

type Config struct {
	InitBalance       *big.Int
	NodeID            uint64 `json:"node_id"`
	ChainID           uint64 `json:"chain_id"`
	BlockSize         uint64 `json:"block_size"`
	BlockInterval     int    `json:"block_interval"`
	InjectSpeed       uint64 `json:"inject_speed"`
	StoragePath       string `json:"storage_root_path"`
	DropDatabase      bool   `json:"drop_database"`
	InputDatasetPath  string `json:"input_dataset_path"`
	OutputDatasetPath string `json:"output_dataset_path"`
	CacheSize         int    `json:"cache_size"`
	NodeMode          string `json:"node_mode"`
	TxGeneratorMode   string `json:"tx_generator_mode"`
	TxPoolSize        int    `json:"tx_pool_size"`

	PeerCount              int    `json:"peer_count"`
	ProposerAddress        string `json:"proposer_address"`
	P2PSenderListenAddress string `json:"p2p_sender_listen_address"`
	P2PSenderIDFilePath    string `json:"p2p_sender_id_file_path"`
	P2PTopic               string `json:"p2p_topic"`
	APIPort                int    `json:"api_port"`

	QueueSize              int     `json:"queue_size"`
	SnapshotSize           int     `json:"snapshot_size"`
	BloomCapacity          uint    `json:"bloom_capacity"`
	BloomFalsePositiveRate float64 `json:"bloom_false_positive_rate"`
	FlushBlockInterval     uint64  `json:"flush_block_interval"`
	ExpMode                string  `json:"exp_mode"`
	Pollution              bool    `json:"pollution"`

	ExpBlockCount uint64 `json:"exp_block_count"`

	PollutionAddressNum int `json:"pollution_address_num"`
	PollutionBlockNum   int `json:"pollution_block_num"`

	IsGenerate bool `json:"is_generate"`
}

func NewConfigFromJson(filePath string) *Config {

	if filePath == "" {
		filePath = "config_leader.json"
	}

	Init_Balance, _ := new(big.Int).SetString("100000000000000000000000000000000000000000000", 10)

	config := &Config{
		InitBalance: Init_Balance,
	}
	file, err := os.ReadFile(filePath)
	if err != nil {
		panic(fmt.Sprintf("无法读取配置文件 config_leader.json: %v", err))
	}

	err = json.Unmarshal(file, &config)
	if err != nil {
		panic(fmt.Sprintf("解析 JSON 配置失败: %v", err))
	}

	return config
}

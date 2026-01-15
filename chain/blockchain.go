package chain

import (
	"blockcooker/behavior"
	"blockcooker/bloom"
	"blockcooker/core"
	"blockcooker/resultWriter"
	"blockcooker/snapshot"
	"blockcooker/statistic"
	"blockcooker/storage"
	"blockcooker/tpe"
	"blockcooker/txGenerator"
	"blockcooker/txPool"
	"blockcooker/utils"
	"bytes"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"

	"blockcooker/trie"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
	log "github.com/sirupsen/logrus"
)

type BlockChain struct {
	GoroutineNum int
	db           ethdb.Database
	triedb       *trie.Database
	ChainConfig  *utils.Config

	CurrentBlock *core.Block
	Storage      *storage.Storage
	TxPool       *txPool.TxPool

	SignSimulator    *utils.SignSimulator
	StartBlockNumber uint64
	ProcessedTxCount uint64
	TxGenerator      txGenerator.TxGeneratorInterface
	ResultWriter     *resultWriter.ResultWriter

	ToBePackagedBlocks chan *core.Block

	Statistic *statistic.ChainStatistic

	trieDBPool []*trie.Database

	Behavior *behavior.HarvesterQueue
	Bloom    *bloom.BloomFilter
	Snapshot *snapshot.Snapshot

	NowInternalBlockNumber uint64

	PollutionAddresses []*common.Address
}

func GetTxTreeRoot(txs []*core.Transaction) []byte {
	triedb := trie.NewDatabase(rawdb.NewMemoryDatabase())
	transactionTree := trie.NewEmpty(triedb)
	for _, tx := range txs {
		transactionTree.Update(tx.TxHash.Bytes(), tx.Encode())
	}
	return transactionTree.Hash().Bytes()
}

func (bc *BlockChain) GenerateBlock(txs []*core.Transaction, blockNumber uint64) *core.Block {

	fixedTime := time.Date(2025, time.January, 1, 12, 0, 0, 0, time.UTC)

	bh := &core.BlockHeader{
		ParentBlockHash: bc.CurrentBlock.Hash,
		Number:          blockNumber,
		Time:            fixedTime,
	}
	if bc.NowInternalBlockNumber >= uint64(bc.ChainConfig.QueueSize) &&
		bc.NowInternalBlockNumber < uint64(bc.ChainConfig.QueueSize)+uint64(bc.ChainConfig.PollutionBlockNum) && !bc.ChainConfig.IsGenerate {
		var pollution_txs []*core.Transaction
		for i := 0; i < len(bc.PollutionAddresses); i++ {

			rand_tx := &core.Transaction{
				Sender:    *bc.PollutionAddresses[i],
				Recipient: *bc.PollutionAddresses[i],
				Value:     new(big.Int).SetInt64(1),
			}
			pollution_txs = append(pollution_txs, rand_tx)
			txs = pollution_txs
		}
	}

	if bc.NowInternalBlockNumber >= 5000+uint64(bc.ChainConfig.QueueSize) &&
		bc.NowInternalBlockNumber < 5000+uint64(bc.ChainConfig.QueueSize)+uint64(bc.ChainConfig.PollutionBlockNum) && !bc.ChainConfig.IsGenerate {
		var pollution_txs []*core.Transaction
		for i := 0; i < len(bc.PollutionAddresses); i++ {

			rand_tx := &core.Transaction{
				Sender:    *bc.PollutionAddresses[i],
				Recipient: *bc.PollutionAddresses[i],
				Value:     new(big.Int).SetInt64(1),
			}
			pollution_txs = append(pollution_txs, rand_tx)
			txs = pollution_txs
		}
	}
	account_addresses := make([]*common.Address, 0)
	for _, tx := range txs {
		account_addresses = append(account_addresses, &tx.Sender)
		account_addresses = append(account_addresses, &tx.Recipient)
	}

	if bc.ChainConfig.ExpMode != "origin" {
		bc.Behavior.PushBlock(blockNumber, account_addresses)
	}
	if bc.NowInternalBlockNumber < uint64(bc.ChainConfig.QueueSize) {
		log.Info("队列还没有填充满,等待先清空计数器")
		bc.Statistic.TotalSnapShotGetNum = 0
		bc.Statistic.ThisRoundSnapShotGetNum = 0
		bc.Statistic.TotalSnapShotHitNum = 0
		bc.Statistic.ThisRoundSnapShotHitNum = 0
		bc.Statistic.TotalLevelDBGetNum = 0
		bc.Statistic.ThisRoundLevelDBGetNum = 0

	}
	if bc.NowInternalBlockNumber%bc.ChainConfig.FlushBlockInterval == 0 && bc.ChainConfig.NodeMode == "leader" {

		hitRate := float64(bc.Statistic.ThisRoundSnapShotHitNum) / float64(bc.Statistic.ThisRoundSnapShotGetNum)

		if bc.NowInternalBlockNumber < uint64(bc.ChainConfig.QueueSize) {
			log.Info("队列还没有填充满,等待...")
			bc.Statistic.TotalSnapShotGetNum = 0
			bc.Statistic.ThisRoundSnapShotGetNum = 0
			bc.Statistic.TotalSnapShotHitNum = 0
			bc.Statistic.ThisRoundSnapShotHitNum = 0
			bc.Statistic.TotalLevelDBGetNum = 0
			bc.Statistic.ThisRoundLevelDBGetNum = 0
		} else {

			if bc.ChainConfig.ExpMode != "origin" {

				bc.Statistic.ThisRoundLevelDBGetNum = 0

				time_start := time.Now()
				newK, err := tpe.ReportHitRateAndGetNextK(hitRate)
				log.Info("Report hit rate:", hitRate, " get new k:", newK, " time consumed:", time.Since(time_start))
				bc.Statistic.KValue = newK
				bc.Statistic.ThisRoundSnapShotGetNum = 0
				bc.Statistic.ThisRoundSnapShotHitNum = 0
				bc.Statistic.ThisRoundLevelDBGetNum = 0
				if err != nil {
					log.Error("Failed to report hit rate and get new k:", err)
				} else {
					bc.Behavior.DecayK = newK
				}

				hotAccountScore := bc.Behavior.CalculateHotspots()

				topN := bc.ChainConfig.SnapshotSize
				if len(hotAccountScore) < topN {
					topN = len(hotAccountScore)
				}
				hotAccountScore = hotAccountScore[:topN]
				hotAccountAddresses := make([]common.Address, 0, topN)
				for _, acc := range hotAccountScore {
					hotAccountAddresses = append(hotAccountAddresses, acc.Addr)
				}

				bc.Snapshot.Clear()

				st, err := trie.New(trie.TrieID(common.BytesToHash(bc.CurrentBlock.Header.StateRoot)), bc.triedb)

				for _, address := range hotAccountAddresses {

					state_enc, _ := st.Get(address.Bytes())
					if state_enc != nil {
						state := core.DecodeAS(state_enc)
						bc.Snapshot.UpdateSnapshot(address, state)
					}

				}

			}
		}
	}

	time_now := time.Now()
	rt, modify_account_count := bc.GetUpdateStateTrie(txs)
	bh.StateRoot = rt.Bytes()

	bh.Miner = bc.ChainConfig.NodeID

	if bc.Bloom != nil {
		bh.ForeSightBloom, _ = bc.Bloom.Encode()
	}

	b := core.NewBlock(bh, txs)
	b.Hash = b.Header.Hash()

	time_duration := time.Since(time_now)
	resultEntity := &resultWriter.ResultEntity{
		BlockNumber:         b.Header.Number,
		InternalBlockNumber: bc.NowInternalBlockNumber,
		BlockHash:           hex.EncodeToString(b.Hash),
		TxNumber:            uint64(len(b.Body)),
		ActionType:          "BlkGenerator",
		ProcessTime:         int64(time_duration.Milliseconds()),
		ModifyAccountCount:  modify_account_count,
		AccountStateGetNum:  atomic.LoadUint64(&bc.Statistic.AccountStateGetNum),

		KValue:       bc.Statistic.KValue,
		ExpMode:      bc.ChainConfig.ExpMode,
		Pollution:    bc.ChainConfig.Pollution,
		PollutionNum: uint64(bc.Statistic.PollutionWorkerNum),

		ThisBlockSnapShotGetNum: bc.Statistic.ThisBlockSnapShotGetNum,
		ThisBlockSnapShotHitNum: bc.Statistic.ThisBlockSnapShotHitNum,

		ThisRoundSnapShotGetNum: bc.Statistic.ThisRoundSnapShotGetNum,
		ThisRoundSnapShotHitNum: bc.Statistic.ThisRoundSnapShotHitNum,
		TotalSnapShotGetNum:     bc.Statistic.TotalSnapShotGetNum,
		TotalSnapShotHitNum:     bc.Statistic.TotalSnapShotHitNum,

		ThisBlockLevelDBGetNum: bc.Statistic.ThisBlockLevelDBGetNum,
		ThisRoundLevelDBGetNum: bc.Statistic.ThisRoundLevelDBGetNum,
		TotalLevelDBGetNum:     bc.Statistic.TotalLevelDBGetNum,

		ReceiveTime: time.Now().UnixMilli(),
	}

	bc.ResultWriter.CsvEntityChan <- resultEntity
	return b
}

func (bc *BlockChain) NewGenisisBlock() *core.Block {
	body := make([]*core.Transaction, 0)
	bh := &core.BlockHeader{
		Number: 0,
	}
	triedb := trie.NewDatabaseWithConfig(bc.db, &trie.Config{
		Cache:     bc.ChainConfig.CacheSize,
		Preimages: true,
	},
		bc.ChainConfig,
		bc.Statistic)

	bc.triedb = triedb
	statusTrie := trie.NewEmpty(triedb)
	bh.StateRoot = statusTrie.Hash().Bytes()
	bh.TxRoot = GetTxTreeRoot(body)
	b := core.NewBlock(bh, body)
	b.Hash = b.Header.Hash()
	return b
}

func (bc *BlockChain) AddGenisisBlock(gb *core.Block) {
	bc.Storage.AddBlock(gb)
	bc.CurrentBlock = gb
	fmt.Println(bc.PrintBlockChain())

}

func (bc *BlockChain) AddBlock(b *core.Block) {
	time_now := time.Now()

	if bc.CurrentBlock.Header.Number%bc.ChainConfig.FlushBlockInterval == 0 && bc.ChainConfig.NodeMode == "peer" {
		bc.Bloom, _ = bloom.DecodeNew(b.Header.ForeSightBloom)
		bc.Snapshot.Clear()
	}

	if b.Header.Number != bc.CurrentBlock.Header.Number+1 {
		fmt.Println("the block height is not correct")
		fmt.Println(b.Header.Number, "vs", bc.CurrentBlock.Header.Number)
		os.Exit(1)
		return
	}

	if !bytes.Equal(b.Header.ParentBlockHash, bc.CurrentBlock.Hash) {
		fmt.Println("err parent block hash", common.BytesToHash(b.Header.ParentBlockHash).String(), "vs", common.BytesToHash(bc.CurrentBlock.Hash).String())
		os.Exit(1)
		return
	}

	var modify_account_count int
	modify_account_count = 0
	_, err := trie.New(trie.TrieID(common.BytesToHash(b.Header.StateRoot)), bc.triedb)
	if err != nil {
		_, modify_account_count_ := bc.GetUpdateStateTrie(b.Body)
		modify_account_count = modify_account_count_
	}
	bc.CurrentBlock = b
	bc.Storage.AddBlock(b)

	time_duration := time.Since(time_now)

	resultEntity := &resultWriter.ResultEntity{
		BlockNumber:        b.Header.Number,
		BlockHash:          hex.EncodeToString(b.Hash),
		TxNumber:           uint64(len(b.Body)),
		ActionType:         "BlkAdder",
		ProcessTime:        int64(time_duration.Milliseconds()),
		ModifyAccountCount: modify_account_count,
		ReceiveTime:        time.Now().UnixMilli(),
	}

	bc.ResultWriter.CsvEntityChan <- resultEntity

}

func (bc *BlockChain) GetUpdateStateTrie(txs []*core.Transaction) (common.Hash, int) {

	modified_account_map := make(map[common.Address]bool)
	if len(txs) == 0 {
		return common.BytesToHash(bc.CurrentBlock.Header.StateRoot), 0
	}
	st, err := trie.New(trie.TrieID(common.BytesToHash(bc.CurrentBlock.Header.StateRoot)), bc.triedb)
	if err != nil {
		log.Panic(err)
	}
	cnt := 0

	var addresses []*common.Address

	var modifiedAccountsInBlock = make(map[common.Address]*core.AccountState)

	for i, tx := range txs {
		bc.Statistic.TempLevelDBCounter = 0

		atomic.AddUint64(&bc.ProcessedTxCount, 1)

		addresses = append(addresses, &tx.Sender)
		addresses = append(addresses, &tx.Recipient)

		bc.SignSimulator.SimulateVerify()

		var s_state *core.AccountState

		s_state, exists := modifiedAccountsInBlock[tx.Sender]
		if !exists {
			time_start := time.Now()
			s_state, exists = bc.Snapshot.GetAccountState(tx.Sender)
			bc.Statistic.ThisRoundSnapShotGetNum++
			bc.Statistic.ThisBlockSnapShotGetNum++
			bc.Statistic.TotalSnapShotGetNum++
			if !exists {
				s_state_enc, _ := st.Get([]byte(tx.Sender.Bytes()))
				bc.Statistic.AccountStateGetTimeConsumed = bc.Statistic.AccountStateGetTimeConsumed + time.Since(time_start)
				bc.Statistic.AccountStateGetNum++
				if s_state_enc == nil {
					ib := new(big.Int)
					ib.Add(ib, bc.ChainConfig.InitBalance)
					s_state = &core.AccountState{
						Nonce:   uint64(i),
						Balance: ib,
					}
				} else {
					s_state = core.DecodeAS(s_state_enc)
				}
			} else {
				bc.Statistic.ThisBlockSnapShotHitNum++
				bc.Statistic.ThisRoundSnapShotHitNum++
				bc.Statistic.TotalSnapShotHitNum++
			}
			if exists {
				bc.Snapshot.UpdateSnapshot(tx.Sender, s_state)
			}
		}

		modified_account_map[tx.Sender] = true
		s_state.Deduct(tx.Value)
		cnt++

		bc.Statistic.ThisBlockLevelDBGetNum += bc.Statistic.TempLevelDBCounter
		bc.Statistic.ThisRoundLevelDBGetNum += bc.Statistic.TempLevelDBCounter
		bc.Statistic.TotalLevelDBGetNum += bc.Statistic.TempLevelDBCounter

		modifiedAccountsInBlock[tx.Sender] = s_state

		bc.Statistic.TempLevelDBCounter = 0

		var r_state *core.AccountState

		r_state, exists = modifiedAccountsInBlock[tx.Recipient]
		if !exists {
			r_state, exists = bc.Snapshot.GetAccountState(tx.Recipient)
			bc.Statistic.ThisRoundSnapShotGetNum++
			bc.Statistic.ThisBlockSnapShotGetNum++
			bc.Statistic.TotalSnapShotGetNum++
			if !exists {
				r_state_enc, _ := st.Get(tx.Recipient.Bytes())
				if r_state_enc == nil {
					ib := new(big.Int)
					ib.Add(ib, bc.ChainConfig.InitBalance)
					r_state = &core.AccountState{
						Nonce:   0,
						Balance: ib,
					}
				} else {
					r_state = core.DecodeAS(r_state_enc)
				}
			} else {
				bc.Statistic.ThisRoundSnapShotHitNum++
				bc.Statistic.ThisBlockSnapShotHitNum++
				bc.Statistic.TotalSnapShotHitNum++
			}
			if exists {
				bc.Snapshot.UpdateSnapshot(tx.Recipient, r_state)
			}
		}

		r_state.Deposit(tx.Value)
		modified_account_map[tx.Recipient] = true

		bc.Statistic.ThisBlockLevelDBGetNum += bc.Statistic.TempLevelDBCounter
		bc.Statistic.ThisRoundLevelDBGetNum += bc.Statistic.TempLevelDBCounter
		bc.Statistic.TotalLevelDBGetNum += bc.Statistic.TempLevelDBCounter

		modifiedAccountsInBlock[tx.Recipient] = r_state

		cnt++
	}

	for addr, state := range modifiedAccountsInBlock {
		st.Update(addr.Bytes(), state.Encode())
	}

	if cnt == 0 {
		return common.BytesToHash(bc.CurrentBlock.Header.StateRoot), 0
	}
	rt, ns := st.Commit(false)
	if ns != nil {
		err = bc.triedb.Update(trie.NewWithNodeSet(ns))
		if err != nil {
			log.Panic()
		}
		err = bc.triedb.Commit(rt, false)
		if err != nil {
			log.Panic(err)
		}
	}
	return rt, len(modified_account_map)
}

func NewBlockChain(cc *utils.Config) (*BlockChain, error) {

	statistic := &statistic.ChainStatistic{
		AccountStateGetNum:          0,
		AccountStateGetTimeConsumed: 0,
	}
	lvldbPath := filepath.Join(cc.StoragePath, "levelDB_node_"+strconv.FormatUint(cc.NodeID, 10))
	log.Info("DataBase path: ", lvldbPath)

	db, _ := rawdb.NewLevelDBDatabase(lvldbPath, 512, 32, "accountState", false)

	rwter := resultWriter.NewResultWriter(filepath.Join(cc.OutputDatasetPath, "NodeID_"+strconv.FormatUint(cc.NodeID, 10)+"_"+cc.ExpMode+"_"+".csv"))

	fmt.Println("Generating a new blockchain", db)

	bc := &BlockChain{
		GoroutineNum:       runtime.NumCPU(),
		db:                 db,
		Storage:            storage.NewStorage(db),
		ChainConfig:        cc,
		ToBePackagedBlocks: make(chan *core.Block, 100),
		TxPool:             txPool.NewTxPool(cc.TxPoolSize),
		SignSimulator:      utils.NewRSASimulator(),
		ResultWriter:       rwter,
		Statistic:          statistic,
	}
	if bc.ChainConfig.ExpMode != "origin" {
		initalK, err := tpe.InitGetK()
		if err != nil {
			log.Panic("TPE Worker 初始化失败", err)
		}

		bc.Behavior = behavior.NewHarvesterQueue(cc.QueueSize, initalK, cc.ExpMode)
	}

	bc.Bloom = bloom.NewCustomBloomFilter(cc.BloomCapacity, cc.BloomFalsePositiveRate)
	bc.Snapshot = snapshot.NewSnapshot()

	if bc.ChainConfig.NodeMode == "leader" {
		if bc.ChainConfig.TxGeneratorMode == "random" {
			bc.TxGenerator = txGenerator.NewTxGeneratorFromRandom()
		} else if bc.ChainConfig.TxGeneratorMode == "csv" {
			bc.TxGenerator = txGenerator.NewTxGeneratorFromCSV(bc.ChainConfig.InputDatasetPath)
		}
	}

	bc.trieDBPool = make([]*trie.Database, 1000)

	curHash, err := bc.Storage.GetNewestBlockHash()

	if err != nil {
		fmt.Println("There is no existed blockchain in the database. ")
		if err.Error() == "cannot find the newest block hash" {
			genisisBlock := bc.NewGenisisBlock()
			bc.AddGenisisBlock(genisisBlock)
			fmt.Println("New genisis block")
			return bc, nil
		}
		log.Panic(err)
	}

	fmt.Println("Existing blockchain found")
	currentBlock, err := bc.Storage.GetBlock(curHash)
	if err != nil {
		log.Panic()
	}

	bc.CurrentBlock = currentBlock
	triedb := trie.NewDatabaseWithConfig(db, &trie.Config{
		Cache:     cc.CacheSize,
		Preimages: true,
	},
		bc.ChainConfig,
		bc.Statistic)

	triedb.ChainConfig = bc.ChainConfig
	triedb.Statistic = bc.Statistic

	bc.triedb = triedb
	_, err = trie.New(trie.TrieID(common.BytesToHash(currentBlock.Header.StateRoot)), triedb)
	if err != nil {
		log.Panic(err)
	}

	fmt.Println("The status trie can be built")

	fmt.Println("Generated a new blockchain successfully")
	bc.PollutionAddresses = make([]*common.Address, 0)
	for i := 0; i < cc.PollutionAddressNum; i++ {
		addr := utils.RandomAddress()
		bc.PollutionAddresses = append(bc.PollutionAddresses, &addr)
	}

	return bc, nil
}

func (bc *BlockChain) CloseBlockChain() {
	bc.Storage.DB.Close()
	bc.triedb.CommitPreimages()
}

func (bc *BlockChain) PrintBlockChain() string {
	res := "=====================\n"
	res += fmt.Sprintf("Current Block Number: %d\n", bc.CurrentBlock.Header.Number)
	res += fmt.Sprintf("internal Block Number: %d\n", bc.NowInternalBlockNumber)
	res += fmt.Sprintf("Current Block Body: %d transactions\n", len(bc.CurrentBlock.Body))
	res += fmt.Sprintf("Current Tx Pool Size: %d\n", len(bc.TxPool.TxQueue))
	res += fmt.Sprintf("Current To Be Packaged Blocks Channel Size: %d\n", len(bc.ToBePackagedBlocks))

	res += fmt.Sprintf("Total Processed Transactions: %d\n", bc.ProcessedTxCount)

	res += fmt.Sprintf("Current Block Hash: %s\n", hex.EncodeToString(bc.CurrentBlock.Hash))
	res += fmt.Sprintf("Parent Block Hash: %s\n", hex.EncodeToString(bc.CurrentBlock.Header.ParentBlockHash))
	res += fmt.Sprintf("Current Block State Root: %s\n", hex.EncodeToString(bc.CurrentBlock.Header.StateRoot))

	res += fmt.Sprintf("This Block SnapShot Hit Rate: %.2f%%\n",
		float64(bc.Statistic.ThisBlockSnapShotHitNum)*100.0/float64(bc.Statistic.ThisBlockSnapShotGetNum))

	res += fmt.Sprintf("This Round SnapShot Hit Rate: %.2f%%\n",
		float64(bc.Statistic.ThisRoundSnapShotHitNum)*100.0/float64(bc.Statistic.ThisRoundSnapShotGetNum))

	res += fmt.Sprintf("Total SnapShot Hit Rate: %.2f%%\n",
		float64(bc.Statistic.TotalSnapShotHitNum)*100.0/float64(bc.Statistic.TotalSnapShotGetNum))

	res += fmt.Sprintf("K Value: %.10f\n", bc.Statistic.KValue)

	res += fmt.Sprintf("This Block LevelDB Get Num: %d\n", bc.Statistic.ThisBlockLevelDBGetNum)
	res += fmt.Sprintf("This Round LevelDB Get Num: %d\n", bc.Statistic.ThisRoundLevelDBGetNum)
	res += fmt.Sprintf("Total LevelDB Get Num: %d\n", bc.Statistic.TotalLevelDBGetNum)

	res += "=====================\n"

	return res
}

func (bc *BlockChain) GetAccountState_for_API(address common.Address) (*core.AccountState, bool) {

	idx := rand.Intn(1000)
	triedb := bc.trieDBPool[idx]

	st, err := trie.New(trie.TrieID(common.BytesToHash(bc.CurrentBlock.Header.StateRoot)), triedb)
	if err != nil {
		log.Error("Failed to create trie for getting account state: ", err)
		return nil, false
	}

	encodedState, err := st.Get(address.Bytes())
	if err != nil {
		log.Error("Failed to get account state from trie: ", err)
		return nil, false
	}

	if encodedState == nil {
		return nil, false
	}

	accountState := core.DecodeAS(encodedState)
	return accountState, true
}

func (bc *BlockChain) ExportStatisticToCSV(filename string) {
	log.Info("Starting statistic exporter goroutine...")

	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Error("Failed to open CSV file: %v", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Flush()

	var prevTotalGetNum uint64

	for {

		time.Sleep(1 * time.Second)

		currentTotalGetNum := atomic.LoadUint64(&bc.Statistic.AccountStateGetNum)

		deltaGetNum := currentTotalGetNum - prevTotalGetNum

		record := []string{
			time.Now().Format(time.RFC3339),
			fmt.Sprintf("%d", deltaGetNum),
			strconv.FormatInt(int64(bc.ChainConfig.BlockSize*bc.CurrentBlock.Header.Number*2), 10),
		}
		log.Info("Writing record to CSV: ", record)
		if err := writer.Write(record); err != nil {
			log.Error("Failed to write CSV record: %v", err)
		}

		writer.Flush()

		fmt.Printf("📊 Statistic (Last 1s): Gets: **%d** | Avg Get Time: **%s** | **Exported to %s**\n", deltaGetNum, "N/A", filename)

	}
}

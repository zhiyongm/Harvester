package chain

import (
	"blockcooker/core"
	p2p "blockcooker/network"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

func (bc *BlockChain) StartBlockGenerator(ctx context.Context, p2pPublisher *p2p.Publisher) {
	tx_pool := bc.TxPool
	block_size := bc.ChainConfig.BlockSize
	block_interval := bc.ChainConfig.BlockInterval

	nowBlkNumber := bc.CurrentBlock.Header.Number

	for {
		bc.NowInternalBlockNumber++
		if bc.NowInternalBlockNumber >= bc.ChainConfig.ExpBlockCount {
			log.Info("区块生成器达到实验区块数量，停止生成区块")
			time.Sleep(5 * time.Second)
			bc.CloseBlockChain()
			time.Sleep(5 * time.Second)
			os.Exit(0)
		}
		if bc.NowInternalBlockNumber == 1000+uint64(bc.ChainConfig.QueueSize) {
			for i := 0; i < 1; i++ {
				if !bc.ChainConfig.IsGenerate {
					bc.triedb.AddRandomVisitor()
				}

			}
			bc.Statistic.PollutionWorkerNum = 1
			bc.Statistic.Queue_pollution_rate = 0.1
		}
		if bc.NowInternalBlockNumber == 2000+uint64(bc.ChainConfig.QueueSize) {
			for i := 0; i < 4; i++ {
				if !bc.ChainConfig.IsGenerate {
					bc.triedb.AddRandomVisitor()
				}
			}
			bc.Statistic.PollutionWorkerNum = 5
			bc.Statistic.Queue_pollution_rate = 0.2

		}

		if bc.NowInternalBlockNumber == 3000+uint64(bc.ChainConfig.QueueSize) {
			for i := 0; i < 5; i++ {
				if !bc.ChainConfig.IsGenerate {
					bc.triedb.AddRandomVisitor()
				}
			}
			bc.Statistic.PollutionWorkerNum = 10
			bc.Statistic.Queue_pollution_rate = 0.4

		}
		if bc.NowInternalBlockNumber == 4000+uint64(bc.ChainConfig.QueueSize) {
			for i := 0; i < 10; i++ {
				if !bc.ChainConfig.IsGenerate {
					bc.triedb.AddRandomVisitor()
				}
			}
			bc.Statistic.PollutionWorkerNum = 20
			bc.Statistic.Queue_pollution_rate = 0.6

		}
		if bc.NowInternalBlockNumber == 5000+uint64(bc.ChainConfig.QueueSize) {
			for i := 0; i < 20; i++ {
				if !bc.ChainConfig.IsGenerate {
					bc.triedb.AddRandomVisitor()
				}
			}
			bc.Statistic.PollutionWorkerNum = 40
			bc.Statistic.Queue_pollution_rate = 0.8

		}

		if bc.NowInternalBlockNumber == 6000+uint64(bc.ChainConfig.QueueSize) {
			for i := 0; i < 40; i++ {
				if !bc.ChainConfig.IsGenerate {
					bc.triedb.AddRandomVisitor()
				}
			}
			bc.Statistic.PollutionWorkerNum = 80
			bc.Statistic.Queue_pollution_rate = 1.0

		}

		if bc.NowInternalBlockNumber == 7000+uint64(bc.ChainConfig.QueueSize) {
			for i := 0; i < 80; i++ {
				if !bc.ChainConfig.IsGenerate {
					bc.triedb.AddRandomVisitor()
				}
			}
			bc.Statistic.PollutionWorkerNum = 160
			bc.Statistic.Queue_pollution_rate = 2.0

		}

		var txs []*core.Transaction
		if block_size == 0 {

			txs = tx_pool.PackTxsByNextBlock()
		} else {
			txs = tx_pool.PackTxs(block_size)
		}

		if len(txs) == 0 {
			continue
		}

		nowBlkNumber++

		blk := bc.GenerateBlock(txs, nowBlkNumber)

		log.Println("[从交易池构建区块]", "区块号", blk.Header.Number, "区块哈希", hex.EncodeToString(blk.Hash), "交易数量", len(txs))

		if p2pPublisher != nil && bc.ChainConfig.PeerCount != 0 {
			go func() {
				err := p2pPublisher.PublishMessage(ctx, string(blk.Encode()))
				if err != nil {
					log.Error("消息广播失败", err)
				}
			}()
		}
		bc.AddBlock(blk)
		log.Println("[添加区块到主链]", "区块号", blk.Header.Number, "区块哈希", hex.EncodeToString(blk.Hash), "交易数量", len(blk.Body))
		fmt.Println(bc.PrintBlockChain())

		bc.Statistic.ThisBlockSnapShotHitNum = 0
		bc.Statistic.ThisBlockSnapShotGetNum = 0
		bc.Statistic.ThisBlockLevelDBGetNum = 0

		time.Sleep(time.Duration(block_interval) * time.Millisecond)

	}

}

func (bc *BlockChain) StartBlockAdder(ctx context.Context, p2pPublisher *p2p.Publisher) {
	log.Println("启动区块添加器")
	go func() {
		for {
			blk := <-bc.ToBePackagedBlocks

			if p2pPublisher != nil {
				go func() {
					err := p2pPublisher.PublishMessage(ctx, string(blk.Encode()))
					if err != nil {
						log.Error("消息广播失败", err)
					}
				}()
			}
			bc.AddBlock(blk)
			log.Println("[添加区块到主链]", "区块号", blk.Header.Number, "区块哈希", hex.EncodeToString(blk.Hash), "交易数量", len(blk.Body))
			fmt.Println(bc.PrintBlockChain())

		}
	}()
}

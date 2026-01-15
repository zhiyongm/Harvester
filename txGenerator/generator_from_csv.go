package txGenerator

import (
	"blockcooker/core"
	"blockcooker/utils"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gocarina/gocsv"
)

type TxGeneratorFromCSV struct {
	csvPath       string
	csvEntityChan chan utils.CSVEntity
	txsChan       chan *core.Transaction
}

func (t *TxGeneratorFromCSV) GetTxs(txsCount int) []*core.Transaction {
	var txs []*core.Transaction
	for i := 0; i < txsCount; i++ {
		tx, ok := <-t.txsChan
		if !ok {
			break
		}
		txs = append(txs, tx)
	}
	return txs
}

func (t *TxGeneratorFromCSV) GetTx() *core.Transaction {

	tx, ok := <-t.txsChan
	if !ok {
		return nil
	}
	return tx
}

func NewTxGeneratorFromCSV(csvPath string) *TxGeneratorFromCSV {
	csvEntityChan := make(chan utils.CSVEntity, 100000)
	txsChan := make(chan *core.Transaction, 100000)

	tg := &TxGeneratorFromCSV{
		csvPath:       csvPath,
		csvEntityChan: csvEntityChan,
		txsChan:       txsChan,
	}

	go func() {

		file, err := os.Open(csvPath)
		if err != nil {
			log.Fatalf("无法打开文件: %v", err)
		}
		defer file.Close()

		if err := gocsv.UnmarshalToChan(file, tg.csvEntityChan); err != nil {
			log.Printf("解析CSV时发生错误: %v", err)
		}
	}()

	go func() {
		defer close(tg.txsChan)
		fixedTime := time.Date(2025, time.January, 1, 12, 0, 0, 0, time.UTC)
		for csvEntity := range csvEntityChan {
			b := new(big.Int)
			b.SetString(csvEntity.Value, 10)
			tx := core.NewTransaction(common.HexToAddress(csvEntity.From),
				common.HexToAddress(csvEntity.To),
				0, b,
				common.HexToHash(csvEntity.Hash), fixedTime)

			if csvEntity.BlockNumber != "" {
				tx.BlockNumber = utils.BytesToUInt64([]byte(csvEntity.BlockNumber))
			}
			txsChan <- tx
		}
	}()
	return tg
}

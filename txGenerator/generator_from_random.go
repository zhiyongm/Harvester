package txGenerator

import (
	"blockcooker/core"
	"blockcooker/utils"
	"math/big"
)

type TxGeneratorFromRandom struct {
	txsChan chan core.Transaction
}

func (t *TxGeneratorFromRandom) GetTxs(txsCount int) []*core.Transaction {
	var txs []*core.Transaction
	for i := 0; i < txsCount; i++ {
		tx, ok := <-t.txsChan
		if !ok {
			break
		}
		txs = append(txs, &tx)
	}
	return txs
}

func (t *TxGeneratorFromRandom) GetTx() *core.Transaction {

	tx, ok := <-t.txsChan
	if !ok {
		return nil
	}
	return &tx
}

func NewTxGeneratorFromRandom() *TxGeneratorFromRandom {

	txChan := make(chan core.Transaction, 100000)
	tg := &TxGeneratorFromRandom{
		txsChan: txChan,
	}

	go func() {
		for {
			tx := core.Transaction{
				Sender:    utils.RandomAddress(),
				Recipient: utils.RandomAddress(),
				Value:     big.NewInt(int64(utils.RandomBytes(8)[0])),
			}
			txChan <- tx
		}
	}()

	return tg
}

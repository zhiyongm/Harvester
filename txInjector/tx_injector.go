package txInjector

import (
	"blockcooker/chain"
	"blockcooker/txGenerator"
	"blockcooker/txPool"
	"blockcooker/utils"
	"log"
)

type TxInjector struct {
	ChainConfig *utils.Config
	txg         txGenerator.TxGeneratorInterface
	txpool      *txPool.TxPool
	isOKChan    chan bool
}

func (txij *TxInjector) StartTxInjector(chainConfig *utils.Config) {

	for {
		tx := txij.txg.GetTx()
		if tx == nil {
			log.Println("注入器", "没有更多交易可注入，退出注入器")
			txij.isOKChan <- true
			break
		}
		txij.txpool.AddTx2Pool(tx)
	}

}

func NewTxInjector(bc *chain.BlockChain) *TxInjector {
	return &TxInjector{
		ChainConfig: bc.ChainConfig,
		txg:         bc.TxGenerator,
		txpool:      bc.TxPool,
		isOKChan:    make(chan bool, 1),
	}
}

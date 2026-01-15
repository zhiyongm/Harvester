package txGenerator

import "blockcooker/core"

type TxGeneratorInterface interface {
	GetTxs(txsCount int) []*core.Transaction
	GetTx() *core.Transaction
}

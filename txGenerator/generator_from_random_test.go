package txGenerator

import (
	"fmt"
	"testing"
)

func TestRandomGenerateTxs(t *testing.T) {
	var tg TxGeneratorInterface
	tg = NewTxGeneratorFromRandom()
	for {
		txs := tg.GetTxs(100)
		for _, tx := range txs {
			fmt.Printf("Sender: %s, Recipient: %s, Value: %d\n", tx.Sender.Hex(), tx.Recipient.Hex(), tx.Value.Int64())
		}

	}
}

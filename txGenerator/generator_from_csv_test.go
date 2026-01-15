package txGenerator

import (
	"fmt"
	"testing"
)

func TestReadCSV(t *testing.T) {
	var tg TxGeneratorInterface
	tg = NewTxGeneratorFromCSV("/Volumes/data/22000000to22249999_InternalTransaction/22000000to22249999_InternalTransaction_Tidy_10w.csv")
	tx_count := 0
	for {
		txs := tg.GetTxs(10000)
		tx_count += len(txs)
		fmt.Printf("Total transactions so far: %d\n", tx_count)
		if len(txs) == 0 {
			fmt.Println("No transactions found")
			fmt.Println(tx_count)
			break
		}

	}
}

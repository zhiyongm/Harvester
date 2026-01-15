package resultWriter

import "testing"

func TestResultWriter(t *testing.T) {
	rw := NewResultWriter("test.csv")
	if rw == nil {
		t.Fatal("Expected non-nil ResultWriter")
	}

	if rw.CsvEntityChan == nil {
		t.Fatal("Expected non-nil CsvEntityChan")
	}

	testEntity := &ResultEntity{BlockNumber: 123456, TxNumber: 7890}
	rw.CsvEntityChan <- testEntity

	testEntity = &ResultEntity{BlockNumber: 226688, TxNumber: 32443}
	rw.CsvEntityChan <- testEntity

	close(rw.CsvEntityChan)

	<-rw.isComplete

	t.Log("TestResultWriter completed successfully")
}

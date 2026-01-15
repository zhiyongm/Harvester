package resultWriter

import (
	"encoding/csv"
	"fmt"
	"os"
	"time"

	"github.com/gocarina/gocsv"
)

type ResultWriter struct {
	csvPath       string
	CsvEntityChan chan interface{}
	isComplete    chan bool
}

func NewResultWriter(csvPath string) *ResultWriter {
	csvEntityChan := make(chan interface{}, 100000)
	isComplete := make(chan bool, 1)
	file, err := os.Create(csvPath)
	if err != nil {
		fmt.Errorf("无法创建文件 %s: %w", csvPath, err)
	}

	csvWriter := csv.NewWriter(file)
	safeWriter := gocsv.NewSafeCSVWriter(csvWriter)

	rw := &ResultWriter{
		csvPath:       csvPath,
		CsvEntityChan: csvEntityChan,
		isComplete:    isComplete,
	}

	go func() {
		err := gocsv.MarshalChan(csvEntityChan, safeWriter)
		if err != nil && err != gocsv.ErrChannelIsClosed {
			fmt.Printf("写入CSV时发生错误: %v\n", err)
		}
		fmt.Println("✅ CSV数据写入完成。")
		isComplete <- true
	}()

	go func() {
		for {
			safeWriter.Flush()
			time.Sleep(1 * time.Second)
		}
	}()

	return rw
}

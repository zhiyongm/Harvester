package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	baseURL = "http://192.168.2.141:56888/account?address="
)

type AccountAddresses struct {
	From string
	To   string
}

func readCSVFile(filePath string, dataCh chan<- AccountAddresses) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("无法打开文件: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("无法读取头部: %w", err)
	}

	fromIndex := -1
	toIndex := -1
	for i, colName := range header {
		lowerColName := strings.ToLower(strings.TrimSpace(colName))
		if lowerColName == "from" {
			fromIndex = i
		} else if lowerColName == "to" {
			toIndex = i
		}
	}

	if fromIndex == -1 || toIndex == -1 {
		return fmt.Errorf("CSV文件中必须包含'from'和'to'列")
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("警告: 读取CSV行时出错: %v. 跳过此行.\n", err)
			continue
		}

		if fromIndex < len(record) && toIndex < len(record) {
			dataCh <- AccountAddresses{
				From: strings.TrimSpace(record[fromIndex]),
				To:   strings.TrimSpace(record[toIndex]),
			}
		} else {
			fmt.Printf("警告: CSV行数据不足，跳过此行: %v\n", record)
		}
	}

	return nil
}

func worker(id int, dataCh <-chan AccountAddresses, wg *sync.WaitGroup) {
	defer wg.Done()

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for addresses := range dataCh {
		requestURLFrom := baseURL + addresses.From
		fmt.Printf("Worker %d: 正在请求 From 地址: %s\n", id, requestURLFrom)
		makeHTTPRequest(client, requestURLFrom)

		requestURLTo := baseURL + addresses.To
		fmt.Printf("Worker %d: 正在请求 To 地址: %s\n", id, requestURLTo)
		makeHTTPRequest(client, requestURLTo)
	}
	fmt.Printf("Worker %d: 完成所有任务并退出。\n", id)
}

func makeHTTPRequest(client *http.Client, url string) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		fmt.Printf("错误: 创建请求 %s 失败: %v\n", url, err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("错误: 请求 %s 失败: %v\n", url, err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("请求 %s 状态码: %d\n", url, resp.StatusCode)

}

func main() {
	csvFilePath := "/Volumes/data/22000000to22249999_InternalTransaction/22000000to22249999_InternalTransaction.csv"
	numWorkers := 1000

	dataChannel := make(chan AccountAddresses, numWorkers*2)
	var wg sync.WaitGroup

	fmt.Printf("启动程序: 从 %s 读取数据，启动 %d 个 Worker goroutine...\n", csvFilePath, numWorkers)

	go func() {
		defer close(dataChannel)
		if err := readCSVFile(csvFilePath, dataChannel); err != nil {
			fmt.Printf("错误: 读取CSV文件失败: %v\n", err)
			os.Exit(1)
		}
	}()

	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go worker(i, dataChannel, &wg)
	}

	wg.Wait()

	fmt.Println("所有任务完成，程序退出。")
}

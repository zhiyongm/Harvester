package behavior_test

import (
	"blockcooker/behavior"
	"reflect"
	"testing"
)

func TestNewFIFOQueueCapacity(t *testing.T) {
	const testCapacity = 5
	q := behavior.NewFIFOQueue(testCapacity)

	if q == nil {
		t.Fatal("NewFIFOQueue 返回了 nil，预期返回一个对象")
	}
	if q.Length() != 0 {
		t.Errorf("新建的队列长度应为 0，实际为 %d", q.Length())
	}
	if q.MaxSize != testCapacity {
		t.Errorf("队列的 MaxSize 错误。期望: %d, 实际: %d", testCapacity, q.MaxSize)
	}
}

func TestFIFOQueueOperations(t *testing.T) {
	q := behavior.NewFIFOQueue(10)

	q.Enqueue(behavior.Address("Addr1"))
	q.Enqueue(behavior.Address("Addr2"))
	q.Enqueue(behavior.Address("Addr3"))

	if q.Length() != 3 {
		t.Errorf("Enqueue 3 个元素后，队列长度应为 3，实际为 %d", q.Length())
	}

	addr, ok := q.Dequeue()
	if !ok || addr != behavior.Address("Addr1") {
		t.Errorf("第一次 Dequeue 失败或不符合 FIFO。期望: Addr1, 实际: %s", addr)
	}

	addr, ok = q.Dequeue()
	if !ok || addr != behavior.Address("Addr2") {
		t.Errorf("第二次 Dequeue 失败或不符合 FIFO。期望: Addr2, 实际: %s", addr)
	}

	q.Dequeue()
	addr, ok = q.Dequeue()
	if ok {
		t.Errorf("从空队列 Dequeue 应该返回 ok=false，实际返回 ok=true")
	}
}

func TestFIFOQueueCapacityLimit(t *testing.T) {
	const maxCap = 3
	q := behavior.NewFIFOQueue(maxCap)

	q.Enqueue(behavior.Address("A1"))
	q.Enqueue(behavior.Address("A2"))
	q.Enqueue(behavior.Address("A3"))

	if q.Length() != maxCap {
		t.Fatalf("队列长度应为 %d，实际为 %d", maxCap, q.Length())
	}

	q.Enqueue(behavior.Address("A4"))

	if q.Length() != maxCap {
		t.Errorf("触发覆盖后，队列长度仍应为 %d，实际为 %d", maxCap, q.Length())
	}

	expectedQueue := []behavior.Address{"A2", "A3", "A4"}
	actualQueue := q.GetAllAddresses()

	if !reflect.DeepEqual(actualQueue, expectedQueue) {
		t.Errorf("队列覆盖机制错误。\n期望: %v\n实际: %v", expectedQueue, actualQueue)
	}

	q.Enqueue(behavior.Address("A5"))

	expectedQueue2 := []behavior.Address{"A3", "A4", "A5"}
	actualQueue2 := q.GetAllAddresses()

	if !reflect.DeepEqual(actualQueue2, expectedQueue2) {
		t.Errorf("二次队列覆盖机制错误。\n期望: %v\n实际: %v", expectedQueue2, actualQueue2)
	}
}

func TestCountAndSortAddresses(t *testing.T) {
	analyser := behavior.NewBehaviorAnalyser(10)

	analyser.Queue.Enqueue(behavior.Address("A"))
	analyser.Queue.Enqueue(behavior.Address("B"))
	analyser.Queue.Enqueue(behavior.Address("A"))
	analyser.Queue.Enqueue(behavior.Address("D"))
	analyser.Queue.Enqueue(behavior.Address("C"))
	analyser.Queue.Enqueue(behavior.Address("B"))
	analyser.Queue.Enqueue(behavior.Address("A"))
	analyser.Queue.Enqueue(behavior.Address("D"))
	analyser.Queue.Enqueue(behavior.Address("D"))

	results := analyser.CountAndSortAddresses()

	expectedLength := 4
	if len(results) != expectedLength {
		t.Fatalf("结果切片长度错误。期望: %d，实际: %d", expectedLength, len(results))
	}

	lastCount := 100
	for i, res := range results {
		if res.Score > lastCount {
			t.Errorf("排序错误：第 %d 个元素的次数 %d 大于前一个元素的次数 %d",
				i+1, res.Score, lastCount)
		}
		lastCount = res.Score
	}

	resultMap := make(map[behavior.Address]int)
	for _, res := range results {
		resultMap[res.Address] = res.Score
	}

	expectedCounts := map[behavior.Address]int{
		"A": 3,
		"B": 2,
		"C": 1,
		"D": 3,
	}

	if !reflect.DeepEqual(resultMap, expectedCounts) {
		t.Errorf("地址计数不正确。\n期望: %v\n实际: %v", expectedCounts, resultMap)
	}
}

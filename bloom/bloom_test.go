package bloom

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestBloomFilterBasic(t *testing.T) {
	const (
		capacity          = 100
		falsePositiveRate = 0.01
	)

	bf := NewCustomBloomFilter(capacity, falsePositiveRate)
	if bf == nil {
		t.Fatal("NewCustomBloomFilter 返回了 nil")
	}

	addr1 := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	addr2 := common.HexToAddress("0xfedcba9876543210fedcba9876543210fedcba98")

	addr3 := common.HexToAddress("0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")

	t.Logf("布隆过滤器创建成功，容量: %d, 误报率: %.2f%%", capacity, falsePositiveRate*100)
	t.Logf("内部位数组大小 (m): %d, 哈希函数数量 (k): %d", bf.filter.Cap(), bf.filter.K())

	bf.AddElement(addr1)
	bf.AddElement(addr2)
	t.Log("添加了两个元素 (addr1, addr2)")

	if !bf.CheckExistence(addr1) {
		t.Errorf("CheckExistence 失败: 期望 addr1 存在 (true), 实际为 false")
	}
	if !bf.CheckExistence(addr2) {
		t.Errorf("CheckExistence 失败: 期望 addr2 存在 (true), 实际为 false")
	}
	t.Log("正向检查通过: 已添加的元素被正确识别")

	if bf.CheckExistence(addr3) {
		t.Logf("警告: 发现误报 (False Positive)，addr3 不应存在但 CheckExistence 返回了 true。这是布隆过滤器的特性。")
	} else {
		t.Log("反向检查通过: 未添加的元素 (addr3) 被正确识别为不存在")
	}

	data, err := bf.Encode()
	if err != nil {
		t.Fatalf("编码失败: %v", err)
	}
	t.Logf("过滤器编码成功，字节大小: %d", len(data))
	if len(data) == 0 {
		t.Fatal("编码结果的字节切片长度为 0")
	}

	bf2, err := DecodeNew(data)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if bf2 == nil || bf2.filter == nil {
		t.Fatal("DecodeNew 返回的过滤器或其内部结构为 nil")
	}
	t.Log("过滤器解码成功")

	if bf.filter.Cap() != bf2.filter.Cap() || bf.filter.K() != bf2.filter.K() {
		t.Errorf("解码后的过滤器参数不匹配。原 Cap/K: %d/%d, 新 Cap/K: %d/%d",
			bf.filter.Cap(), bf.filter.K(), bf2.filter.Cap(), bf2.filter.K())
	}

	if !bf2.CheckExistence(addr1) {
		t.Errorf("解码后的过滤器 CheckExistence 失败: 期望 addr1 存在 (true), 实际为 false")
	}

	if bf2.CheckExistence(addr3) && !bf.CheckExistence(addr3) {
		t.Logf("警告: 解码后的过滤器对 addr3 产生了误报。")
	} else if !bf2.CheckExistence(addr3) {
		t.Log("解码后的过滤器反向检查通过")
	}

}

package service

import "testing"

// 端口必须在 40000-65000 高位区间内，且不能呈「每条 +1」的规律
func TestRandomBatchPort(t *testing.T) {
	seen := make(map[int]bool)
	prev := -1
	sequentialRun := 0

	for i := 0; i < 200; i++ {
		p := randomBatchPort()
		if p < minAutoPort || p > maxAutoPort {
			t.Fatalf("端口 %d 超出 %d-%d 区间", p, minAutoPort, maxAutoPort)
		}
		if prev >= 0 && p == prev+1 {
			sequentialRun++
		}
		prev = p
		seen[p] = true
	}

	if len(seen) < 150 {
		t.Errorf("200 次取样只有 %d 个不同端口，分布过于集中", len(seen))
	}
	if sequentialRun > 5 {
		t.Errorf("出现 %d 次连续 +1，看起来像有规律的递增", sequentialRun)
	}
}

// 显式指定 startPort 时仍应顺序分配（尊重用户选择）
func TestPortRangeConstants(t *testing.T) {
	if minAutoPort != 40000 || maxAutoPort != 65000 {
		t.Fatalf("自动端口区间应为 40000-65000，实际 %d-%d", minAutoPort, maxAutoPort)
	}
	if minUsablePort >= minAutoPort {
		t.Errorf("minUsablePort(%d) 应小于自动区间起点(%d)", minUsablePort, minAutoPort)
	}
}

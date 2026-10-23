package service

import (
	"testing"
	"time"
)

func TestRandomAliveDomainReturnsValidShape(t *testing.T) {
	s := &RealityDomainService{}
	// 用一个很短的超时，保证测试不会卡住
	d := s.RandomAliveDomain(1500 * time.Millisecond)
	if d == nil {
		t.Fatal("返回值不应为 nil")
	}
	if d.Domain == "" || d.SNI == "" {
		t.Fatalf("域名/SNI 不应为空: %+v", d)
	}
	if d.Target != d.Domain+":443" {
		t.Fatalf("target 应为 domain:443，实际 %q", d.Target)
	}
	if d.SNI != d.Domain {
		t.Fatalf("sni 应等于 domain，实际 %q != %q", d.SNI, d.Domain)
	}
}

func TestRealityDomainPoolNoDuplicate(t *testing.T) {
	seen := map[string]struct{}{}
	for _, d := range realityDomainPool {
		if _, ok := seen[d]; ok {
			t.Fatalf("候选池存在重复域名: %s", d)
		}
		seen[d] = struct{}{}
	}
}

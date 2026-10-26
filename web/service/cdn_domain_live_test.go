package service

import (
	"os"
	"testing"
)

// TestFindCDNDomainLive 需要真实 DNS，默认跳过；
// 调试时用 X_PANEL_LIVE_DNS=1 go test -run TestFindCDNDomainLive 运行。
func TestFindCDNDomainLive(t *testing.T) {
	if os.Getenv("X_PANEL_LIVE_DNS") != "1" {
		t.Skip("需要真实 DNS，设置 X_PANEL_LIVE_DNS=1 后运行")
	}
	cases := []string{"us1.bbua.pp.ua"}
	for _, host := range cases {
		got := FindCDNDomain(host)
		t.Logf("%s -> %q", host, got)
		if got == "" {
			t.Errorf("%s 应该能嗅探到同主域下的 CDN 域名", host)
		}
	}
	if baseDomain("us1.bbua.pp.ua") != "bbua.pp.ua" {
		t.Errorf("baseDomain 错误: %s", baseDomain("us1.bbua.pp.ua"))
	}
	if baseDomain("bbua.pp.ua") != "pp.ua" {
		t.Errorf("baseDomain 对二级域名处理错误: %s", baseDomain("bbua.pp.ua"))
	}
}

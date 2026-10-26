package service

import (
	"net"
	"testing"
)

func TestIsCloudflareIP(t *testing.T) {
	cases := map[string]bool{
		"104.16.132.229":  true,  // Cloudflare
		"172.64.155.209":  true,  // Cloudflare
		"162.158.10.10":   true,  // Cloudflare
		"182.255.80.150":  false, // 普通 VPS
		"1.1.1.1":         false, // 虽然属于 CF 的 DNS，但不在代理回源网段里
		"2606:4700::6810": true,  // Cloudflare IPv6
	}
	for ipStr, want := range cases {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			t.Fatalf("bad ip %s", ipStr)
		}
		if got := isCloudflareIP(ip); got != want {
			t.Errorf("isCloudflareIP(%s) = %v, want %v", ipStr, got, want)
		}
	}
}

func TestIsCDNPort(t *testing.T) {
	for _, p := range CDNPorts() {
		if !IsCDNPort(p) {
			t.Errorf("port %d 应该被判定为 CDN 端口", p)
		}
	}
	if IsCDNPort(20000) {
		t.Error("20000 不是 Cloudflare 支持的回源端口")
	}
}

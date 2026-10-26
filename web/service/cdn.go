package service

import (
	"net"
	"sync"
	"time"
)

// 中文注释: 【开源增强】CDN 自动识别
//
// 思路：一键配置生成 TLS 系节点时，如果面板域名已经在 Cloudflare 后面（橙色云），
// 就自动：
//  1. 把入站端口挑到 Cloudflare 支持的回源端口（443/2053/2083/2087/2096/8443）
//  2. 追加一条指向 CDN 域名的分享链接
// 用户不需要理解 CDN，也不需要额外输入任何参数。

// cfCIDRs Cloudflare 官方公布的出口网段
var cfCIDRs = []string{
	"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22",
	"141.101.64.0/18", "108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20",
	"197.234.240.0/22", "198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13",
	"104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22", "2400:cb00::/32",
	"2606:4700::/32", "2803:f800::/32", "2405:b500::/32", "2405:8100::/32",
	"2a06:98c0::/29", "2c0f:f248::/32",
}

// CDN 支持的回源端口（客户端连 CDN 用的端口，Cloudflare 会转发到源站同一端口）
var cdnPorts = []int{2053, 2083, 2087, 2096, 8443, 443}

var (
	cfNetsOnce sync.Once
	cfNets     []*net.IPNet

	cdnCacheMu sync.Mutex
	cdnCache   = map[string]cdnCacheEntry{}
)

type cdnCacheEntry struct {
	behind bool
	at     time.Time
}

func loadCFNets() {
	cfNetsOnce.Do(func() {
		for _, c := range cfCIDRs {
			if _, n, err := net.ParseCIDR(c); err == nil {
				cfNets = append(cfNets, n)
			}
		}
	})
}

// IsBehindCloudflare 判断某个域名当前是否解析到 Cloudflare 边缘节点。
// 结果缓存 10 分钟，避免一键配置时反复查 DNS。
func IsBehindCloudflare(domain string) bool {
	if domain == "" {
		return false
	}
	cdnCacheMu.Lock()
	if e, ok := cdnCache[domain]; ok && time.Since(e.at) < 10*time.Minute {
		cdnCacheMu.Unlock()
		return e.behind
	}
	cdnCacheMu.Unlock()

	behind := false
	if ips, err := net.LookupIP(domain); err == nil {
		for _, ip := range ips {
			if isCloudflareIP(ip) {
				behind = true
				break
			}
		}
	}

	cdnCacheMu.Lock()
	cdnCache[domain] = cdnCacheEntry{behind: behind, at: time.Now()}
	cdnCacheMu.Unlock()
	return behind
}

// CDNPorts 返回 Cloudflare 支持的回源端口列表（按优先级）
func CDNPorts() []int {
	out := make([]int, len(cdnPorts))
	copy(out, cdnPorts)
	return out
}

// IsCDNPort 判断端口是否属于 CDN 支持的回源端口
func IsCDNPort(port int) bool {
	for _, p := range cdnPorts {
		if p == port {
			return true
		}
	}
	return false
}

// isCloudflareIP 判断 IP 是否落在 Cloudflare 的网段内
func isCloudflareIP(ip net.IP) bool {
	loadCFNets()
	for _, n := range cfNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

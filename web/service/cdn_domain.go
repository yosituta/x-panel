package service

import (
	"strings"
)

// 中文注释: 【开源增强】CDN 域名自动嗅探
//
// 多台服务器的推荐命名（面板灰云、节点橙云）：
//
//	panel1.example.com  +  cdn1.example.com
//	panel2.example.com  +  cdn2.example.com
//
// 嗅探顺序：
//  1. 传入域名本身走 Cloudflare → 直接用
//  2. 按「前缀数字」推导：panel2 → cdn2（避免多台服务器串到同一台）
//  3. 兜底：cdn / cdn1 / cdn2 / cdn3 / proxy / cf / ws
//
// 都找不到时返回空字符串，调用方按普通直连处理。

// cdnPrefixes 兜底的 CDN 子域前缀（按优先顺序）
var cdnPrefixes = []string{"cdn", "cdn1", "cdn2", "cdn3", "proxy", "cf", "ws"}

// baseDomain 去掉最左边一级标签：us1.bbua.pp.ua -> bbua.pp.ua
func baseDomain(host string) string {
	host = strings.TrimSpace(strings.TrimSuffix(host, "."))
	if i := strings.Index(host, "."); i >= 0 && i < len(host)-1 {
		return host[i+1:]
	}
	return ""
}

// trailingDigits 取字符串结尾的连续数字：panel12 -> 12，us1 -> 1
func trailingDigits(s string) string {
	i := len(s)
	for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
		i--
	}
	return s[i:]
}

// FindCDNDomain 返回可用的 CDN 域名，找不到返回空字符串
func FindCDNDomain(host string) string {
	// 面板设置里显式指定的 CDN 域名优先
	var settingService SettingService
	if configured, err := settingService.GetCdnDomain(); err == nil {
		configured = strings.TrimSpace(configured)
		if configured != "" {
			return configured
		}
	}

	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if IsBehindCloudflare(host) {
		return host
	}

	base := baseDomain(host)
	if base == "" || strings.Contains(base, ":") {
		return ""
	}

	first := strings.SplitN(strings.TrimSuffix(host, "."), ".", 2)[0]
	var candidates []string
	seen := map[string]bool{}

	add := func(d string) {
		if d != "" && !seen[d] {
			seen[d] = true
			candidates = append(candidates, d)
		}
	}

	// 1) 同名序号：panel2 / vps2 / hk2 → cdn2
	if n := trailingDigits(first); n != "" {
		add("cdn" + n + "." + base)
	}
	// 2) 兜底前缀
	for _, p := range cdnPrefixes {
		add(p + "." + base)
	}

	for _, c := range candidates {
		if IsBehindCloudflare(c) {
			return c
		}
	}
	return ""
}

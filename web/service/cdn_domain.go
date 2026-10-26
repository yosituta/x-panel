package service

import (
	"strings"
)

// 中文注释: 【开源增强】CDN 域名自动嗅探
//
// 用户通常把面板域名（灰云）和 CDN 域名（橙云）放在同一个主域下，
// 例如 us1.bbua.pp.ua + cdn.bbua.pp.ua。
// 一键配置时如果面板域名本身没走 CDN，就自动按常见命名找一圈，
// 找到就直接用，用户不需要在「域名」里手填。

// cdnPrefixes 常见 CDN 子域前缀（按优先顺序）
var cdnPrefixes = []string{"cdn", "cdn1", "cdn2", "cdn3", "proxy", "cf", "ws"}

// baseDomain 去掉最左边一级标签：us1.bbua.pp.ua -> bbua.pp.ua
func baseDomain(host string) string {
	host = strings.TrimSpace(strings.TrimSuffix(host, "."))
	if i := strings.Index(host, "."); i >= 0 && i < len(host)-1 {
		return host[i+1:]
	}
	return ""
}

// FindCDNDomain 返回可用的 CDN 域名：
//  1. 传入的域名本身走 Cloudflare → 直接用
//  2. 否则按 cdn / cdn1 / cdn2 ... 依次探测同主域下的兄弟域名
//
// 都找不到时返回空字符串，调用方按普通直连处理。
func FindCDNDomain(host string) string {
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
	for _, p := range cdnPrefixes {
		candidate := p + "." + base
		if IsBehindCloudflare(candidate) {
			return candidate
		}
	}
	return ""
}

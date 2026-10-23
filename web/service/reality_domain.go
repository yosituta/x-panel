package service

import (
	"crypto/tls"
	"math/rand/v2"
	"net"
	"time"
)

// 中文注释: 【开源增强】Reality “偷用域名”随机挑选器。
//
// Reality 要求目标站点满足：TLS 1.3 可用、HTTP/2、且从本服务器可直连 443。
// 这里对内置候选池做并发探测（仅做 TLS 1.3 握手，不校验证书），
// 从存活的域名里随机返回一个，避免把用户坑在“域名已挂”的配置上。

// RealityDomain 返回给前端的结果
type RealityDomain struct {
	Domain string `json:"domain"` // 域名
	Target string `json:"target"` // domain:443，填到 reality.target
	SNI    string `json:"sni"`    // 填到 reality.serverNames
}

// 候选域名池：Reality 的“偷用域名”。
//
// 选型要求（踩过坑，别乱加）：
//  1. 支持 TLS 1.3 + HTTP/2，且 443 直连可达（本服务会实际探测）；
//  2. **优先非 CDN 站点**。Cloudflare / Akamai 前置的域名（cloudflare.com、
//     zoom.us、bing.com、skype.com、amazon.com 等）会按客户端指纹协商
//     混合密钥交换（X25519MLKEM768），一旦触发 HelloRetryRequest 或协商到
//     iOS 客户端不支持的组，Reality 握手就会在客户端侧失败——
//     表现为“App 延迟测试能过、但一个字节都传不出去”。
//  3. 站点要长期稳定、不频繁改证书/跳转。
var realityDomainPool = []string{
	"www.microsoft.com",
	"www.apple.com",
	"www.icloud.com",
	"www.tesla.com",
	"www.nvidia.com",
	"www.intel.com",
	"www.samsung.com",
	"www.oracle.com",
	"www.ibm.com",
	"www.docker.com",
	"www.python.org",
	"www.mozilla.org",
	"www.adobe.com",
	"www.salesforce.com",
}

type RealityDomainService struct{}

// probeTLS13 探测域名 443 端口是否可完成 TLS1.3 握手
func (s *RealityDomainService) probeTLS13(domain string, timeout time.Duration) bool {
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(domain, "443"), &tls.Config{
		ServerName:         domain,
		InsecureSkipVerify: true, // 只判断可达性与协议版本，不需要校验证书
		MinVersion:         tls.VersionTLS13,
	})
	if err != nil {
		return false
	}
	defer conn.Close()
	return conn.ConnectionState().Version == tls.VersionTLS13
}

// RandomAliveDomain 并发探测候选池，返回一个可用的域名。
// 若全部不可用（例如本机出网受限），退化为随机返回一个候选域名，便于离线/内网场景仍能填表。
func (s *RealityDomainService) RandomAliveDomain(timeout time.Duration) *RealityDomain {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	pool := make([]string, len(realityDomainPool))
	copy(pool, realityDomainPool)
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	// 限制并发探测数量，避免一次性打出过多连接
	const maxProbe = 8
	if len(pool) > maxProbe {
		pool = pool[:maxProbe]
	}

	type result struct{ domain string }
	ch := make(chan result, len(pool))
	for _, domain := range pool {
		go func(d string) {
			if s.probeTLS13(d, timeout) {
				ch <- result{domain: d}
				return
			}
			ch <- result{}
		}(domain)
	}

	deadline := time.After(timeout + 500*time.Millisecond)
	for i := 0; i < len(pool); i++ {
		select {
		case r := <-ch:
			if r.domain != "" {
				return &RealityDomain{
					Domain: r.domain,
					Target: net.JoinHostPort(r.domain, "443"),
					SNI:    r.domain,
				}
			}
		case <-deadline:
			i = len(pool) // 超时，跳出
		}
	}

	// 全部探测失败时的兜底
	fallback := realityDomainPool[rand.IntN(len(realityDomainPool))]
	return &RealityDomain{
		Domain: fallback,
		Target: net.JoinHostPort(fallback, "443"),
		SNI:    fallback,
	}
}

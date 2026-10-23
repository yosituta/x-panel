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

// 候选域名池：均为长期稳定、支持 TLS1.3 + H2 的大厂站点
var realityDomainPool = []string{
	"www.microsoft.com",
	"www.apple.com",
	"www.cloudflare.com",
	"www.amazon.com",
	"www.tesla.com",
	"www.nvidia.com",
	"www.samsung.com",
	"www.adobe.com",
	"www.intel.com",
	"www.oracle.com",
	"www.ibm.com",
	"www.salesforce.com",
	"www.zoom.us",
	"www.bing.com",
	"www.skype.com",
	"dl.google.com",
	"www.python.org",
	"www.golang.org",
	"www.docker.com",
	"www.mozilla.org",
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

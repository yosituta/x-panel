package service

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"
)

// 证书显示用的域名：CN 是 "CloudFlare Origin Certificate" 时不能直接当域名显示
func TestCertDisplayDomain(t *testing.T) {
	cases := []struct {
		name string
		cert *x509.Certificate
		want string
	}{
		{
			name: "普通证书用 CN",
			cert: &x509.Certificate{
				Subject:  pkix.Name{CommonName: "hk.bbua.pp.ua"},
				DNSNames: []string{"hk.bbua.pp.ua"},
			},
			want: "hk.bbua.pp.ua",
		},
		{
			name: "CF 源站证书：CN 不是域名，回落 SAN",
			cert: &x509.Certificate{
				Subject:  pkix.Name{CommonName: "CloudFlare Origin Certificate"},
				DNSNames: []string{"*.bbua.pp.ua"},
			},
			want: "*.bbua.pp.ua",
		},
		{
			name: "CN 是 IP 时回落 SAN",
			cert: &x509.Certificate{
				Subject:  pkix.Name{CommonName: "182.255.80.150"},
				DNSNames: []string{"panel.example.com"},
			},
			want: "panel.example.com",
		},
	}
	for _, c := range cases {
		if got := certDisplayDomain(c.cert); got != c.want {
			t.Errorf("%s: certDisplayDomain() = %q, want %q", c.name, got, c.want)
		}
	}
}

// 识别 Cloudflare 源站证书（用于前端提示"浏览器不信任"）
func TestIsCfOriginCert(t *testing.T) {
	cf := &x509.Certificate{Issuer: pkix.Name{
		CommonName:   "CloudFlare Origin SSL Certificate Authority",
		Organization: []string{"CloudFlare, Inc."},
	}}
	if !isCfOriginCert(cf) {
		t.Error("Cloudflare 源站证书应被识别")
	}
	le := &x509.Certificate{Issuer: pkix.Name{
		CommonName:   "R11",
		Organization: []string{"Let's Encrypt"},
	}}
	if isCfOriginCert(le) {
		t.Error("Let's Encrypt 证书不应被识别为 CF 源站证书")
	}
}

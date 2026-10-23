package service

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// 订阅节点：把「入站 + 客户端」还原成与格式无关的节点描述，
// 再分别渲染成 v2ray 链接 / Clash / sing-box / Surge。
type SubNode struct {
	Protocol      string // vless / vmess / trojan / shadowsocks
	Remark        string
	Server        string
	Port          int
	UUID          string
	Password      string
	Method        string // shadowsocks 加密方式
	AlterID       int
	Flow          string
	Network       string // tcp / ws / grpc / http
	Security      string // tls / reality / none
	SNI           string
	HostHeader    string
	Path          string
	ServiceName   string
	PublicKey     string
	ShortID       string
	Fingerprint   string
	SpiderX       string
	AllowInsecure bool
}

func (n SubNode) name() string {
	if n.Remark != "" {
		return n.Remark
	}
	return n.Server + ":" + strconv.Itoa(n.Port)
}

// URI 渲染成 v2ray 系标准分享链接
func (n SubNode) URI() string {
	q := url.Values{}
	switch n.Protocol {
	case "vless":
		q.Set("type", orDefault(n.Network, "tcp"))
		q.Set("encryption", "none")
		q.Set("security", orDefault(n.Security, "none"))
		if n.Flow != "" {
			q.Set("flow", n.Flow)
		}
		if n.SNI != "" {
			q.Set("sni", n.SNI)
		}
		if n.Fingerprint != "" {
			q.Set("fp", n.Fingerprint)
		}
		if n.PublicKey != "" {
			q.Set("pbk", n.PublicKey)
		}
		if n.ShortID != "" {
			q.Set("sid", n.ShortID)
		}
		if n.SpiderX != "" {
			q.Set("spx", n.SpiderX)
		}
		applyTransport(q, n)
		return fmt.Sprintf("vless://%s@%s:%d?%s#%s", n.UUID, n.Server, n.Port, q.Encode(), url.PathEscape(n.name()))
	case "trojan":
		q.Set("type", orDefault(n.Network, "tcp"))
		q.Set("security", orDefault(n.Security, "tls"))
		if n.SNI != "" {
			q.Set("sni", n.SNI)
		}
		if n.AllowInsecure {
			q.Set("allowInsecure", "1")
		}
		applyTransport(q, n)
		return fmt.Sprintf("trojan://%s@%s:%d?%s#%s", url.QueryEscape(n.Password), n.Server, n.Port, q.Encode(), url.PathEscape(n.name()))
	case "shadowsocks":
		userinfo := base64.RawURLEncoding.EncodeToString([]byte(n.Method + ":" + n.Password))
		return fmt.Sprintf("ss://%s@%s:%d#%s", userinfo, n.Server, n.Port, url.PathEscape(n.name()))
	case "vmess":
		m := map[string]any{
			"v": "2", "ps": n.name(), "add": n.Server, "port": n.Port,
			"id": n.UUID, "aid": n.AlterID, "scy": "auto",
			"net": orDefault(n.Network, "tcp"), "type": "none",
			"host": n.HostHeader, "path": n.Path,
			"tls": ternary(n.Security == "tls", "tls", ""),
			"sni": n.SNI, "fp": n.Fingerprint,
		}
		if n.Network == "grpc" {
			m["path"] = n.ServiceName
		}
		b, _ := json.Marshal(m)
		return "vmess://" + base64.StdEncoding.EncodeToString(b)
	}
	return ""
}

func applyTransport(q url.Values, n SubNode) {
	switch n.Network {
	case "ws":
		if n.Path != "" {
			q.Set("path", n.Path)
		}
		if n.HostHeader != "" {
			q.Set("host", n.HostHeader)
		}
	case "grpc":
		if n.ServiceName != "" {
			q.Set("serviceName", n.ServiceName)
		}
	case "http":
		if n.Path != "" {
			q.Set("path", n.Path)
		}
		if n.HostHeader != "" {
			q.Set("host", n.HostHeader)
		}
	}
}

// 【开源增强】新订阅 ID：16 位小写字母+数字，作为客户端订阅地址的凭证
func NewSubID() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("sub%015d", time.Now().UnixNano()%1e15)
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

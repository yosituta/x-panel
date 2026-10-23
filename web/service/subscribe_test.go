package service

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func sampleNodes() []SubNode {
	return []SubNode{
		{Protocol: "vless", Remark: "tls-01", Server: "example.com", Port: 443,
			UUID: "11111111-2222-3333-4444-555555555555", Network: "tcp", Security: "tls",
			SNI: "example.com", Fingerprint: "chrome"},
		{Protocol: "vless", Remark: "reality-01", Server: "example.com", Port: 20000,
			UUID: "11111111-2222-3333-4444-555555555555", Network: "tcp", Security: "reality",
			SNI: "www.microsoft.com", PublicKey: "pk-test", ShortID: "ab12", Fingerprint: "chrome",
			SpiderX: "/", Flow: "xtls-rprx-vision"},
	}
}

func TestSubNodeURI(t *testing.T) {
	n := sampleNodes()[0]
	uri := n.URI()
	if !strings.HasPrefix(uri, "vless://") {
		t.Fatalf("vless 链接前缀错误: %s", uri)
	}
	for _, want := range []string{"security=tls", "sni=example.com", "type=tcp", "encryption=none"} {
		if !strings.Contains(uri, want) {
			t.Errorf("链接缺少 %q: %s", want, uri)
		}
	}

	r := sampleNodes()[1]
	ru := r.URI()
	for _, want := range []string{"security=reality", "pbk=pk-test", "sid=ab12", "flow=xtls-rprx-vision"} {
		if !strings.Contains(ru, want) {
			t.Errorf("reality 链接缺少 %q: %s", want, ru)
		}
	}
}

func TestLinksBase64RoundTrip(t *testing.T) {
	out := LinksBase64(sampleNodes())
	if out == "" {
		t.Fatal("订阅内容为空")
	}
	decoded, err := base64.StdEncoding.DecodeString(out)
	if err != nil {
		t.Fatalf("base64 解码失败: %v", err)
	}
	if len(strings.Split(string(decoded), "\n")) != 2 {
		t.Errorf("期望 2 条链接，实际: %q", string(decoded))
	}
}

func TestClashConfig(t *testing.T) {
	out := ClashConfig(sampleNodes(), "标题")
	for _, want := range []string{"proxies:", "type: vless", "reality-opts:", "MATCH,PROXY"} {
		if !strings.Contains(out, want) {
			t.Errorf("Clash 配置缺少 %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "short-id: \"\"") {
		t.Error("reality short-id 不应为空")
	}
}

func TestSingboxConfigValidJSON(t *testing.T) {
	out := SingboxConfig(sampleNodes(), "标题")
	var cfg map[string]any
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("sing-box 输出不是合法 JSON: %v", err)
	}
	if _, ok := cfg["outbounds"]; !ok {
		t.Error("缺少 outbounds")
	}
}

func TestSurgeSkipsReality(t *testing.T) {
	out := SurgeConfig(sampleNodes(), "标题")
	if !strings.Contains(out, "[Proxy]") {
		t.Fatal("缺少 [Proxy] 段")
	}
	if strings.Contains(out, "reality-01") {
		t.Error("Surge 不支持 reality，应跳过该节点")
	}
	if !strings.Contains(out, "tls-01") {
		t.Error("TLS 节点应出现在 Surge 配置中")
	}
}

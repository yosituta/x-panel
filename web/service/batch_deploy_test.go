package service

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestGenX25519Format(t *testing.T) {
	priv, pub, err := genX25519()
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}
	if len(priv) != 43 || len(pub) != 43 {
		t.Fatalf("密钥长度异常: priv=%d pub=%d", len(priv), len(pub))
	}
	if b, err := base64.RawURLEncoding.DecodeString(priv); err != nil || len(b) != 32 {
		t.Fatalf("私钥不是合法的 base64url 32 字节: %v", err)
	}
	if b, err := base64.RawURLEncoding.DecodeString(pub); err != nil || len(b) != 32 {
		t.Fatalf("公钥不是合法的 base64url 32 字节: %v", err)
	}
	priv2, _, _ := genX25519()
	if priv == priv2 {
		t.Fatal("两次生成的私钥相同")
	}
}

func TestGenShortId(t *testing.T) {
	id, err := genShortId()
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 8 || strings.Trim(id, "0123456789abcdef") != "" {
		t.Fatalf("shortId 应为 8 位十六进制，实际 %s", id)
	}
}

func TestBuildRealityInbound(t *testing.T) {
	o := &BatchDeployOptions{
		Count: 1, RemarkPrefix: "test", Fingerprint: "chrome",
		TotalGB: 2, Host: "1.2.3.4", Preset: PresetReality,
	}
	in, d, err := buildInbound(o, 23456, "www.microsoft.com", "test-01", nil)
	if err != nil {
		t.Fatalf("组装失败: %v", err)
	}
	if in.Port != 23456 || in.Tag != "inbound-23456" || string(in.Protocol) != "vless" {
		t.Fatalf("入站基础字段错误: %+v", in)
	}
	wantTotal := int64(2 * 1024 * 1024 * 1024)
	if in.Total != wantTotal {
		t.Fatalf("总流量换算错误: %d != %d", in.Total, wantTotal)
	}

	var settings map[string]any
	if err := json.Unmarshal([]byte(in.Settings), &settings); err != nil {
		t.Fatalf("settings 不是合法 JSON: %v", err)
	}
	c := settings["clients"].([]any)[0].(map[string]any)
	if c["id"] != d.UUID || c["flow"] != "xtls-rprx-vision" {
		t.Fatalf("客户端配置错误: %+v", c)
	}

	var stream map[string]any
	if err := json.Unmarshal([]byte(in.StreamSettings), &stream); err != nil {
		t.Fatalf("streamSettings 不是合法 JSON: %v", err)
	}
	if stream["network"] != "tcp" || stream["security"] != "reality" {
		t.Fatalf("传输层配置错误")
	}
	rs := stream["realitySettings"].(map[string]any)
	if rs["dest"] != "www.microsoft.com:443" {
		t.Fatalf("dest 错误: %v", rs["dest"])
	}
	if rs["settings"].(map[string]any)["publicKey"] != d.PublicKey {
		t.Fatal("publicKey 不一致")
	}
	if !strings.Contains(d.Link, "security=reality") || !strings.Contains(d.Link, "flow=xtls-rprx-vision") {
		t.Fatalf("Reality 链接参数缺失: %s", d.Link)
	}
}

func TestBuildRealityNoVisionHasNoFlow(t *testing.T) {
	o := &BatchDeployOptions{Count: 1, RemarkPrefix: "compat", Fingerprint: "chrome",
		Host: "1.2.3.4", Preset: PresetRealityNoVision}
	in, d, err := buildInbound(o, 23458, "www.apple.com", "compat-01", nil)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	_ = json.Unmarshal([]byte(in.Settings), &settings)
	c := settings["clients"].([]any)[0].(map[string]any)
	if c["flow"] != "" {
		t.Fatalf("no-vision 预设的 flow 应为空，实际 %v", c["flow"])
	}
	if strings.Contains(d.Link, "flow=") {
		t.Fatalf("no-vision 链接不应带 flow: %s", d.Link)
	}
	if !strings.Contains(d.Link, "security=reality") {
		t.Fatalf("仍应是 Reality 链接: %s", d.Link)
	}
}

func TestBuildRealityUDP443(t *testing.T) {
	o := &BatchDeployOptions{Count: 1, RemarkPrefix: "udp", Fingerprint: "chrome",
		Host: "1.2.3.4", Preset: PresetRealityUDP443}
	in, d, err := buildInbound(o, 23459, "www.microsoft.com", "udp-01", nil)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	_ = json.Unmarshal([]byte(in.Settings), &settings)
	c := settings["clients"].([]any)[0].(map[string]any)
	if c["flow"] != "xtls-rprx-vision-udp443" {
		t.Fatalf("flow 应为 xtls-rprx-vision-udp443，实际 %v", c["flow"])
	}
	if !strings.Contains(d.Link, "flow=xtls-rprx-vision-udp443") {
		t.Fatalf("链接应带 udp443 flow: %s", d.Link)
	}
	if !strings.Contains(d.Link, "security=reality") {
		t.Fatalf("仍应是 Reality 链接: %s", d.Link)
	}
}

func TestBuildTLSInbound(t *testing.T) {
	o := &BatchDeployOptions{Count: 1, RemarkPrefix: "tls", Fingerprint: "chrome",
		Host: "us1.example.com", Preset: PresetTLS}
	creds := &tlsCreds{CertFile: "/etc/le/fullchain.pem", KeyFile: "/etc/le/privkey.pem",
		ServerName: "us1.example.com"}
	in, d, err := buildInbound(o, 24443, "", "tls-01", creds)
	if err != nil {
		t.Fatalf("组装 TLS 失败: %v", err)
	}
	var stream map[string]any
	if err := json.Unmarshal([]byte(in.StreamSettings), &stream); err != nil {
		t.Fatal(err)
	}
	if stream["security"] != "tls" {
		t.Fatalf("security 应为 tls: %v", stream["security"])
	}
	tls := stream["tlsSettings"].(map[string]any)
	certs := tls["certificates"].([]any)[0].(map[string]any)
	if certs["certificateFile"] != "/etc/le/fullchain.pem" {
		t.Fatalf("证书路径未写入: %v", certs)
	}
	if tls["serverName"] != "us1.example.com" {
		t.Fatalf("serverName 错误: %v", tls["serverName"])
	}
	if !strings.Contains(d.Link, "security=tls") || !strings.Contains(d.Link, "sni=us1.example.com") {
		t.Fatalf("TLS 链接参数缺失: %s", d.Link)
	}
	if strings.Contains(d.Link, "flow=") {
		t.Fatalf("TLS 链接不应带 flow: %s", d.Link)
	}
}

func TestBuildTLSUsesDomainAsSNI(t *testing.T) {
	o := &BatchDeployOptions{Preset: PresetTLS, Domain: "cdn.example.com", Host: "1.2.3.4"}
	_, d, err := buildInbound(o, 24444, "", "tls-02",
		&tlsCreds{CertFile: "c", KeyFile: "k", ServerName: "cdn.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if d.ServerName != "cdn.example.com" {
		t.Fatalf("SNI 应取证书域名，实际 %s", d.ServerName)
	}
}

func TestBuildTrojanInbound(t *testing.T) {
	o := &BatchDeployOptions{Preset: PresetTrojan, Host: "us1.example.com"}
	in, d, err := buildInbound(o, 24445, "", "trojan-01",
		&tlsCreds{CertFile: "c", KeyFile: "k", ServerName: "us1.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if string(in.Protocol) != "trojan" {
		t.Fatalf("协议应为 trojan: %s", in.Protocol)
	}
	if d.Password == "" {
		t.Fatal("Trojan 密码为空")
	}
	if !strings.HasPrefix(d.Link, "trojan://"+d.Password+"@") {
		t.Fatalf("Trojan 链接格式错误: %s", d.Link)
	}
}

func TestBuildTLSWithoutCertFails(t *testing.T) {
	o := &BatchDeployOptions{Preset: PresetTLS}
	if _, _, err := buildInbound(o, 24446, "", "x", nil); err == nil {
		t.Fatal("没有证书时应当报错")
	}
}

func TestUnknownPresetFails(t *testing.T) {
	o := &BatchDeployOptions{Preset: "nonsense"}
	if _, _, err := buildInbound(o, 24447, "a.com", "x", nil); err == nil {
		t.Fatal("未知预设应当报错")
	}
}

func TestBuildLinkEmptyHost(t *testing.T) {
	o := &BatchDeployOptions{}
	d := &DeployedInbound{Port: 1, UUID: "u", PublicKey: "p", ShortId: "s", ServerName: "d", Preset: PresetReality}
	if link := buildLink(o, d); link != "" {
		t.Fatalf("host 为空时不应生成链接，实际 %q", link)
	}
}

func TestBuildInboundWithListenTag(t *testing.T) {
	o := &BatchDeployOptions{Listen: "10.0.0.1", Fingerprint: "chrome", Preset: PresetReality}
	in, _, err := buildInbound(o, 23457, "www.apple.com", "test-01", nil)
	if err != nil {
		t.Fatal(err)
	}
	if in.Tag != "inbound-10.0.0.1:23457" {
		t.Fatalf("带监听地址的 tag 错误: %s", in.Tag)
	}
}

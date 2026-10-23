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
	// xray 的 x25519 密钥是 32 字节的 base64url（无 padding），长度应为 43
	if len(priv) != 43 || len(pub) != 43 {
		t.Fatalf("密钥长度异常: priv=%d pub=%d", len(priv), len(pub))
	}
	// 应可解码回 32 字节
	if b, err := base64.RawURLEncoding.DecodeString(priv); err != nil || len(b) != 32 {
		t.Fatalf("私钥不是合法的 base64url 32 字节: %v", err)
	}
	if b, err := base64.RawURLEncoding.DecodeString(pub); err != nil || len(b) != 32 {
		t.Fatalf("公钥不是合法的 base64url 32 字节: %v", err)
	}
	// 两次生成必须不同
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
	if len(id) != 8 {
		t.Fatalf("shortId 长度应为 8，实际 %d", len(id))
	}
	if strings.Trim(id, "0123456789abcdef") != "" {
		t.Fatalf("shortId 应为十六进制: %s", id)
	}
}

func TestBuildVlessRealityInbound(t *testing.T) {
	o := &BatchDeployOptions{
		Count:        1,
		RemarkPrefix: "test",
		Fingerprint:  "chrome",
		TotalGB:      2,
		Host:         "1.2.3.4",
	}
	inbound, deployed, err := buildVlessRealityInbound(o, 23456, "www.microsoft.com", "test-01")
	if err != nil {
		t.Fatalf("组装失败: %v", err)
	}

	if inbound.Port != 23456 {
		t.Fatalf("端口错误: %d", inbound.Port)
	}
	if inbound.Tag != "inbound-23456" {
		t.Fatalf("tag 错误: %s", inbound.Tag)
	}
	if string(inbound.Protocol) != "vless" {
		t.Fatalf("协议错误: %s", inbound.Protocol)
	}
	// 2GB -> 字节
	wantTotal := int64(2 * 1024 * 1024 * 1024)
	if inbound.Total != wantTotal {
		t.Fatalf("总流量换算错误: %d != %d", inbound.Total, wantTotal)
	}

	var settings map[string]any
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		t.Fatalf("settings 不是合法 JSON: %v", err)
	}
	if settings["decryption"] != "none" {
		t.Fatalf("decryption 应为 none")
	}
	clients, ok := settings["clients"].([]any)
	if !ok || len(clients) != 1 {
		t.Fatalf("clients 数量错误")
	}
	c := clients[0].(map[string]any)
	if c["id"] != deployed.UUID {
		t.Fatalf("客户端 UUID 与结果不一致")
	}
	if c["flow"] != "xtls-rprx-vision" {
		t.Fatalf("flow 应为 xtls-rprx-vision，实际 %v", c["flow"])
	}
	if c["totalGB"].(float64) != float64(wantTotal) {
		t.Fatalf("客户端 totalGB 换算错误: %v", c["totalGB"])
	}

	var stream map[string]any
	if err := json.Unmarshal([]byte(inbound.StreamSettings), &stream); err != nil {
		t.Fatalf("streamSettings 不是合法 JSON: %v", err)
	}
	if stream["network"] != "tcp" || stream["security"] != "reality" {
		t.Fatalf("传输层配置错误: %v / %v", stream["network"], stream["security"])
	}
	rs := stream["realitySettings"].(map[string]any)
	if rs["dest"] != "www.microsoft.com:443" {
		t.Fatalf("dest 错误: %v", rs["dest"])
	}
	sns := rs["serverNames"].([]any)
	if len(sns) != 1 || sns[0] != "www.microsoft.com" {
		t.Fatalf("serverNames 错误: %v", sns)
	}
	rsSettings := rs["settings"].(map[string]any)
	if rsSettings["publicKey"] != deployed.PublicKey {
		t.Fatalf("publicKey 不一致")
	}
	if rsSettings["fingerprint"] != "chrome" {
		t.Fatalf("fingerprint 错误")
	}

	if !strings.HasPrefix(deployed.Link, "vless://"+deployed.UUID+"@1.2.3.4:23456?") {
		t.Fatalf("分享链接格式错误: %s", deployed.Link)
	}
	if !strings.Contains(deployed.Link, "security=reality") ||
		!strings.Contains(deployed.Link, "flow=xtls-rprx-vision") ||
		!strings.Contains(deployed.Link, "sid="+deployed.ShortId) {
		t.Fatalf("分享链接参数缺失: %s", deployed.Link)
	}
}

func TestBuildVlessLinkEmptyHost(t *testing.T) {
	o := &BatchDeployOptions{}
	d := &DeployedInbound{Port: 1, UUID: "u", PublicKey: "p", ShortId: "s", ServerName: "d"}
	if link := buildVlessLink(o, d); link != "" {
		t.Fatalf("host 为空时不应生成链接，实际 %q", link)
	}
}

func TestBuildInboundWithListenTag(t *testing.T) {
	o := &BatchDeployOptions{Listen: "10.0.0.1", Fingerprint: "chrome"}
	in, _, err := buildVlessRealityInbound(o, 23457, "www.apple.com", "test-01")
	if err != nil {
		t.Fatal(err)
	}
	if in.Tag != "inbound-10.0.0.1:23457" {
		t.Fatalf("带监听地址的 tag 错误: %s", in.Tag)
	}
}

package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// LinksBase64 渲染成 v2ray 标准订阅（base64 后的多行链接）
func LinksBase64(nodes []SubNode) string {
	lines := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if u := n.URI(); u != "" {
			lines = append(lines, u)
		}
	}
	return base64.StdEncoding.EncodeToString([]byte(strings.Join(lines, "\n")))
}

func q(s string) string {
	return strconv.Quote(s)
}

// clashProxy 单条 Clash/Mihomo 代理
func clashProxy(n SubNode) string {
	kv := []string{"name: " + q(n.name())}
	switch n.Protocol {
	case "vless":
		kv = append(kv, "type: vless", "server: "+q(n.Server), "port: "+strconv.Itoa(n.Port),
			"uuid: "+q(n.UUID), "udp: true")
		if n.Flow != "" {
			kv = append(kv, "flow: "+n.Flow)
		}
		if n.Security == "tls" {
			kv = append(kv, "tls: true", "servername: "+q(n.SNI))
			if n.AllowInsecure {
				kv = append(kv, "skip-cert-verify: true")
			}
		}
		if n.Security == "reality" {
			kv = append(kv, "tls: true", "servername: "+q(n.SNI),
				"client-fingerprint: "+q(orDefault(n.Fingerprint, "chrome")),
				fmt.Sprintf("reality-opts: {public-key: %s, short-id: %s}", q(n.PublicKey), q(n.ShortID)))
		}
	case "vmess":
		kv = append(kv, "type: vmess", "server: "+q(n.Server), "port: "+strconv.Itoa(n.Port),
			"uuid: "+q(n.UUID), "alterId: "+strconv.Itoa(n.AlterID), "cipher: auto", "udp: true")
		if n.Security == "tls" {
			kv = append(kv, "tls: true", "servername: "+q(n.SNI), "skip-cert-verify: true")
		}
	case "trojan":
		kv = append(kv, "type: trojan", "server: "+q(n.Server), "port: "+strconv.Itoa(n.Port),
			"password: "+q(n.Password), "udp: true", "sni: "+q(n.SNI),
			"skip-cert-verify: "+strconv.FormatBool(n.AllowInsecure))
	case "shadowsocks":
		kv = append(kv, "type: ss", "server: "+q(n.Server), "port: "+strconv.Itoa(n.Port),
			"cipher: "+q(n.Method), "password: "+q(n.Password), "udp: true")
	default:
		return ""
	}
	switch n.Network {
	case "ws":
		kv = append(kv, "network: ws",
			fmt.Sprintf("ws-opts: {path: %s, headers: {Host: %s}}", q(orDefault(n.Path, "/")), q(n.HostHeader)))
	case "grpc":
		kv = append(kv, "network: grpc", fmt.Sprintf("grpc-opts: {grpc-service-name: %s}", q(n.ServiceName)))
	default:
		kv = append(kv, "network: tcp")
	}
	return "  - {" + strings.Join(kv, ", ") + "}"
}

// ClashConfig 生成可直接导入 Clash/Mihomo 的完整配置
func ClashConfig(nodes []SubNode, title string) string {
	var b strings.Builder
	names := make([]string, 0, len(nodes))
	b.WriteString("# " + title + "\n")
	b.WriteString("mixed-port: 7890\nallow-lan: false\nmode: rule\nlog-level: info\n")
	b.WriteString("proxies:\n")
	for _, n := range nodes {
		if p := clashProxy(n); p != "" {
			b.WriteString(p + "\n")
			names = append(names, q(n.name()))
		}
	}
	if len(names) == 0 {
		return ""
	}
	b.WriteString("proxy-groups:\n")
	b.WriteString("  - {name: \"PROXY\", type: select, proxies: [" + strings.Join(names, ", ") + "]}\n")
	b.WriteString("rules:\n  - MATCH,PROXY\n")
	return b.String()
}

// SurgeConfig 生成 Surge 配置片段（Surge 不支持 reality，自动跳过）
func SurgeConfig(nodes []SubNode, title string) string {
	var b strings.Builder
	b.WriteString("# " + title + "\n[Proxy]\n")
	for _, n := range nodes {
		if n.Security == "reality" {
			continue
		}
		switch n.Protocol {
		case "vless":
			line := fmt.Sprintf("%s = vless, %s, %d, username=%s, tls=true, sni=%s, skip-cert-verify=true",
				n.name(), n.Server, n.Port, n.UUID, n.SNI)
			if n.Flow != "" {
				line += ", flow=" + n.Flow
			}
			b.WriteString(line + "\n")
		case "trojan":
			b.WriteString(fmt.Sprintf("%s = trojan, %s, %d, password=%s, sni=%s, skip-cert-verify=true\n",
				n.name(), n.Server, n.Port, n.Password, n.SNI))
		case "shadowsocks":
			b.WriteString(fmt.Sprintf("%s = ss, %s, %d, encrypt-method=%s, password=%s\n",
				n.name(), n.Server, n.Port, n.Method, n.Password))
		}
	}
	return b.String()
}

// SingboxConfig 生成 sing-box 配置
func SingboxConfig(nodes []SubNode, title string) string {
	outbounds := make([]map[string]any, 0, len(nodes)+2)
	tags := make([]string, 0, len(nodes))
	for _, n := range nodes {
		ob := map[string]any{
			"type": n.Protocol, "tag": n.name(),
			"server": n.Server, "server_port": n.Port,
		}
		switch n.Protocol {
		case "vless":
			ob["uuid"] = n.UUID
			if n.Flow != "" {
				ob["flow"] = n.Flow
			}
		case "vmess":
			ob["uuid"] = n.UUID
			ob["security"] = "auto"
		case "trojan":
			ob["password"] = n.Password
		case "shadowsocks":
			ob["method"] = n.Method
			ob["password"] = n.Password
		}
		if n.Security == "tls" || n.Security == "reality" {
			tls := map[string]any{
				"enabled": true, "server_name": n.SNI,
				"utls": map[string]any{"enabled": true, "fingerprint": orDefault(n.Fingerprint, "chrome")},
			}
			if n.AllowInsecure {
				tls["insecure"] = true
			}
			if n.Security == "reality" {
				tls["reality"] = map[string]any{
					"enabled": true, "public_key": n.PublicKey, "short_id": n.ShortID,
				}
			}
			ob["tls"] = tls
		}
		switch n.Network {
		case "ws":
			ob["transport"] = map[string]any{"type": "ws", "path": orDefault(n.Path, "/"),
				"headers": map[string]any{"Host": n.HostHeader}}
		case "grpc":
			ob["transport"] = map[string]any{"type": "grpc", "service_name": n.ServiceName}
		}
		outbounds = append(outbounds, ob)
		tags = append(tags, n.name())
	}
	if len(tags) > 0 {
		outbounds = append([]map[string]any{{"type": "selector", "tag": "proxy", "outbounds": tags}}, outbounds...)
	}
	outbounds = append(outbounds, map[string]any{"type": "direct", "tag": "direct"})
	cfg := map[string]any{
		"log":       map[string]any{"level": "info"},
		"outbounds": outbounds,
		"route":     map[string]any{"rules": []any{map[string]any{"outbound": "proxy"}}},
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return string(b)
}

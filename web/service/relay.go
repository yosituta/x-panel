package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"x-ui/database/model"
)

// 【开源增强】一键中转节点：
// 入口（本机 VLESS）→ 远程 Socks5/HTTP 上游 → 出网。
// 做法是往 xray 模板里加一个 outbound + 一条 inbound 路由规则，
// 再建对应入站，客户端拿到的就是本机地址的普通节点。

type RelayOptions struct {
	Name         string `json:"name"`
	Port         int    `json:"port"`
	UpstreamType string `json:"upstreamType"` // socks / http，默认 socks
	UpstreamHost string `json:"upstreamHost"`
	UpstreamPort int    `json:"upstreamPort"`
	UpstreamUser string `json:"upstreamUser"`
	UpstreamPass string `json:"upstreamPass"`
	Remark       string `json:"remark"`
}

type RelayService struct {
	InboundService
	SettingService
}

// Create 建中转节点，返回入站与分享链接
func (s *RelayService) Create(o *RelayOptions) (*model.Inbound, string, error) {
	if o.Port < 1024 || o.Port > 65535 {
		return nil, "", fmt.Errorf("入口端口需在 1024-65535 之间")
	}
	if strings.TrimSpace(o.UpstreamHost) == "" || o.UpstreamPort <= 0 {
		return nil, "", fmt.Errorf("上游地址与端口必填")
	}
	if o.UpstreamType == "" {
		o.UpstreamType = "socks"
	}
	if o.Remark == "" {
		o.Remark = fmt.Sprintf("relay-%d", o.Port)
	}

	outboundTag := fmt.Sprintf("relay-%d", o.Port)
	inboundTag := fmt.Sprintf("inbound-%d", o.Port)

	raw, err := s.GetXrayConfigTemplate()
	if err != nil {
		return nil, "", fmt.Errorf("读取 xray 模板失败: %w", err)
	}
	var tmpl map[string]any
	if err := json.Unmarshal([]byte(raw), &tmpl); err != nil {
		return nil, "", fmt.Errorf("xray 模板不是合法 JSON: %w", err)
	}

	// 1) 上游 outbound
	server := map[string]any{"address": o.UpstreamHost, "port": o.UpstreamPort}
	if o.UpstreamUser != "" {
		server["users"] = []map[string]any{{"user": o.UpstreamUser, "pass": o.UpstreamPass}}
	}
	outbound := map[string]any{
		"tag":      outboundTag,
		"protocol": o.UpstreamType,
		"settings": map[string]any{"servers": []map[string]any{server}},
	}
	outbounds, _ := tmpl["outbounds"].([]any)
	// 已存在同名 tag 时先删掉，保证幂等
	filtered := make([]any, 0, len(outbounds)+1)
	for _, ob := range outbounds {
		if m, ok := ob.(map[string]any); ok {
			if tag, _ := m["tag"].(string); tag == outboundTag {
				continue
			}
		}
		filtered = append(filtered, ob)
	}
	tmpl["outbounds"] = append(filtered, outbound)

	// 2) 路由规则
	routing, _ := tmpl["routing"].(map[string]any)
	if routing == nil {
		routing = map[string]any{}
	}
	rules, _ := routing["rules"].([]any)
	kept := make([]any, 0, len(rules)+1)
	for _, r := range rules {
		if m, ok := r.(map[string]any); ok {
			if tag, _ := m["outboundTag"].(string); tag == outboundTag {
				continue
			}
		}
		kept = append(kept, r)
	}
	kept = append(kept, map[string]any{
		"type":        "field",
		"inboundTag":  []string{inboundTag},
		"outboundTag": outboundTag,
	})
	routing["rules"] = kept
	tmpl["routing"] = routing

	bs, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		return nil, "", err
	}
	var xraySetting XraySettingService
	if err := xraySetting.SaveXraySetting(string(bs)); err != nil {
		return nil, "", fmt.Errorf("保存 xray 模板失败: %w", err)
	}

	// 3) 入口入站（VLESS + TCP，无 TLS：靠上游节点自身的安全层）
	clientUUID := uuid.NewString()
	now := time.Now().UnixMilli()
	settings := map[string]any{
		"clients": []map[string]any{{
			"id": clientUUID, "flow": "", "email": fmt.Sprintf("relay-%d@local", o.Port),
			"limitIp": 0, "totalGB": 0, "expiryTime": 0, "enable": true,
			"tgId": 0, "subId": NewSubID(), "comment": "relay", "reset": 0,
			"created_at": now, "updated_at": now,
		}},
		"decryption": "none",
		"fallbacks":  []any{},
	}
	inbound := &model.Inbound{
		UserId:         1,
		Remark:         o.Remark,
		Port:           o.Port,
		Protocol:       model.VLESS,
		Enable:         true,
		Settings:       mustJSON(settings),
		StreamSettings: mustJSON(map[string]any{"network": "tcp", "security": "none"}),
		Sniffing:       mustJSON(map[string]any{"enabled": false}),
		Tag:            inboundTag,
	}
	if _, _, err := s.InboundService.AddInbound(inbound); err != nil {
		return nil, "", err
	}

	host := ""
	var tg Tgbot
	host = tg.ResolveLinkHost()
	link := fmt.Sprintf("vless://%s@%s:%d?type=tcp&encryption=none&security=none#%s",
		clientUUID, host, o.Port, o.Name+strconv.Itoa(0))
	if o.Name != "" {
		link = fmt.Sprintf("vless://%s@%s:%d?type=tcp&encryption=none&security=none#%s",
			clientUUID, host, o.Port, o.Name)
	}
	return inbound, link, nil
}

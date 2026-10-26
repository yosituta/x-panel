package service

import (
	"encoding/json"
	"fmt"

	"x-ui/database/model"
)

// Remove 删除中转节点：入站 + xray 模板里的 outbound 与路由规则
func (s *RelayService) Remove(port int) error {
	outboundTag := fmt.Sprintf("relay-%d", port)

	// 1) 删入站
	inbounds, err := s.InboundService.GetAllInbounds()
	if err != nil {
		return err
	}
	found := false
	for _, ib := range inbounds {
		if ib.Port == port {
			if _, err := s.InboundService.DelInbound(ib.Id); err != nil {
				return err
			}
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("没有找到端口为 %d 的入站", port)
	}

	// 2) 清模板
	raw, err := s.GetXrayConfigTemplate()
	if err != nil {
		return err
	}
	var tmpl map[string]any
	if err := json.Unmarshal([]byte(raw), &tmpl); err != nil {
		return err
	}
	if outbounds, ok := tmpl["outbounds"].([]any); ok {
		kept := make([]any, 0, len(outbounds))
		for _, ob := range outbounds {
			if m, ok := ob.(map[string]any); ok {
				if tag, _ := m["tag"].(string); tag == outboundTag {
					continue
				}
			}
			kept = append(kept, ob)
		}
		tmpl["outbounds"] = kept
	}
	if routing, ok := tmpl["routing"].(map[string]any); ok {
		if rules, ok := routing["rules"].([]any); ok {
			kept := make([]any, 0, len(rules))
			for _, r := range rules {
				if m, ok := r.(map[string]any); ok {
					if tag, _ := m["outboundTag"].(string); tag == outboundTag {
						continue
					}
				}
				kept = append(kept, r)
			}
			routing["rules"] = kept
			tmpl["routing"] = routing
		}
	}
	bs, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		return err
	}
	var xs XraySettingService
	return xs.SaveXraySetting(string(bs))
}

// 便于外部引用
var _ = model.VLESS

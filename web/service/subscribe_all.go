package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"x-ui/database"
	"x-ui/database/model"
)

// AllNodes 汇总全部启用入站下的启用客户端，供 TG /link 命令使用
func (s *SubscribeService) AllNodes(host string) ([]SubNode, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	if err := db.Model(&model.Inbound{}).Preload("ClientStats").Find(&inbounds).Error; err != nil {
		return nil, err
	}
	nodes := make([]SubNode, 0, 16)
	for _, ib := range inbounds {
		if !ib.Enable {
			continue
		}
		var cfg subClients
		if err := json.Unmarshal([]byte(ib.Settings), &cfg); err != nil {
			continue
		}
		for _, c := range cfg.Clients {
			if !c.Enable {
				continue
			}
			node := buildNode(ib, cfg, c, host)
			// 多客户端入站时把邮箱带进备注，避免重名
			if len(cfg.Clients) > 1 && c.Email != "" && !strings.Contains(node.Remark, c.Email) {
				node.Remark = fmt.Sprintf("%s-%s", ib.Remark, c.Email)
			}
			if node.URI() != "" {
				nodes = append(nodes, node)
			}
		}
	}
	return nodes, nil
}

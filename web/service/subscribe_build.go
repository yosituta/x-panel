package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"x-ui/database"
	"x-ui/database/model"
)

// 订阅用户信息，用于 Subscription-Userinfo 响应头
type SubUserInfo struct {
	Upload   int64
	Download int64
	Total    int64
	Expire   int64 // 秒级时间戳，0 表示不限
}

type SubscribeService struct{}

// 入站 settings 里客户端集合的通用结构
type subClients struct {
	Clients []model.Client `json:"clients"`
	Method  string         `json:"method"`
	Password string        `json:"password"`
	Network string         `json:"network"`
}

// streamSettings 只取渲染需要到的字段，兼容平铺与嵌套两种写法
type subStream struct {
	Network         string `json:"network"`
	Security        string `json:"security"`
	TcpSettings     struct {
		Header struct {
			Type   string `json:"type"`
			Request struct {
				Path    []string          `json:"path"`
				Headers map[string]string `json:"headers"`
			} `json:"request"`
		} `json:"header"`
	} `json:"tcpSettings"`
	WsSettings struct {
		Path    string            `json:"path"`
		Headers map[string]string `json:"headers"`
	} `json:"wsSettings"`
	GrpcSettings struct {
		ServiceName string `json:"serviceName"`
	} `json:"grpcSettings"`
	TLSSettings struct {
		ServerName    string `json:"serverName"`
		AllowInsecure bool   `json:"allowInsecure"`
		Settings      struct {
			ServerName    string `json:"serverName"`
			AllowInsecure bool   `json:"allowInsecure"`
		} `json:"settings"`
	} `json:"tlsSettings"`
	RealitySettings struct {
		ServerName string   `json:"serverName"`
		ServerNames []string `json:"serverNames"`
		PublicKey  string   `json:"publicKey"`
		ShortID    string   `json:"shortId"`
		ShortIDs   []string `json:"shortIds"`
		Fingerprint string  `json:"fingerprint"`
		SpiderX    string   `json:"spiderX"`
		Settings   struct {
			PublicKey   string `json:"publicKey"`
			Fingerprint string `json:"fingerprint"`
			SpiderX     string `json:"spiderX"`
			ServerName  string `json:"serverName"`
		} `json:"settings"`
	} `json:"realitySettings"`
}

// NodesBySubID 按订阅 ID 汇总节点；host 为客户端应连接的地址
func (s *SubscribeService) NodesBySubID(subID, host string) ([]SubNode, *SubUserInfo, error) {
	subID = strings.TrimSpace(subID)
	if subID == "" {
		return nil, nil, fmt.Errorf("订阅 ID 为空")
	}
	db := database.GetDB()
	var inbounds []*model.Inbound
	if err := db.Model(&model.Inbound{}).Preload("ClientStats").Find(&inbounds).Error; err != nil {
		return nil, nil, err
	}

	nodes := make([]SubNode, 0, 8)
	info := &SubUserInfo{}
	for _, ib := range inbounds {
		if !ib.Enable {
			continue
		}
		var cfg subClients
		if err := json.Unmarshal([]byte(ib.Settings), &cfg); err != nil {
			continue
		}
		for _, c := range cfg.Clients {
			if c.SubID != subID || !c.Enable {
				continue
			}
			if expire := c.ExpiryTime / 1000; expire > 0 && info.Expire == 0 {
				info.Expire = expire
			}
			info.Total += c.TotalGB
			node := buildNode(ib, cfg, c, host)
			if node.URI() != "" {
				nodes = append(nodes, node)
			}
			// 累积该客户端已用流量
			for _, st := range ib.ClientStats {
				if st.Email == c.Email {
					info.Upload += st.Up
					info.Download += st.Down
				}
			}
		}
	}
	if len(nodes) == 0 {
		return nil, nil, fmt.Errorf("没有找到该订阅下的可用节点")
	}
	return nodes, info, nil
}

// buildNode 把一条客户端配置还原成节点
func buildNode(ib *model.Inbound, cfg subClients, c model.Client, host string) SubNode {
	n := SubNode{
		Protocol: string(ib.Protocol),
		Remark:   ib.Remark,
		Server:   host,
		Port:     ib.Port,
		Network:  "tcp",
		Security: "none",
	}
	if len(cfg.Clients) > 1 && c.Email != "" {
		n.Remark = fmt.Sprintf("%s-%s", ib.Remark, c.Email)
	}
	switch ib.Protocol {
	case model.VLESS:
		n.UUID, n.Flow = c.ID, c.Flow
	case model.VMESS:
		n.UUID, n.AlterID = c.ID, 0
	case model.Trojan:
		n.Password = c.Password
	case model.Shadowsocks:
		n.Method, n.Password = cfg.Method, cfg.Password
		if len(cfg.Clients) > 0 && cfg.Password == "" {
			n.Password = c.Password
		}
	default:
		return n
	}
	applyStream(&n, ib.StreamSettings)
	return n
}

// applyStream 解析传输层与安全层
func applyStream(n *SubNode, raw string) {
	var st subStream
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return
	}
	if st.Network != "" {
		n.Network = st.Network
	}
	n.Security = orDefault(st.Security, "none")
	switch n.Network {
	case "ws":
		n.Path = st.WsSettings.Path
		n.HostHeader = st.WsSettings.Headers["Host"]
	case "grpc":
		n.ServiceName = st.GrpcSettings.ServiceName
	case "http", "h2":
		if len(st.TcpSettings.Header.Request.Path) > 0 {
			n.Path = st.TcpSettings.Header.Request.Path[0]
		}
		n.HostHeader = st.TcpSettings.Header.Request.Headers["Host"]
	}
	switch n.Security {
	case "tls":
		n.SNI = orDefault(st.TLSSettings.ServerName, st.TLSSettings.Settings.ServerName)
		n.AllowInsecure = st.TLSSettings.AllowInsecure || st.TLSSettings.Settings.AllowInsecure
	case "reality":
		rs := st.RealitySettings
		n.SNI = orDefault(rs.ServerName, rs.Settings.ServerName)
		if n.SNI == "" && len(rs.ServerNames) > 0 {
			n.SNI = rs.ServerNames[0]
		}
		n.PublicKey = orDefault(rs.PublicKey, rs.Settings.PublicKey)
		n.Fingerprint = orDefault(rs.Fingerprint, rs.Settings.Fingerprint)
		n.SpiderX = orDefault(rs.SpiderX, rs.Settings.SpiderX)
		n.ShortID = rs.ShortID
		if n.ShortID == "" && len(rs.ShortIDs) > 0 {
			n.ShortID = rs.ShortIDs[0]
		}
	}
}

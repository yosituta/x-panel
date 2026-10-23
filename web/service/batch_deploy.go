package service

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"x-ui/database/model"
	"x-ui/logger"

	"github.com/google/uuid"
)

// 中文注释: 【开源增强】“一键配置 / 批量部署节点”。
//
// 一次点击批量生成 N 条入站，每条使用独立的端口、UUID、密钥，并直接热加载进 Xray。
// 支持 4 种预设，覆盖不同客户端的兼容性：
//
//	reality           VLESS + TCP + Reality + Vision（默认，最新客户端）
//	reality-novision  VLESS + TCP + Reality（不带 flow，兼容老版 Shadowrocket / v2rayN）
//	reality-udp443    VLESS + TCP + Reality + Vision + UDP443（xtls-rprx-vision-udp443）
//	tls               VLESS + TCP + TLS（用面板已配置的证书，任何客户端都支持）
//	trojan            Trojan + TCP + TLS（同上）
//
// 为什么需要 non-Reality 预设：部分客户端（尤其 iOS 上的旧版/共享版 Shadowrocket）
// 对 REALITY 支持不完整，表现是“延迟测试通过、但一个字节都传不出去”。
// 这时换 TLS 预设即可，不需要换服务器。

const (
	PresetReality         = "reality"
	PresetRealityNoVision = "reality-novision"
	PresetRealityUDP443   = "reality-udp443"
	PresetRealityLegacy   = "reality-legacy" // 完全复刻上游旧版“一键配置”的写法
	PresetTLS             = "tls"
	PresetTrojan          = "trojan"

	DefaultBatchCount   = 10
	DefaultBatchPrefix  = "reality"
	DefaultFingerprint  = "chrome"
	DefaultSpiderX      = "/"
	defaultBatchPort    = 20000 // 未指定时的默认起始端口
	minUsablePort       = 1024  // 允许用户指定 1024 起的端口（如 2053 / 443 之外的常用端口）
	maxBatchPort        = 60000
	batchPortMaxRetries = 4096
)

// BatchDeployOptions 批量部署参数
type BatchDeployOptions struct {
	UserId       int    `json:"userId" form:"userId"`
	Count        int    `json:"count" form:"count"`               // 生成条数，默认 10
	StartPort    int    `json:"startPort" form:"startPort"`       // 起始端口，0 表示自动挑选
	RemarkPrefix string `json:"remarkPrefix" form:"remarkPrefix"` // 备注前缀
	Domain       string `json:"domain" form:"domain"`             // Reality: 偷用域名；TLS: 证书域名
	Listen       string `json:"listen" form:"listen"`             // 监听地址，默认空（全部地址）
	Fingerprint  string `json:"fingerprint" form:"fingerprint"`   // uTLS 指纹
	TotalGB      int64  `json:"totalGB" form:"totalGB"`           // 单条入站流量上限（GB，0=不限）
	ExpiryTime   int64  `json:"expiryTime" form:"expiryTime"`     // 到期时间（0=不过期）
	Host         string `json:"host" form:"host"`                 // 生成分享链接用的地址
	Preset       string `json:"preset" form:"preset"`             // 预设协议，见上方常量
}

// DeployedInbound 单条部署结果
type DeployedInbound struct {
	Id         int    `json:"id"`
	Remark     string `json:"remark"`
	Preset     string `json:"preset"`
	Port       int    `json:"port"`
	UUID       string `json:"uuid"`
	Password   string `json:"password,omitempty"`
	PublicKey  string `json:"publicKey,omitempty"`
	ShortId    string `json:"shortId,omitempty"`
	ServerName string `json:"serverName,omitempty"`
	Link       string `json:"link"`
}

// BatchDeployResult 批量部署结果
type BatchDeployResult struct {
	Created   []DeployedInbound `json:"created"`
	Failed    []string          `json:"failed"`
	Restarted bool              `json:"restarted"`
}

// legacyRealityDests 上游旧版一键配置使用的目标域名（原文就是裸域名，dest 形如 tesla.com:443）
var legacyRealityDests = []string{
	"tesla.com",
	"sega.com",
	"apple.com",
	"icloud.com",
	"lovelive-anime.jp",
	"meta.com",
}

type BatchDeployService struct{}

// tlsCreds TLS 预设要用的证书信息
type tlsCreds struct {
	CertFile   string
	KeyFile    string
	ServerName string
}

func genX25519() (privateKey, publicKey string, err error) {
	curve := ecdh.X25519()
	key, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	privateKey = base64.RawURLEncoding.EncodeToString(key.Bytes())
	publicKey = base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())
	return privateKey, publicKey, nil
}

func genShortId() (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func genPassword(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func portAvailable(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

func buildTag(o *BatchDeployOptions, port int) string {
	if o.Listen != "" && o.Listen != "0.0.0.0" && o.Listen != "::" {
		return fmt.Sprintf("inbound-%s:%d", o.Listen, port)
	}
	return fmt.Sprintf("inbound-%d", port)
}

// newClient 生成统一的客户端配置片段
func newVlessClient(id, email, flow string, totalBytes, expiry int64, now int64) map[string]any {
	return map[string]any{
		"id": id, "flow": flow, "email": email, "limitIp": 0,
		"totalGB": totalBytes, "expiryTime": expiry, "enable": true,
		"tgId": 0, "subId": "", "comment": "", "reset": 0,
		"created_at": now, "updated_at": now,
	}
}

func mustJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

func defaultSniffing() map[string]any {
	return map[string]any{
		"enabled":      false,
		"destOverride": []string{"http", "tls", "quic"},
		"metadataOnly": false,
		"routeOnly":    false,
	}
}

// buildInbound 按预设组装一条入站（纯函数，便于单测）
func buildInbound(o *BatchDeployOptions, port int, domain, remark string, creds *tlsCreds) (*model.Inbound, *DeployedInbound, error) {
	preset := o.Preset
	if preset == "" {
		preset = PresetReality
	}
	email := fmt.Sprintf("%s-%d", strings.SplitN(preset, "-", 2)[0], port)
	now := time.Now().UnixMilli()
	totalBytes := o.TotalGB * 1024 * 1024 * 1024

	inbound := &model.Inbound{
		UserId:      o.UserId,
		Remark:      remark,
		Enable:      true,
		ExpiryTime:  o.ExpiryTime,
		Total:       totalBytes,
		Listen:      o.Listen,
		Port:        port,
		Tag:         buildTag(o, port),
		ResetPeriod: ResetPeriodNever,
		ResetDay:    1,
	}
	deployed := &DeployedInbound{Remark: remark, Preset: preset, Port: port}

	switch preset {
	case PresetReality, PresetRealityNoVision, PresetRealityUDP443:
		flow := "xtls-rprx-vision"
		switch preset {
		case PresetRealityNoVision:
			flow = ""
		case PresetRealityUDP443:
			flow = "xtls-rprx-vision-udp443"
		}
		clientUUID := uuid.NewString()
		privateKey, publicKey, err := genX25519()
		if err != nil {
			return nil, nil, err
		}
		shortId, err := genShortId()
		if err != nil {
			return nil, nil, err
		}
		inbound.Protocol = model.VLESS
		inbound.Settings = mustJSON(map[string]any{
			"clients":    []map[string]any{newVlessClient(clientUUID, email, flow, totalBytes, o.ExpiryTime, now)},
			"decryption": "none",
			"fallbacks":  []any{},
		})
		inbound.StreamSettings = mustJSON(map[string]any{
			"network":  "tcp",
			"security": "reality",
			"realitySettings": map[string]any{
				"show":        false,
				"xver":        0,
				"dest":        net.JoinHostPort(domain, "443"),
				"serverNames": []string{domain},
				"privateKey":  privateKey,
				"shortIds":    []string{shortId},
				"settings": map[string]any{
					"publicKey":   publicKey,
					"fingerprint": o.Fingerprint,
					"serverName":  "",
					"spiderX":     DefaultSpiderX,
				},
			},
		})
		inbound.Sniffing = mustJSON(defaultSniffing())
		deployed.UUID = clientUUID
		deployed.PublicKey = publicKey
		deployed.ShortId = shortId
		deployed.ServerName = domain

	case PresetRealityLegacy:
		clientUUID := uuid.NewString()
		privateKey, publicKey, err := genX25519()
		if err != nil {
			return nil, nil, err
		}
		shortId, err := genShortId()
		if err != nil {
			return nil, nil, err
		}
		bare := domain
		inbound.Protocol = model.VLESS
		// 与上游旧版 createVlessRealityInbound 的 JSON 结构保持一致
		inbound.Settings = mustJSON(map[string]any{
			"clients": []map[string]any{{
				"id": clientUUID, "flow": "xtls-rprx-vision", "email": email,
				"level": 0, "enable": true,
			}},
			"decryption": "none",
			"fallbacks":  []any{},
		})
		inbound.StreamSettings = mustJSON(map[string]any{
			"network":  "tcp",
			"security": "reality",
			"realitySettings": map[string]any{
				"show":         false,
				"target":       net.JoinHostPort(bare, "443"),
				"xver":         0,
				"serverNames":  []string{bare, "www." + bare},
				"settings":     map[string]any{"publicKey": publicKey, "spiderX": "/", "mldsa65Verify": ""},
				"privateKey":   privateKey,
				"maxClientVer": "", "minClientVer": "", "maxTimediff": 0, "mldsa65Seed": "",
				"shortIds": []string{shortId},
			},
			"tcpSettings": map[string]any{
				"acceptProxyProtocol": false,
				"header":              map[string]any{"type": "none"},
			},
		})
		inbound.Sniffing = mustJSON(map[string]any{
			"enabled":      true,
			"destOverride": []string{"http", "tls", "quic", "fakedns"},
			"metadataOnly": false,
			"routeOnly":    false,
		})
		deployed.UUID = clientUUID
		deployed.PublicKey = publicKey
		deployed.ShortId = shortId
		deployed.ServerName = bare

	case PresetTLS:
		if creds == nil || creds.CertFile == "" || creds.KeyFile == "" {
			return nil, nil, fmt.Errorf("TLS 预设需要在面板【设置】里配置证书（webCertFile / webKeyFile）")
		}
		clientUUID := uuid.NewString()
		inbound.Protocol = model.VLESS
		inbound.Settings = mustJSON(map[string]any{
			"clients":    []map[string]any{newVlessClient(clientUUID, email, "", totalBytes, o.ExpiryTime, now)},
			"decryption": "none",
			"fallbacks":  []any{},
		})
		inbound.StreamSettings = mustJSON(map[string]any{
			"network":  "tcp",
			"security": "tls",
			"tlsSettings": map[string]any{
				"serverName": creds.ServerName,
				"alpn":       []string{"h2", "http/1.1"},
				"certificates": []map[string]any{{
					"certificateFile": creds.CertFile,
					"keyFile":         creds.KeyFile,
				}},
			},
		})
		sniff := defaultSniffing()
		sniff["enabled"] = true
		inbound.Sniffing = mustJSON(sniff)
		deployed.UUID = clientUUID
		deployed.ServerName = creds.ServerName

	case PresetTrojan:
		if creds == nil || creds.CertFile == "" || creds.KeyFile == "" {
			return nil, nil, fmt.Errorf("Trojan 预设需要在面板【设置】里配置证书（webCertFile / webKeyFile）")
		}
		password, err := genPassword(16)
		if err != nil {
			return nil, nil, err
		}
		inbound.Protocol = model.Trojan
		inbound.Settings = mustJSON(map[string]any{
			"clients": []map[string]any{{
				"password": password, "email": email, "limitIp": 0,
				"totalGB": totalBytes, "expiryTime": o.ExpiryTime, "enable": true,
				"tgId": 0, "subId": "", "comment": "", "reset": 0,
				"created_at": now, "updated_at": now,
			}},
			"fallbacks": []any{},
		})
		inbound.StreamSettings = mustJSON(map[string]any{
			"network":  "tcp",
			"security": "tls",
			"tlsSettings": map[string]any{
				"serverName": creds.ServerName,
				"alpn":       []string{"h2", "http/1.1"},
				"certificates": []map[string]any{{
					"certificateFile": creds.CertFile,
					"keyFile":         creds.KeyFile,
				}},
			},
		})
		sniff := defaultSniffing()
		sniff["enabled"] = true
		inbound.Sniffing = mustJSON(sniff)
		deployed.Password = password
		deployed.ServerName = creds.ServerName

	default:
		return nil, nil, fmt.Errorf("未知预设: %s", preset)
	}

	deployed.Link = buildLink(o, deployed)
	return inbound, deployed, nil
}

// buildLink 生成分享链接
func buildLink(o *BatchDeployOptions, d *DeployedInbound) string {
	if o.Host == "" {
		return ""
	}
	host := o.Host
	fp := o.Fingerprint
	if fp == "" {
		fp = DefaultFingerprint
	}
	switch d.Preset {
	case PresetRealityLegacy:
		return fmt.Sprintf(
			"vless://%s@%s:%d?type=tcp&encryption=none&security=reality&pbk=%s&fp=%s&sni=%s&sid=%s&spx=%%2F&flow=xtls-rprx-vision#%s",
			d.UUID, host, d.Port, d.PublicKey, fp, d.ServerName, d.ShortId, d.Remark,
		)
	case PresetReality, PresetRealityNoVision, PresetRealityUDP443:
		flow := ""
		switch d.Preset {
		case PresetReality:
			flow = "&flow=xtls-rprx-vision"
		case PresetRealityUDP443:
			flow = "&flow=xtls-rprx-vision-udp443"
		}
		return fmt.Sprintf(
			"vless://%s@%s:%d?type=tcp&encryption=none&security=reality&pbk=%s&fp=%s&sni=%s&sid=%s&spx=%%2F%s#%s",
			d.UUID, host, d.Port, d.PublicKey, fp, d.ServerName, d.ShortId, flow, d.Remark,
		)
	case PresetTLS:
		return fmt.Sprintf(
			"vless://%s@%s:%d?type=tcp&encryption=none&security=tls&sni=%s&fp=%s#%s",
			d.UUID, host, d.Port, d.ServerName, fp, d.Remark,
		)
	case PresetTrojan:
		return fmt.Sprintf(
			"trojan://%s@%s:%d?security=tls&sni=%s&fp=%s#%s",
			d.Password, host, d.Port, d.ServerName, fp, d.Remark,
		)
	}
	return ""
}

// Deploy 执行批量部署。返回 (结果, 是否需要重启 Xray, 错误)
func (s *BatchDeployService) Deploy(o *BatchDeployOptions) (*BatchDeployResult, bool, error) {
	if o == nil {
		return nil, false, fmt.Errorf("参数不能为空")
	}
	if o.Count <= 0 {
		o.Count = DefaultBatchCount
	}
	if o.Count > 50 {
		o.Count = 50
	}
	if o.RemarkPrefix == "" {
		o.RemarkPrefix = DefaultBatchPrefix
	}
	if o.Fingerprint == "" {
		o.Fingerprint = DefaultFingerprint
	}
	if o.Preset == "" {
		o.Preset = PresetReality
	}

	result := &BatchDeployResult{
		Created: make([]DeployedInbound, 0, o.Count),
		Failed:  make([]string, 0),
	}

	// TLS / Trojan 预设需要证书：取面板设置里的证书，域名优先用请求里的 Domain，其次 Host
	var creds *tlsCreds
	if o.Preset == PresetTLS || o.Preset == PresetTrojan {
		var settingService SettingService
		cert, err1 := settingService.GetCertFile()
		key, err2 := settingService.GetKeyFile()
		if err1 != nil || err2 != nil || cert == "" || key == "" {
			return nil, false, fmt.Errorf("TLS/Trojan 预设需要先在面板【设置】里配置 HTTPS 证书")
		}
		serverName := o.Domain
		if serverName == "" {
			serverName = o.Host
		}
		if serverName == "" {
			return nil, false, fmt.Errorf("TLS/Trojan 预设需要提供域名（证书对应的域名）")
		}
		creds = &tlsCreds{CertFile: cert, KeyFile: key, ServerName: serverName}
	}

	// Reality 预设需要偷用域名（未指定则探测一个可用的）
	domain := o.Domain
	if o.Preset == PresetRealityLegacy {
		// 上游旧版：从固定域名池随机挑一个裸域名
		if domain == "" {
			domain = legacyRealityDests[time.Now().UnixNano()%int64(len(legacyRealityDests))]
		}
	} else if o.Preset == PresetReality || o.Preset == PresetRealityNoVision || o.Preset == PresetRealityUDP443 {
		if domain == "" {
			var domainService RealityDomainService
			domain = domainService.RandomAliveDomain(3 * time.Second).Domain
		}
	}

	port := o.StartPort
	if port < minUsablePort || port > maxBatchPort {
		port = defaultBatchPort
	}

	var inboundService InboundService
	needRestart := false
	attempts := 0

	for i := 0; i < o.Count; i++ {
		for {
			if attempts > batchPortMaxRetries {
				result.Failed = append(result.Failed, fmt.Sprintf("第 %d 条: 找不到可用端口", i+1))
				attempts = 0
				break
			}
			attempts++
			existed, _ := inboundService.checkPortExist(o.Listen, port, 0)
			if !existed && portAvailable(port) {
				break
			}
			port++
			if port > 65535 {
				result.Failed = append(result.Failed, fmt.Sprintf("第 %d 条: 端口已用尽", i+1))
				attempts = batchPortMaxRetries + 1
				break
			}
		}
		if attempts > batchPortMaxRetries {
			attempts = 0
			continue
		}

		remark := fmt.Sprintf("%s-%02d", o.RemarkPrefix, i+1)
		inbound, deployed, err := buildInbound(o, port, domain, remark, creds)
		if err != nil {
			logger.Warningf("批量部署第 %d 条组装失败: %v", i+1, err)
			result.Failed = append(result.Failed, fmt.Sprintf("第 %d 条: %v", i+1, err))
			port++
			continue
		}

		saved, restart, err := inboundService.AddInbound(inbound)
		if err != nil {
			logger.Warningf("批量部署第 %d 条失败: %v", i+1, err)
			result.Failed = append(result.Failed, fmt.Sprintf("第 %d 条(端口 %d): %v", i+1, port, err))
			port++
			continue
		}
		if restart {
			needRestart = true
		}
		deployed.Id = saved.Id
		result.Created = append(result.Created, *deployed)
		port++
	}

	result.Restarted = needRestart
	return result, needRestart, nil
}

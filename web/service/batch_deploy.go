package service

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"x-ui/database/model"
	"x-ui/logger"

	"github.com/google/uuid"
)

// 中文注释: 【开源增强】“一键配置 / 批量部署节点”。
//
// 依据上游 README 的描述复刻：一次点击批量生成 N 条
// VLESS + TCP + Reality + Vision 入站，每条使用独立的
// 端口、UUID、x25519 密钥对与 shortId，并直接热加载进 Xray。
//
// 与上游 PRO 版的实现无关，全部为本仓库自行实现的逻辑。

const (
	DefaultBatchCount   = 10
	DefaultBatchPrefix  = "reality"
	DefaultFingerprint  = "chrome"
	DefaultSpiderX      = "/"
	minBatchPort        = 20000
	maxBatchPort        = 60000
	batchPortMaxRetries = 4096
)

// BatchDeployOptions 批量部署参数
type BatchDeployOptions struct {
	UserId       int    `json:"userId" form:"userId"`
	Count        int    `json:"count" form:"count"`               // 生成条数，默认 10
	StartPort    int    `json:"startPort" form:"startPort"`       // 起始端口，0 表示自动挑选
	RemarkPrefix string `json:"remarkPrefix" form:"remarkPrefix"` // 备注前缀
	Domain       string `json:"domain" form:"domain"`             // 偷用的域名，空则自动探测
	Listen       string `json:"listen" form:"listen"`             // 监听地址，默认空（全部地址）
	Fingerprint  string `json:"fingerprint" form:"fingerprint"`   // uTLS 指纹
	TotalGB      int64  `json:"totalGB" form:"totalGB"`           // 单条入站流量上限（0=不限）
	ExpiryTime   int64  `json:"expiryTime" form:"expiryTime"`     // 到期时间（0=不过期）
	Host         string `json:"host" form:"host"`                 // 生成分享链接用的地址
}

// DeployedInbound 单条部署结果
type DeployedInbound struct {
	Id         int    `json:"id"`
	Remark     string `json:"remark"`
	Port       int    `json:"port"`
	UUID       string `json:"uuid"`
	PublicKey  string `json:"publicKey"`
	ShortId    string `json:"shortId"`
	ServerName string `json:"serverName"`
	Link       string `json:"link"`
}

// BatchDeployResult 批量部署结果
type BatchDeployResult struct {
	Created   []DeployedInbound `json:"created"`
	Failed    []string          `json:"failed"`
	Restarted bool              `json:"restarted"`
}

type BatchDeployService struct{}

// genX25519 生成 Reality 所需的 x25519 密钥对（base64url raw 编码，与 xray x25519 输出一致）
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

// genShortId 生成 8 位十六进制 shortId
func genShortId() (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// portAvailable 判断端口是否可绑定
func portAvailable(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

// buildVlessRealityInbound 组装一条 VLESS+TCP+Reality+Vision 入站（纯函数，便于单测）
func buildVlessRealityInbound(o *BatchDeployOptions, port int, domain, remark string) (*model.Inbound, *DeployedInbound, error) {
	clientUUID := uuid.NewString()
	privateKey, publicKey, err := genX25519()
	if err != nil {
		return nil, nil, err
	}
	shortId, err := genShortId()
	if err != nil {
		return nil, nil, err
	}

	email := fmt.Sprintf("vless-%d", port)
	now := time.Now().UnixMilli()
	// 前端传入的 TotalGB 为 GB，模型内统一用字节
	totalBytes := o.TotalGB * 1024 * 1024 * 1024

	settings := map[string]any{
		"clients": []map[string]any{{
			"id":         clientUUID,
			"flow":       "xtls-rprx-vision",
			"email":      email,
			"limitIp":    0,
			"totalGB":    totalBytes,
			"expiryTime": o.ExpiryTime,
			"enable":     true,
			"tgId":       0,
			"subId":      "",
			"comment":    "",
			"reset":      0,
			"created_at": now,
			"updated_at": now,
		}},
		"decryption": "none",
		"fallbacks":  []any{},
	}
	streamSettings := map[string]any{
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
	}
	sniffing := map[string]any{
		"enabled":      false,
		"destOverride": []string{"http", "tls", "quic"},
		"metadataOnly": false,
		"routeOnly":    false,
	}

	settingsBytes, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	streamBytes, err := json.MarshalIndent(streamSettings, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	sniffingBytes, err := json.MarshalIndent(sniffing, "", "  ")
	if err != nil {
		return nil, nil, err
	}

	tag := fmt.Sprintf("inbound-%d", port)
	if o.Listen != "" && o.Listen != "0.0.0.0" && o.Listen != "::" {
		tag = fmt.Sprintf("inbound-%s:%d", o.Listen, port)
	}

	inbound := &model.Inbound{
		UserId:         o.UserId,
		Remark:         remark,
		Enable:         true,
		ExpiryTime:     o.ExpiryTime,
		Total:          totalBytes,
		Listen:         o.Listen,
		Port:           port,
		Protocol:       model.VLESS,
		Settings:       string(settingsBytes),
		StreamSettings: string(streamBytes),
		Sniffing:       string(sniffingBytes),
		Tag:            tag,
		ResetPeriod:    ResetPeriodNever,
		ResetDay:       1,
	}

	deployed := &DeployedInbound{
		Remark:     remark,
		Port:       port,
		UUID:       clientUUID,
		PublicKey:  publicKey,
		ShortId:    shortId,
		ServerName: domain,
	}
	deployed.Link = buildVlessLink(o, deployed)

	return inbound, deployed, nil
}

// buildVlessLink 组装 vless:// 分享链接
func buildVlessLink(o *BatchDeployOptions, d *DeployedInbound) string {
	if o.Host == "" {
		return ""
	}
	return fmt.Sprintf(
		"vless://%s@%s:%d?type=tcp&security=reality&pbk=%s&fp=%s&sni=%s&sid=%s&spx=%%2F&flow=xtls-rprx-vision#%s",
		d.UUID, o.Host, d.Port, d.PublicKey, o.Fingerprint, d.ServerName, d.ShortId, d.Remark,
	)
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

	result := &BatchDeployResult{
		Created: make([]DeployedInbound, 0, o.Count),
		Failed:  make([]string, 0),
	}

	// 1. 选定偷用域名（未指定则探测一个可用的）
	domain := o.Domain
	if domain == "" {
		var domainService RealityDomainService
		domain = domainService.RandomAliveDomain(3 * time.Second).Domain
	}

	// 2. 端口起点
	port := o.StartPort
	if port <= 0 || port < minBatchPort || port > maxBatchPort {
		port = minBatchPort
	}

	var inboundService InboundService
	needRestart := false
	attempts := 0

	for i := 0; i < o.Count; i++ {
		// 找一个可用端口（既要能绑定，也要没被现有入站占用）
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
		inbound, deployed, err := buildVlessRealityInbound(o, port, domain, remark)
		if err != nil {
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

package service

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"x-ui/database/model"
)

// 【开源增强】面板证书：网页里直接粘贴 / 上传，不依赖 SSH 与 API Key
//
// 支持的证书类型：
//   - Cloudflare Origin CA（15 年，需域名橙云）
//   - Let's Encrypt / ZeroSSL 等任意 PEM 证书
//   - 自签证书
type CertService struct{}

// CertInfo 当前绑定状态，供前端展示
type CertInfo struct {
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
	Domain   string `json:"domain"`
	Issuer   string `json:"issuer"`
	NotAfter string `json:"notAfter"`
	Days     int    `json:"days"`
}

func (s *CertService) certDir(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimPrefix(domain, "*.")
	domain = strings.ReplaceAll(domain, "..", ".")
	domain = strings.ReplaceAll(domain, "/", "")
	return filepath.Join("/root/cert", domain)
}

// Status 读取当前绑定的证书信息
func (s *CertService) Status() (*CertInfo, error) {
	var settingService SettingService
	certFile, _ := settingService.GetCertFile()
	keyFile, _ := settingService.GetKeyFile()
	info := &CertInfo{CertFile: certFile, KeyFile: keyFile}
	if certFile == "" {
		return info, nil
	}
	data, err := os.ReadFile(certFile)
	if err != nil {
		return info, nil
	}
	cert, err := parseFirstCert(data)
	if err != nil {
		return info, err
	}
	info.Domain = cert.Subject.CommonName
	info.Issuer = cert.Issuer.CommonName
	info.NotAfter = cert.NotAfter.Format("2006-01-02")
	info.Days = int(time.Until(cert.NotAfter).Hours() / 24)
	if len(cert.DNSNames) > 0 && info.Domain == "" {
		info.Domain = cert.DNSNames[0]
	}
	return info, nil
}

// Save 校验证书/私钥，落盘并写入设置（调用方负责重启面板）
func (s *CertService) Save(domain, certPEM, keyPEM string) (*CertInfo, error) {
	domain = strings.TrimSpace(domain)
	certPEM = strings.TrimSpace(certPEM)
	keyPEM = strings.TrimSpace(keyPEM)

	if certPEM == "" || keyPEM == "" {
		return nil, errors.New("证书和私钥都不能为空")
	}
	if !strings.Contains(certPEM, "BEGIN CERTIFICATE") {
		return nil, errors.New("证书格式不对，应以 -----BEGIN CERTIFICATE----- 开头")
	}
	if !strings.Contains(keyPEM, "BEGIN") || !strings.Contains(keyPEM, "PRIVATE KEY") {
		return nil, errors.New("私钥格式不对，应以 -----BEGIN PRIVATE KEY----- 开头")
	}

	cert, err := parseFirstCert([]byte(certPEM))
	if err != nil {
		return nil, fmt.Errorf("证书解析失败：%v", err)
	}

	// 校验私钥与证书是否配对
	if err := verifyKeyMatch(cert, []byte(keyPEM)); err != nil {
		return nil, err
	}

	// 域名一致性：给了域名就要求证书覆盖它
	if domain != "" {
		if !certCoversHost(cert, domain) {
			return nil, fmt.Errorf("该证书不包含域名 %s，请确认申请时填写的域名一致", domain)
		}
	} else if len(cert.DNSNames) > 0 {
		domain = cert.DNSNames[0]
	} else if cert.Subject.CommonName != "" {
		domain = cert.Subject.CommonName
	}
	if domain == "" {
		return nil, errors.New("无法从证书中识别域名，请手动填写域名")
	}

	dir := s.certDir(domain)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建目录失败：%v", err)
	}
	certPath := filepath.Join(dir, "fullchain.pem")
	keyPath := filepath.Join(dir, "privkey.pem")

	if err := os.WriteFile(certPath, []byte(certPEM+"\n"), 0o644); err != nil {
		return nil, fmt.Errorf("写入证书失败：%v", err)
	}
	if err := os.WriteFile(keyPath, []byte(keyPEM+"\n"), 0o600); err != nil {
		return nil, fmt.Errorf("写入私钥失败：%v", err)
	}

	var settingService SettingService
	if err := settingService.SetCertFile(certPath); err != nil {
		return nil, err
	}
	if err := settingService.SetKeyFile(keyPath); err != nil {
		return nil, err
	}

	return &CertInfo{
		CertFile: certPath,
		KeyFile:  keyPath,
		Domain:   domain,
		Issuer:   cert.Issuer.CommonName,
		NotAfter: cert.NotAfter.Format("2006-01-02"),
		Days:     int(time.Until(cert.NotAfter).Hours() / 24),
	}, nil
}

// Clear 清除绑定，面板将回到「无证书」状态（只监听 127.0.0.1）
func (s *CertService) Clear() error {
	var settingService SettingService
	if err := settingService.SetCertFile(""); err != nil {
		return err
	}
	return settingService.SetKeyFile("")
}

func parseFirstCert(data []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("不是有效的 PEM 证书")
	}
	return x509.ParseCertificate(block.Bytes)
}

func certCoversHost(cert *x509.Certificate, host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	for _, n := range cert.DNSNames {
		n = strings.ToLower(n)
		if n == host {
			return true
		}
		if strings.HasPrefix(n, "*.") && strings.HasSuffix(host, n[1:]) &&
			strings.Count(host, ".") == strings.Count(n, ".") {
			return true
		}
	}
	if strings.EqualFold(cert.Subject.CommonName, host) {
		return true
	}
	return false
}

func verifyKeyMatch(cert *x509.Certificate, keyPEM []byte) error {
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return errors.New("私钥不是有效的 PEM 格式")
	}
	var key any
	var err error
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		key, err = x509.ParseECPrivateKey(block.Bytes)
	default:
		return fmt.Errorf("不支持的私钥类型：%s", block.Type)
	}
	if err != nil {
		return fmt.Errorf("私钥解析失败：%v", err)
	}

	// 不同私钥类型的 Public() 签名不一致，这里显式分支处理
	var pub any
	switch k := key.(type) {
	case *rsa.PrivateKey:
		pub = &k.PublicKey
	case *ecdsa.PrivateKey:
		pub = &k.PublicKey
	case ed25519.PrivateKey:
		pub = k.Public()
	default:
		return errors.New("不支持的私钥类型")
	}

	certPub, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return err
	}
	keyPub, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return err
	}
	if sha256.Sum256(certPub) != sha256.Sum256(keyPub) {
		return errors.New("证书与私钥不匹配，请确认两者是同一对")
	}
	return nil
}

// 保证 model 包被引用（用于类型对齐，避免误删导入）
var _ = model.Setting{}

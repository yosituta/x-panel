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
	"net"
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
	// CfOrigin 是否为 Cloudflare 源站证书（15 年）
	// 这类证书只被 Cloudflare 代理信任：直接把面板暴露在 54321 上，
	// 浏览器照样提示"不安全"，必须让用户知道原因。
	CfOrigin bool `json:"cfOrigin"`
	// SelfSigned 自签证书（颁发者与主体相同）
	SelfSigned bool `json:"selfSigned"`
	// Trusted 是否为公共 CA 签发（浏览器默认可信）
	Trusted bool `json:"trusted"`
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
	info.Domain = certDisplayDomain(cert)
	info.Issuer = cert.Issuer.CommonName
	info.NotAfter = cert.NotAfter.Format("2006-01-02")
	info.Days = int(time.Until(cert.NotAfter).Hours() / 24)
	info.CfOrigin = isCfOriginCert(cert)
	info.SelfSigned = cert.Issuer.String() == cert.Subject.String()
	info.Trusted = !info.CfOrigin && !info.SelfSigned && isPublicCA(cert)
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

	// 记住用户实际访问用的域名（通配证书不覆盖已有记录）
	if !strings.HasPrefix(domain, "*.") {
		_ = os.MkdirAll("/root/cert", 0o755)
		_ = os.WriteFile("/root/cert/.panel_domain", []byte(domain+"\n"), 0o644)
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

// certDisplayDomain 取一个"像域名"的名字用于展示
// Cloudflare 源站证书的 CN 是 "CloudFlare Origin Certificate"（含空格、不是域名），
// 直接用 CN 会把这个字符串当成域名显示，所以要回落到 SAN 里的第一个域名。
func certDisplayDomain(cert *x509.Certificate) string {
	cn := strings.TrimSpace(cert.Subject.CommonName)
	if cn != "" && strings.Contains(cn, ".") && !strings.Contains(cn, " ") && net.ParseIP(cn) == nil {
		return cn
	}
	if len(cert.DNSNames) > 0 {
		return cert.DNSNames[0]
	}
	// 兜底：安装时记录的域名
	if data, err := os.ReadFile("/root/cert/.panel_domain"); err == nil {
		if d := strings.TrimSpace(string(data)); d != "" {
			return d
		}
	}
	return cn
}

// isCfOriginCert 判断是否为 Cloudflare Origin CA 签发的证书
func isCfOriginCert(cert *x509.Certificate) bool {
	hay := strings.ToLower(cert.Issuer.CommonName + " " + strings.Join(cert.Issuer.Organization, " "))
	return strings.Contains(hay, "cloudflare")
}
// isPublicCA 粗判是否为公共可信 CA（浏览器默认信任的那些）
// 只用于界面提示，不影响任何证书功能。
func isPublicCA(cert *x509.Certificate) bool {
	hay := strings.ToLower(cert.Issuer.CommonName + " " + strings.Join(cert.Issuer.Organization, " "))
	for _, k := range []string{"let's encrypt", "zerossl", "buypass", "digicert", "sectigo",
		"globalsign", "google trust", "amazon", "geotrust", "comodo", "thawte", "rapidssl",
		"trustasia", "ssl.com", "certum", "entrust"} {
		if strings.Contains(hay, k) {
			return true
		}
	}
	return false
}

// IsTrustedByBrowser 当前绑定的面板证书是否被浏览器默认信任
// 只有公共 CA（Let's Encrypt / ZeroSSL / 商业 CA）且不是 Cloudflare 源站证书才算。
func (s *CertService) IsTrustedByBrowser() bool {
	var ss SettingService
	f, _ := ss.GetCertFile()
	if f == "" {
		return false
	}
	data, err := os.ReadFile(f)
	if err != nil {
		return false
	}
	cert, err := parseFirstCert(data)
	if err != nil {
		return false
	}
	if isCfOriginCert(cert) {
		return false
	}
	if cert.Issuer.String() == cert.Subject.String() {
		return false // 自签
	}
	return isPublicCA(cert)
}

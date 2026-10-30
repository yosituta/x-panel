package service

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

func makeTestCert(t *testing.T, dns string) (certPEM, keyPEM string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: dns},
		DNSNames:     []string{dns},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	return
}

func TestCertSaveRejectsMismatchedKey(t *testing.T) {
	certPEM, _ := makeTestCert(t, "example.com")
	_, otherKey := makeTestCert(t, "example.com")

	var s CertService
	if _, err := s.Save("example.com", certPEM, otherKey); err == nil {
		t.Fatal("证书与私钥不匹配时必须报错")
	}
}

func TestCertSaveRejectsWrongDomain(t *testing.T) {
	certPEM, keyPEM := makeTestCert(t, "example.com")
	var s CertService
	if _, err := s.Save("other.com", certPEM, keyPEM); err == nil {
		t.Fatal("证书不覆盖目标域名时必须报错")
	}
}

func TestCertSaveAcceptsValidPair(t *testing.T) {
	certPEM, keyPEM := makeTestCert(t, "panel.example.com")

	// 仅验证校验逻辑（真正落盘需要 root 与 /root/cert）
	if err := verifyKeyMatch(mustParse(t, certPEM), []byte(keyPEM)); err != nil {
		t.Fatalf("合法的一对应当通过：%v", err)
	}
	cert := mustParse(t, certPEM)
	if !certCoversHost(cert, "panel.example.com") {
		t.Error("域名匹配失败")
	}
	if certCoversHost(cert, "www.panel.example.com") {
		t.Error("不应匹配不存在的子域")
	}
}

func mustParse(t *testing.T, certPEM string) *x509.Certificate {
	t.Helper()
	c, err := parseFirstCert([]byte(certPEM))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

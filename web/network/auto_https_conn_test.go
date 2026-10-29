package network

import (
	"bufio"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// 明文 HTTP 打到 TLS 端口时，必须回一个浏览器能识别的 308 跳转
func TestAutoHttpsConnRedirectsPlainHTTP(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()

	go func() {
		c := NewAutoHttpsConn(server)
		buf := make([]byte, 1024)
		_, _ = c.Read(buf) // 触发探测
	}()

	_ = client.SetDeadline(time.Now().Add(3 * time.Second))
	_, err := client.Write([]byte("GET /xp-7f3a9c/ HTTP/1.1\r\nHost: us1.example.com:54321\r\n\r\n"))
	if err != nil {
		t.Fatalf("写入请求失败: %v", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(client), nil)
	if err != nil {
		t.Fatalf("读取响应失败（浏览器会认为打不开）: %v", err)
	}
	if resp.StatusCode != http.StatusPermanentRedirect {
		t.Fatalf("状态码应为 308，实际 %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Location"); got != "https://us1.example.com:54321/xp-7f3a9c/" {
		t.Fatalf("跳转地址不对: %s", got)
	}
	if !strings.HasPrefix(resp.Proto, "HTTP/1.") {
		t.Fatalf("状态行协议应为 HTTP/1.x（HTTP/0.0 会被浏览器判为非法）: %s", resp.Proto)
	}
}

// TLS ClientHello 不能被当成 HTTP 吞掉
func TestAutoHttpsConnPassesTLSBytes(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()

	done := make(chan []byte, 1)
	go func() {
		c := NewAutoHttpsConn(server)
		buf := make([]byte, 16)
		n, _ := c.Read(buf)
		done <- buf[:n]
	}()

	hello := []byte{0x16, 0x03, 0x01, 0x00, 0xa5, 0x01, 0x00, 0x00}
	_ = client.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := client.Write(hello); err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	select {
	case got := <-done:
		if len(got) == 0 || got[0] != 0x16 {
			t.Fatalf("TLS 首字节被破坏: %v", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("TLS 数据未被透传（握手会失败）")
	}
}

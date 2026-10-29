package network

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"sync"
)

type AutoHttpsConn struct {
	net.Conn

	firstBuf []byte
	bufStart int

	readRequestOnce sync.Once
}

func NewAutoHttpsConn(conn net.Conn) net.Conn {
	return &AutoHttpsConn{
		Conn: conn,
	}
}

// readRequest 读取首个数据包，判断是不是明文 HTTP 请求：
//   - 是（浏览器用 http:// 访问 TLS 端口）→ 回一个 308 跳转到 https:// 同地址
//   - 否（TLS ClientHello）→ 原样还回去，交给 TLS 层
//
// 注意：这里必须写完整的状态行（HTTP/1.1），否则浏览器收到 "HTTP/0.0" 会直接判为非法响应，
// 表现就是"页面打不开"而不会跳转。
func (c *AutoHttpsConn) readRequest() bool {
	c.firstBuf = make([]byte, 2048)
	n, err := c.Conn.Read(c.firstBuf)
	c.firstBuf = c.firstBuf[:n]
	if err != nil {
		return false
	}
	reader := bytes.NewReader(c.firstBuf)
	bufReader := bufio.NewReader(reader)
	request, err := http.ReadRequest(bufReader)
	if err != nil {
		return false
	}

	host := request.Host
	if host == "" {
		host = c.Conn.LocalAddr().String()
	}

	target := fmt.Sprintf("https://%s%s", host, request.RequestURI)

	resp := &http.Response{
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		StatusCode: http.StatusPermanentRedirect, // 308：保留原始方法与请求体
		Status:     "308 Permanent Redirect",
		Header: http.Header{
			"Location":      []string{target},
			"Connection":    []string{"close"},
			"Cache-Control": []string{"no-store"},
		},
		Close: true,
	}
	resp.Header.Set("Content-Length", "0")
	_ = resp.Write(c.Conn)
	_ = c.Close()
	c.firstBuf = nil
	return true
}

func (c *AutoHttpsConn) Read(buf []byte) (int, error) {
	c.readRequestOnce.Do(func() {
		c.readRequest()
	})

	if c.firstBuf != nil {
		n := copy(buf, c.firstBuf[c.bufStart:])
		c.bufStart += n
		if c.bufStart >= len(c.firstBuf) {
			c.firstBuf = nil
		}
		return n, nil
	}

	return c.Conn.Read(buf)
}

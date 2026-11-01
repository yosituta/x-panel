package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// 验证 ntfy / webhook 两个渠道的请求格式（用本地 httptest 冒充接收端）
func TestCertNotifySendChannels(t *testing.T) {
	type capture struct {
		path   string
		title  string
		tags   string
		body   string
		method string
	}
	var mu sync.Mutex
	got := make([]capture, 0, 2)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = append(got, capture{
			path:   r.URL.Path,
			title:  r.Header.Get("Title"),
			tags:   r.Header.Get("Tags"),
			body:   string(b),
			method: r.Method,
		})
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	svc := &CertNotifyService{}
	cfg := CertNotifyConfig{
		Channels:   []string{"ntfy", "webhook"},
		NtfyServer: srv.URL,
		NtfyTopic:  "test-topic",
		WebhookURL: srv.URL + "/hook",
	}

	if ok := svc.Send(cfg, "证书到期提醒", "剩余 7 天，请续期"); !ok {
		t.Fatal("Send 返回失败，两个渠道都应成功")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 {
		t.Fatalf("期望收到 2 个请求，实际 %d 个", len(got))
	}

	var ntfy, hook *capture
	for i := range got {
		if got[i].path == "/test-topic" {
			ntfy = &got[i]
		} else if got[i].path == "/hook" {
			hook = &got[i]
		}
	}
	if ntfy == nil {
		t.Fatal("ntfy 请求未到达 /test-topic")
	}
	if ntfy.method != "POST" {
		t.Errorf("ntfy 应为 POST，实际 %s", ntfy.method)
	}
	if ntfy.title != "证书到期提醒" {
		t.Errorf("ntfy Title 头不对: %q", ntfy.title)
	}
	if !strings.Contains(ntfy.body, "剩余 7 天") {
		t.Errorf("ntfy body 不对: %q", ntfy.body)
	}

	if hook == nil {
		t.Fatal("webhook 请求未到达 /hook")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(hook.body), &payload); err != nil {
		t.Fatalf("webhook body 不是合法 JSON: %v", err)
	}
	if payload["type"] != "cert-expiry" || payload["source"] != "x-panel" {
		t.Errorf("webhook 字段不对: %v", payload)
	}
	if payload["title"] != "证书到期提醒" {
		t.Errorf("webhook title 不对: %v", payload["title"])
	}
}

// 渠道缺参数时应报错而不是静默成功
func TestCertNotifyMissingConfig(t *testing.T) {
	svc := &CertNotifyService{}
	if err := svc.sendNtfy(CertNotifyConfig{NtfyServer: "https://ntfy.sh"}, "t", "x"); err == nil {
		t.Error("ntfy 缺 topic 时应报错")
	}
	if err := svc.sendWebhook(CertNotifyConfig{}, "t", "x"); err == nil {
		t.Error("webhook 缺地址时应报错")
	}
}

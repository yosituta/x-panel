package service

import (
	"strings"
	"testing"
	"time"
)

func TestAlertTrafficThreshold(t *testing.T) {
	var s AlertService
	cfg := AlertConfig{Enabled: true, TrafficPercent: 80, ExpireDays: 3}
	now := time.Now()

	// 79% 不应提醒
	u := ClientUsage{Email: "a@x", Inbound: "tls-01", Used: 79, Total: 100}
	if items := s.evaluate(u, now, cfg, nil); len(items) != 0 {
		t.Fatalf("79%% 不应提醒，实际 %d 条", len(items))
	}

	// 85% 提醒一次
	u.Used = 85
	items := s.evaluate(u, now, cfg, nil)
	if len(items) != 1 {
		t.Fatalf("85%% 应提醒 1 条，实际 %d 条", len(items))
	}
	if !strings.Contains(items[0].Message, "85.0%") {
		t.Errorf("消息里应包含百分比: %s", items[0].Message)
	}

	// 超出上限：流量提醒 + 用尽提醒
	u.Used = 120
	items = s.evaluate(u, now, cfg, nil)
	if len(items) != 2 {
		t.Fatalf("超量应提醒 2 条，实际 %d 条", len(items))
	}
	seen := map[string]bool{}
	for _, it := range items {
		if seen[it.Key] {
			t.Errorf("key 重复: %s", it.Key)
		}
		seen[it.Key] = true
	}
}

func TestAlertExpiry(t *testing.T) {
	var s AlertService
	cfg := AlertConfig{Enabled: true, TrafficPercent: 80, ExpireDays: 3}
	now := time.Now()

	// 还剩 2 天：应提醒
	u := ClientUsage{Email: "b@x", Inbound: "ss", ExpireTime: now.Add(48 * time.Hour).UnixMilli()}
	items := s.evaluate(u, now, cfg, nil)
	if len(items) != 1 {
		t.Fatalf("2 天后到期应提醒 1 条，实际 %d 条", len(items))
	}
	if !strings.Contains(items[0].Message, "还有 2 天") {
		t.Errorf("天数计算不对: %s", items[0].Message)
	}

	// 还剩 10 天：不提醒
	u.ExpireTime = now.Add(240 * time.Hour).UnixMilli()
	if items := s.evaluate(u, now, cfg, nil); len(items) != 0 {
		t.Fatalf("10 天后到期不应提醒，实际 %d 条", len(items))
	}

	// 已过期
	u.ExpireTime = now.Add(-time.Hour).UnixMilli()
	items = s.evaluate(u, now, cfg, nil)
	if len(items) != 1 || !strings.Contains(items[0].Message, "已到期") {
		t.Fatalf("过期应提醒，实际: %+v", items)
	}
}

func TestAlertUnlimited(t *testing.T) {
	var s AlertService
	cfg := AlertConfig{Enabled: true, TrafficPercent: 80, ExpireDays: 3}
	// 不限流量、不限时长：不产生任何提醒
	u := ClientUsage{Email: "c@x", Inbound: "tls", Used: 1 << 40, Total: 0, ExpireTime: 0}
	if items := s.evaluate(u, time.Now(), cfg, nil); len(items) != 0 {
		t.Fatalf("不限量不应提醒，实际 %d 条", len(items))
	}
}

func TestHumanBytes(t *testing.T) {
	cases := map[int64]string{
		512:              "512 B",
		1024:             "1.00 KB",
		10 * 1024 * 1024: "10.00 MB",
	}
	for in, want := range cases {
		if got := humanBytes(in); got != want {
			t.Errorf("humanBytes(%d) = %s, 期望 %s", in, got, want)
		}
	}
}

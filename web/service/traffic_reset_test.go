package service

import (
	"testing"
	"time"

	"x-ui/database/model"
)

// 这些用例只覆盖纯时间判断逻辑，不触碰数据库。

func TestScheduledAtDaily(t *testing.T) {
	s := &TrafficResetService{}
	now := time.Date(2026, 10, 23, 15, 30, 0, 0, time.Local)
	got, ok := s.scheduledAt(ResetPeriodDaily, 1, now)
	if !ok {
		t.Fatal("daily 应参与重置")
	}
	want := time.Date(2026, 10, 23, 0, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Fatalf("daily scheduledAt = %v, want %v", got, want)
	}
}

func TestScheduledAtWeekly(t *testing.T) {
	s := &TrafficResetService{}
	// 2026-10-23 是周五，本周一应为 2026-10-19
	now := time.Date(2026, 10, 23, 15, 30, 0, 0, time.Local)
	got, _ := s.scheduledAt(ResetPeriodWeekly, 1, now)
	want := time.Date(2026, 10, 19, 0, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Fatalf("weekly scheduledAt = %v, want %v", got, want)
	}

	// 周一当天，scheduledAt 应等于当天 00:00
	monday := time.Date(2026, 10, 19, 0, 0, 1, 0, time.Local)
	got, _ = s.scheduledAt(ResetPeriodWeekly, 1, monday)
	if !got.Equal(time.Date(2026, 10, 19, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("weekly(周一) scheduledAt = %v", got)
	}

	// 周日应归属上一周（周一为一周起点）
	sunday := time.Date(2026, 10, 25, 12, 0, 0, 0, time.Local)
	got, _ = s.scheduledAt(ResetPeriodWeekly, 1, sunday)
	if !got.Equal(time.Date(2026, 10, 19, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("weekly(周日) scheduledAt = %v, want 2026-10-19", got)
	}
}

func TestScheduledAtMonthly(t *testing.T) {
	s := &TrafficResetService{}

	now := time.Date(2026, 10, 23, 15, 30, 0, 0, time.Local)
	got, _ := s.scheduledAt(ResetPeriodMonthly, 12, now)
	if !got.Equal(time.Date(2026, 10, 12, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("monthly(12) = %v", got)
	}

	// 2 月 30 日不存在 -> 落到 2 月最后一天（2026 非闰年 -> 28 日）
	feb := time.Date(2026, 2, 15, 10, 0, 0, 0, time.Local)
	got, _ = s.scheduledAt(ResetPeriodMonthly, 30, feb)
	if !got.Equal(time.Date(2026, 2, 28, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("monthly(30) in Feb = %v, want 2026-02-28", got)
	}

	// 闰年 2 月
	feb2028 := time.Date(2028, 2, 10, 0, 0, 0, 0, time.Local)
	got, _ = s.scheduledAt(ResetPeriodMonthly, 30, feb2028)
	if !got.Equal(time.Date(2028, 2, 29, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("monthly(30) in Feb 2028 = %v, want 2028-02-29", got)
	}

	// 非法天数回落为 1 号
	got, _ = s.scheduledAt(ResetPeriodMonthly, 99, now)
	if !got.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("monthly(99) = %v, want day 1", got)
	}
}

func TestScheduledAtNever(t *testing.T) {
	s := &TrafficResetService{}
	now := time.Now()
	if _, ok := s.scheduledAt(ResetPeriodNever, 1, now); ok {
		t.Fatal("never 不应参与重置")
	}
	if _, ok := s.scheduledAt("", 1, now); ok {
		t.Fatal("空策略不应参与重置")
	}
}

func TestIsDueIdempotent(t *testing.T) {
	s := &TrafficResetService{}
	now := time.Date(2026, 10, 23, 15, 30, 0, 0, time.Local)

	in := &model.Inbound{Enable: true, ResetPeriod: ResetPeriodDaily}
	// 首次：从未重置过 -> 应到期
	if _, due := s.IsDue(in, now); !due {
		t.Fatal("首次应到期")
	}

	// 已在本周期重置过 -> 不应重复
	scheduled := time.Date(2026, 10, 23, 0, 0, 0, 0, time.Local)
	in.LastResetTime = scheduled.UnixMilli()
	if _, due := s.IsDue(in, now); due {
		t.Fatal("同一周期内不应重复重置")
	}

	// 上一天重置的 -> 今天应再次到期
	in.LastResetTime = time.Date(2026, 10, 22, 0, 0, 0, 0, time.Local).UnixMilli()
	if _, due := s.IsDue(in, now); !due {
		t.Fatal("跨天后应再次到期")
	}

	// 入站被禁用 -> 不重置
	in.Enable = false
	if _, due := s.IsDue(in, now); due {
		t.Fatal("禁用的入站不应重置")
	}
}

func TestIsDueBeforeScheduledMoment(t *testing.T) {
	s := &TrafficResetService{}
	// 每月 12 号，今天是 10 月 5 号 -> 本月重置时刻尚未到，不应重置
	in := &model.Inbound{Enable: true, ResetPeriod: ResetPeriodMonthly, ResetDay: 12}
	now := time.Date(2026, 10, 5, 9, 0, 0, 0, time.Local)
	if _, due := s.IsDue(in, now); due {
		t.Fatal("重置日未到不应重置")
	}
	// 同一入站，10 月 12 号当天 -> 应到期
	now = time.Date(2026, 10, 12, 0, 1, 0, 0, time.Local)
	if _, due := s.IsDue(in, now); !due {
		t.Fatal("重置日当天应到期")
	}
}

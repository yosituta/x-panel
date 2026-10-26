package service

import (
	"testing"
	"time"
)

func TestIsOnlineWindow(t *testing.T) {
	now := time.Now()
	window := 10 * time.Minute

	if !isOnline(now, window, now.Add(-1*time.Minute).UnixMilli()) {
		t.Error("1 分钟前活动应算在线")
	}
	if !isOnline(now, window, now.Add(-9*time.Minute).UnixMilli()) {
		t.Error("9 分钟前活动应算在线")
	}
	if isOnline(now, window, now.Add(-11*time.Minute).UnixMilli()) {
		t.Error("11 分钟前活动应算下线")
	}
	if isOnline(now, window, 0) {
		t.Error("从未活动应算下线")
	}
}

func TestOnlineWindowSettingBounds(t *testing.T) {
	var s PresenceService
	d := s.onlineWindow()
	if d < 2*time.Minute {
		t.Errorf("窗口不应小于 2 分钟，实际 %v", d)
	}
}

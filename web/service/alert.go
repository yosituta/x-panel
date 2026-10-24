package service

import (
	"fmt"
	"time"
)

// 流量/到期告警的纯逻辑部分，便于单测
type AlertConfig struct {
	Enabled        bool
	TrafficPercent int // 用量达到该百分比时提醒
	ExpireDays     int // 到期前 N 天提醒
}

// ClientUsage 单个客户端的用量快照
type ClientUsage struct {
	Email      string
	Inbound    string
	Used       int64
	Total      int64 // 0 表示不限
	ExpireTime int64 // 毫秒时间戳，0 表示不限
}

// AlertItem 一条待发提醒
type AlertItem struct {
	Key     string `json:"key"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit && exp < 5; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// evaluate 根据用量与已通知状态算出本次需要发送的提醒
// notified 为上一轮已通知的 key 集合，用于去重
func (s *AlertService) evaluate(c ClientUsage, now time.Time, cfg AlertConfig, notified map[string]bool) []AlertItem {
	items := make([]AlertItem, 0, 4)
	name := c.Email
	if name == "" {
		name = c.Inbound
	}

	if c.Total > 0 {
		percent := float64(c.Used) / float64(c.Total) * 100
		threshold := cfg.TrafficPercent
		if threshold <= 0 || threshold > 100 {
			threshold = 80
		}
		if percent >= float64(threshold) {
			key := c.Email + "|traffic" + fmt.Sprint(threshold)
			items = append(items, AlertItem{
				Key:   key,
				Level: "warning",
				Message: fmt.Sprintf("⚠️ 流量提醒：%s（%s）已用 %.1f%%（%s / %s）",
					name, c.Inbound, percent, humanBytes(c.Used), humanBytes(c.Total)),
			})
		}
		if percent >= 100 {
			key := c.Email + "|traffic100"
			items = append(items, AlertItem{
				Key:   key,
				Level: "exhausted",
				Message: fmt.Sprintf("⛔ 流量用尽：%s（%s）已超出上限（%s / %s），客户端已被停用",
					name, c.Inbound, humanBytes(c.Used), humanBytes(c.Total)),
			})
		}
	}

	if c.ExpireTime > 0 {
		days := cfg.ExpireDays
		if days <= 0 {
			days = 3
		}
		left := time.Until(time.UnixMilli(c.ExpireTime))
		// 向上取整到天，避免 2.4 天被当成 2 天
		daysLeft := int((left + 24*time.Hour - 1) / (24 * time.Hour))
		if left <= 0 {
			items = append(items, AlertItem{
				Key:     c.Email + "|expired",
				Level:   "expired",
				Message: fmt.Sprintf("⛔ 已到期：%s（%s）于 %s 到期", name, c.Inbound, time.UnixMilli(c.ExpireTime).Format("2006-01-02 15:04")),
			})
		} else if daysLeft <= days {
			items = append(items, AlertItem{
				Key:   fmt.Sprintf("%s|expire%d", c.Email, daysLeft),
				Level: "warning",
				Message: fmt.Sprintf("⏰ 到期提醒：%s（%s）还有 %d 天到期（%s）",
					name, c.Inbound, daysLeft, time.UnixMilli(c.ExpireTime).Format("2006-01-02")),
			})
		}
	}
	_ = now
	return items
}

package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"x-ui/database"
	"x-ui/database/model"
	"x-ui/logger"
)

// PresenceService 节点（客户端）上下线通知：
// 用 clientTraffic.lastOnline 判断在线，状态变化时推送 Telegram 管理员
type PresenceService struct{}

const (
	presenceStateKey = "presenceState"
	// 多久算“还在线”：超过这个窗口没有流量上报即视为下线
	defaultOnlineWindowMin = 10
)

// settingsReady 数据库未初始化时（例如单测环境）跳过读设置
func settingsReady() bool {
	return database.GetDB() != nil
}

func (s *PresenceService) onlineWindow() time.Duration {
	var ss SettingService
	min := defaultOnlineWindowMin
	if settingsReady() {
		if v, err := ss.getString("presenceWindowMinutes"); err == nil {
			if n, err2 := strconv.Atoi(strings.TrimSpace(v)); err2 == nil && n > 0 {
				min = n
			}
		}
	}
	if min < 2 {
		min = 2
	}
	return time.Duration(min) * time.Minute
}

func (s *PresenceService) enabled() bool {
	if !settingsReady() {
		return true
	}
	var ss SettingService
	v, err := ss.getString("presenceEnable")
	if err != nil {
		return true
	}
	return strings.EqualFold(v, "true")
}

func (s *PresenceService) loadState() map[string]bool {
	state := map[string]bool{}
	if !settingsReady() {
		return state
	}
	var ss SettingService
	raw, _ := ss.getString(presenceStateKey)
	if strings.TrimSpace(raw) == "" {
		return state
	}
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return map[string]bool{}
	}
	return state
}

func (s *PresenceService) saveState(state map[string]bool) {
	bs, err := json.Marshal(state)
	if err != nil {
		return
	}
	var ss SettingService
	if err := ss.setString(presenceStateKey, string(bs)); err != nil {
		logger.Warning("上下线状态保存失败:", err)
	}
}

// isOnline 最后一次活动时间是否在窗口内
func isOnline(now time.Time, window time.Duration, ts int64) bool {
	return ts > 0 && now.Sub(time.UnixMilli(ts)) <= window
}

// CheckAndNotify 比对状态并推送变化，返回通知条数
func (s *PresenceService) CheckAndNotify() (int, error) {
	if !s.enabled() {
		return 0, nil
	}

	db := database.GetDB()
	var inbounds []*model.Inbound
	if err := db.Model(&model.Inbound{}).
		Select("id, remark, enable, settings").
		Find(&inbounds).Error; err != nil {
		return 0, err
	}

	var inb InboundService
	lastOnline, err := inb.GetClientsLastOnline()
	if err != nil {
		return 0, err
	}

	oldState := s.loadState()
	newState := map[string]bool{}
	messages := make([]string, 0, 4)
	window := s.onlineWindow()
	now := time.Now()

	for _, ib := range inbounds {
		if !ib.Enable {
			continue
		}
		var cfg struct {
			Clients []model.Client `json:"clients"`
		}
		if err := json.Unmarshal([]byte(ib.Settings), &cfg); err != nil {
			continue
		}
		for _, c := range cfg.Clients {
			if !c.Enable || c.Email == "" {
				continue
			}
			ts := lastOnline[c.Email]
			online := isOnline(now, window, ts)
			newState[c.Email] = online

			prev, seen := oldState[c.Email]
			if !seen || prev == online {
				continue
			}
			if online {
				messages = append(messages, fmt.Sprintf("🟢 节点上线：%s（%s）\n最后活动：%s",
					ib.Remark, c.Email, now.Format("2006-01-02 15:04:05")))
			} else {
				messages = append(messages, fmt.Sprintf("🔴 节点下线：%s（%s）\n最后活动：%s",
					ib.Remark, c.Email, time.UnixMilli(ts).Format("2006-01-02 15:04:05")))
			}
		}
	}

	s.saveState(newState)

	// 首次运行只记录基线，不刷屏
	sent := 0
	if len(oldState) > 0 {
		for _, m := range messages {
			logger.Info("[上下线]", m)
			sent++
		}
		if sent > 0 {
			var tg Tgbot
			tg.SendMsgToTgbotAdmins(fmt.Sprintf("📡 节点状态变化（%d 条）\r\n\r\n%s",
				sent, strings.Join(messages, "\r\n\r\n")))
		}
	}
	return sent, nil
}

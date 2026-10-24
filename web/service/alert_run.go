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

// AlertService 流量与到期告警：定期扫描客户端用量，接近阈值时推送到 Telegram 管理员
type AlertService struct{}

const (
	alertStateKey     = "alertNotifyState"
	defaultAlertPct   = 80
	defaultAlertDays  = 3
)

// config 读取告警配置，取不到时使用默认值
func (s *AlertService) config() AlertConfig {
	var ss SettingService
	cfg := AlertConfig{Enabled: true, TrafficPercent: defaultAlertPct, ExpireDays: defaultAlertDays}
	if v, err := ss.getString("alertEnable"); err == nil {
		cfg.Enabled = strings.EqualFold(v, "true")
	}
	if v, err := ss.getString("alertTrafficPercent"); err == nil {
		if n, err2 := strconv.Atoi(v); err2 == nil && n > 0 && n <= 100 {
			cfg.TrafficPercent = n
		}
	}
	if v, err := ss.getString("alertExpireDays"); err == nil {
		if n, err2 := strconv.Atoi(v); err2 == nil && n > 0 {
			cfg.ExpireDays = n
		}
	}
	return cfg
}

func (s *AlertService) loadState() map[string]bool {
	var ss SettingService
	raw, err := ss.getString(alertStateKey)
	state := map[string]bool{}
	if err != nil || strings.TrimSpace(raw) == "" {
		return state
	}
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		logger.Warning("告警状态解析失败，将重新开始:", err)
		return map[string]bool{}
	}
	return state
}

func (s *AlertService) saveState(state map[string]bool) {
	bs, err := json.Marshal(state)
	if err != nil {
		return
	}
	var ss SettingService
	if err := ss.setString(alertStateKey, string(bs)); err != nil {
		logger.Warning("告警状态保存失败:", err)
	}
}

// CheckAndNotify 扫描全部入站客户端，返回本次发送的提醒条数
func (s *AlertService) CheckAndNotify() (int, error) {
	cfg := s.config()
	if !cfg.Enabled {
		return 0, nil
	}

	db := database.GetDB()
	var inbounds []*model.Inbound
	if err := db.Model(&model.Inbound{}).Preload("ClientStats").Find(&inbounds).Error; err != nil {
		return 0, err
	}

	oldState := s.loadState()
	newState := map[string]bool{}
	messages := make([]string, 0, 4)
	now := time.Now()

	for _, ib := range inbounds {
		if !ib.Enable {
			continue
		}
		var settings struct {
			Clients []model.Client `json:"clients"`
		}
		if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
			continue
		}
		for _, c := range settings.Clients {
			if !c.Enable {
				continue
			}
			if c.TotalGB <= 0 && c.ExpiryTime <= 0 {
				continue
			}
			usage := ClientUsage{
				Email:      c.Email,
				Inbound:    ib.Remark,
				Total:      c.TotalGB,
				ExpireTime: c.ExpiryTime,
			}
			for _, st := range ib.ClientStats {
				if st.Email == c.Email {
					usage.Used = st.Up + st.Down
				}
			}
			for _, item := range s.evaluate(usage, now, cfg, oldState) {
				newState[item.Key] = true
				if !oldState[item.Key] {
					messages = append(messages, item.Message)
					logger.Info("[告警]", item.Message)
				}
			}
		}
	}

	// 条件不再成立的记录会自动从 newState 中消失，下一轮若再次触发会重新提醒
	s.saveState(newState)

	if len(messages) == 0 {
		return 0, nil
	}
	var tg Tgbot
	tg.SendMsgToTgbotAdmins(fmt.Sprintf("🔔 面板告警（%d 条）\r\n\r\n%s",
		len(messages), strings.Join(messages, "\r\n\r\n")))
	return len(messages), nil
}

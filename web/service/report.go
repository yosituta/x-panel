package service

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"x-ui/database"
	"x-ui/database/model"
	"x-ui/logger"
)

// 【开源增强】TG 每日报告：内容可定制、时间可选（每天/每周/每月）
type ReportService struct{}

const reportLastSentKey = "reportLastSent"

type reportConfig struct {
	Enable   bool
	Interval string // daily / weekly / monthly
	Time     string // HH:MM
	Weekday  int    // 1=周一 ... 7=周日
	Day      int    // 1-31（monthly）
	Content  []string
}

func (s *ReportService) config() reportConfig {
	var ss SettingService
	cfg := reportConfig{Enable: false, Interval: "daily", Time: "09:00", Weekday: 1, Day: 1,
		Content: []string{"server", "traffic", "inbounds"}}
	get := func(k string) string {
		if !settingsReady() {
			return ""
		}
		v, err := ss.getString(k)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(v)
	}
	if v := get("reportEnable"); v != "" {
		cfg.Enable = strings.EqualFold(v, "true")
	}
	if v := get("reportInterval"); v != "" {
		cfg.Interval = v
	}
	if v := get("reportTime"); v != "" {
		cfg.Time = v
	}
	if v := get("reportWeekday"); v != "" {
		if n, err := parseInt(v); err == nil && n >= 1 && n <= 7 {
			cfg.Weekday = n
		}
	}
	if v := get("reportDay"); v != "" {
		if n, err := parseInt(v); err == nil && n >= 1 && n <= 31 {
			cfg.Day = n
		}
	}
	if v := get("reportContent"); v != "" {
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		if len(out) > 0 {
			cfg.Content = out
		}
	}
	return cfg
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

// due 判断当前时刻是否该发送
func reportDue(cfg reportConfig, now time.Time, lastSent string) bool {
	if !cfg.Enable {
		return false
	}
	hhmm := now.Format("15:04")
	if cfg.Time != "" && hhmm != cfg.Time {
		return false
	}
	key := now.Format("2006-01-02")
	switch cfg.Interval {
	case "weekly":
		if int(now.Weekday()) != cfg.Weekday%7 {
			return false
		}
	case "monthly":
		if now.Day() != cfg.Day {
			return false
		}
	}
	return lastSent != key
}

// BuildReport 组装报告正文
func (s *ReportService) BuildReport(cfg reportConfig) string {
	var b strings.Builder
	now := time.Now()
	b.WriteString(fmt.Sprintf("📊 面板报告 %s\r\n", now.Format("2006-01-02 15:04")))

	want := map[string]bool{}
	for _, c := range cfg.Content {
		want[c] = true
	}

	if want["server"] {
		var ss ServerService
		st := ss.GetStatus(&Status{})
		host, _ := hostName()
		b.WriteString(fmt.Sprintf("\r\n🖥 主机：%s\r\nCPU：%.1f%%　内存：%s / %s\r\n运行：%s\r\n",
			host,
			st.Cpu,
			humanBytes(int64(st.Mem.Current)),
			humanBytes(int64(st.Mem.Total)),
			(time.Duration(st.Uptime) * time.Second).Round(time.Minute).String()))
	}

	db := database.GetDB()
	inbounds := make([]*model.Inbound, 0, 8)
	if want["traffic"] || want["inbounds"] {
		db.Model(&model.Inbound{}).Preload("ClientStats").Find(&inbounds)
	}

	if want["traffic"] {
		var up, down int64
		top := make([]string, 0, 5)
		type cli struct {
			name string
			used int64
		}
		clients := make([]cli, 0, 32)
		for _, ib := range inbounds {
			up += ib.Up
			down += ib.Down
			for _, st := range ib.ClientStats {
				clients = append(clients, cli{fmt.Sprintf("%s(%s)", ib.Remark, st.Email), st.Up + st.Down})
			}
		}
		for i := 0; i < len(clients) && i < 5; i++ {
			for j := i + 1; j < len(clients); j++ {
				if clients[j].used > clients[i].used {
					clients[i], clients[j] = clients[j], clients[i]
				}
			}
		}
		for i := 0; i < len(clients) && i < 5; i++ {
			if clients[i].used > 0 {
				top = append(top, fmt.Sprintf("  %d. %s — %s", i+1, clients[i].name, humanBytes(clients[i].used)))
			}
		}
		b.WriteString(fmt.Sprintf("\r\n🚦 流量：↑%s ↓%s（合计 %s）\r\n",
			humanBytes(up), humanBytes(down), humanBytes(up+down)))
		if len(top) > 0 {
			b.WriteString("用量前五：\r\n" + strings.Join(top, "\r\n") + "\r\n")
		}
	}

	if want["inbounds"] {
		enabled, disabled, expiring := 0, 0, 0
		deadline := now.Add(72 * time.Hour).UnixMilli()
		for _, ib := range inbounds {
			if ib.Enable {
				enabled++
			} else {
				disabled++
			}
			if ib.ExpiryTime > 0 && ib.ExpiryTime < deadline {
				expiring++
			}
		}
		b.WriteString(fmt.Sprintf("\r\n📡 入站：启用 %d / 停用 %d，72 小时内到期 %d 条\r\n", enabled, disabled, expiring))
	}

	if want["clients"] {
		var inb InboundService
		last, _ := inb.GetClientsLastOnline()
		online := 0
		for _, ts := range last {
			if isOnline(now, 10*time.Minute, ts) {
				online++
			}
		}
		b.WriteString(fmt.Sprintf("\r\n👥 近 10 分钟在线客户端：%d 个\r\n", online))
	}
	return b.String()
}

// CheckAndSend 定时任务入口：到点则发送
func (s *ReportService) CheckAndSend() (bool, error) {
	cfg := s.config()
	if !cfg.Enable || !settingsReady() {
		return false, nil
	}
	var ss SettingService
	last, _ := ss.getString(reportLastSentKey)
	now := time.Now()
	if !reportDue(cfg, now, strings.TrimSpace(last)) {
		return false, nil
	}

	text := s.BuildReport(cfg)
	if strings.TrimSpace(text) == "" {
		return false, nil
	}
	var tg Tgbot
	tg.SendMsgToTgbotAdmins(text)
	if err := ss.setString(reportLastSentKey, now.Format("2006-01-02")); err != nil {
		logger.Warning("记录报告发送时间失败:", err)
	}
	logger.Info("[报告] 已发送定时报告，内容项:", strings.Join(cfg.Content, ","))
	return true, nil
}

// PreviewReport 供面板/接口预览
func (s *ReportService) PreviewReport() string {
	return s.BuildReport(s.config())
}

func hostName() (string, error) {
	h, err := osHostname()
	return h, err
}

// 便于测试替换
var osHostname = func() (string, error) { return os.Hostname() }

// 输出 JSON 配置，供前端读取
func (s *ReportService) ConfigJSON() string {
	bs, _ := json.Marshal(s.config())
	return string(bs)
}

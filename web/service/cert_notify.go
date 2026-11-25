package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"x-ui/logger"
)

// 【开源增强】证书到期提醒
//
// 背景：acme standalone 续期需要独占 80 端口，机器上常有 nginx/宝塔占着，
// 自动续期会失败。所以除了"能自动就自动"，还要在证书快到期前提前提醒，
// 让人手动腾出 80 端口去续。
//
// 支持多选渠道：Telegram / ntfy / Webhook；提醒节点可自定义（如 15、7、3 天）。
type CertNotifyConfig struct {
	Enable     bool     `json:"enable"`
	Channels   []string `json:"channels"` // tg / ntfy / webhook，可多选
	Days       []int    `json:"days"`     // 例如 [15,7,3]
	NtfyServer string   `json:"ntfyServer"`
	NtfyTopic  string   `json:"ntfyTopic"`
	WebhookURL string   `json:"webhookUrl"`
}

// CertNotifyStatus 给前端展示用
type CertNotifyStatus struct {
	Config     CertNotifyConfig `json:"config"`
	Domain     string           `json:"domain"`
	Issuer     string           `json:"issuer"`
	NotAfter   string           `json:"notAfter"`
	DaysLeft   int              `json:"daysLeft"`
	CfOrigin   bool             `json:"cfOrigin"`
	SelfSigned bool             `json:"selfSigned"`
	LastSent   map[string]string `json:"lastSent"` // 天数值 -> 已发送日期
	Port80Busy bool             `json:"port80Busy"`
}

type CertNotifyService struct{}

const (
	keyNotifyEnable   = "certNotifyEnable"
	keyNotifyChannels = "certNotifyChannels"
	keyNotifyDays     = "certNotifyDays"
	keyNotifyNtfyURL  = "certNotifyNtfyServer"
	keyNotifyNtfyTop  = "certNotifyNtfyTopic"
	keyNotifyWebhook  = "certNotifyWebhook"
	keyNotifyState    = "certNotifyState"
)

func (s *CertNotifyService) Load() CertNotifyConfig {
	var ss SettingService
	cfg := CertNotifyConfig{
		NtfyServer: "https://ntfy.sh",
		Days:       []int{15, 7, 3},
		Channels:   []string{}, // 必须是空切片而不是 nil：JSON 里 null 会让前端 .includes 抛错
	}
	if v, err := ss.getString(keyNotifyEnable); err == nil {
		cfg.Enable = v == "true" || v == "1"
	}
	if v, err := ss.getString(keyNotifyChannels); err == nil && strings.TrimSpace(v) != "" {
		for _, c := range strings.Split(v, ",") {
			if c = strings.TrimSpace(c); c != "" {
				cfg.Channels = append(cfg.Channels, c)
			}
		}
	}
	if v, err := ss.getString(keyNotifyDays); err == nil && strings.TrimSpace(v) != "" {
		var days []int
		for _, p := range strings.Split(v, ",") {
			if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && n > 0 {
				days = append(days, n)
			}
		}
		if len(days) > 0 {
			sort.Sort(sort.Reverse(sort.IntSlice(days)))
			cfg.Days = days
		}
	}
	if v, err := ss.getString(keyNotifyNtfyURL); err == nil && v != "" {
		cfg.NtfyServer = v
	}
	if v, err := ss.getString(keyNotifyNtfyTop); err == nil {
		cfg.NtfyTopic = v
	}
	if v, err := ss.getString(keyNotifyWebhook); err == nil {
		cfg.WebhookURL = v
	}
	return cfg
}

func (s *CertNotifyService) Save(cfg CertNotifyConfig) error {
	var ss SettingService
	if err := ss.setString(keyNotifyEnable, boolStr(cfg.Enable)); err != nil {
		return err
	}
	if err := ss.setString(keyNotifyChannels, strings.Join(cfg.Channels, ",")); err != nil {
		return err
	}
	days := make([]string, 0, len(cfg.Days))
	for _, d := range cfg.Days {
		if d > 0 {
			days = append(days, strconv.Itoa(d))
		}
	}
	if err := ss.setString(keyNotifyDays, strings.Join(days, ",")); err != nil {
		return err
	}
	if err := ss.setString(keyNotifyNtfyURL, strings.TrimSpace(cfg.NtfyServer)); err != nil {
		return err
	}
	if err := ss.setString(keyNotifyNtfyTop, strings.TrimSpace(cfg.NtfyTopic)); err != nil {
		return err
	}
	return ss.setString(keyNotifyWebhook, strings.TrimSpace(cfg.WebhookURL))
}

func (s *CertNotifyService) Status() (*CertNotifyStatus, error) {
	cfg := s.Load()
	if cfg.Channels == nil {
		cfg.Channels = []string{}
	}
	if cfg.Days == nil {
		cfg.Days = []int{15, 7, 3}
	}
	out := &CertNotifyStatus{Config: cfg, LastSent: s.loadState()}
	if out.LastSent == nil {
		out.LastSent = map[string]string{}
	}
	var cs CertService
	info, err := cs.Status()
	if err != nil {
		return out, err
	}
	out.Domain = info.Domain
	out.Issuer = info.Issuer
	out.NotAfter = info.NotAfter
	out.DaysLeft = info.Days
	out.CfOrigin = info.CfOrigin
	out.SelfSigned = info.SelfSigned
	out.Port80Busy = port80InUse()
	return out, nil
}

// Check 由定时任务调用：到期天数命中设置的提醒点就推送一次（同一天只推一次）
func (s *CertNotifyService) Check() (int, error) {
	cfg := s.Load()
	if !cfg.Enable || len(cfg.Channels) == 0 || len(cfg.Days) == 0 {
		return 0, nil
	}
	var cs CertService
	info, err := cs.Status()
	if err != nil || info.CertFile == "" {
		return 0, err
	}

	state := s.loadState()
	today := time.Now().Format("2006-01-02")
	sent := 0
	for _, d := range cfg.Days {
		// 剩余天数已经到（或低于）提醒点，且还没在这个周期内推过
		if info.Days > d || info.Days < 0 {
			continue
		}
		key := fmt.Sprintf("%s|%s|%d", info.Domain, info.NotAfter, d)
		if state[key] == today {
			continue
		}
		text := s.message(info, d)
		if ok := s.Send(cfg, "证书到期提醒", text); ok {
			state[key] = today
			sent++
			logger.Info("[证书提醒] 已推送：", info.Domain, "剩余", info.Days, "天（提醒点", d, "天）")
		}
	}
	s.saveState(state)
	return sent, nil
}

func (s *CertNotifyService) message(info *CertInfo, at int) string {
	lines := []string{
		fmt.Sprintf("⚠️ 面板证书即将到期"),
		fmt.Sprintf("域名：%s", info.Domain),
		fmt.Sprintf("到期：%s（剩余 %d 天）", info.NotAfter, info.Days),
		fmt.Sprintf("颁发者：%s", info.Issuer),
	}
	if port80InUse() {
		lines = append(lines, "提示：当前 80 端口被占用（nginx/宝塔），续期前请先 systemctl stop nginx，签完再 systemctl start nginx")
	}
	lines = append(lines, "续期：x-ui → 18 → 3（强制更新）或 18 → 1（重新签发）")
	_ = at
	return strings.Join(lines, "\n")
}

// SendTest 给前端「发测试」按钮用
func (s *CertNotifyService) SendTest() (string, error) {
	cfg := s.Load()
	if len(cfg.Channels) == 0 {
		return "", fmt.Errorf("请先勾选至少一个推送渠道")
	}
	var cs CertService
	info, _ := cs.Status()
	text := "🔔 这是一条测试消息：X-Panel 证书到期提醒已配置成功。"
	if info != nil && info.Domain != "" {
		text = fmt.Sprintf("%s\n当前证书：%s（剩余 %d 天）", text, info.Domain, info.Days)
	}
	if ok, errs := s.SendDetailed(cfg, "X-Panel 测试", text); !ok {
		if len(errs) > 0 {
			return "", fmt.Errorf("推送失败 → %s", strings.Join(errs, "；"))
		}
		return "", fmt.Errorf("推送失败：没有可用的渠道")
	}
	return "已发送，请检查接收端", nil
}

// Send 按配置的渠道逐个发送，任一成功即算成功
func (s *CertNotifyService) Send(cfg CertNotifyConfig, title, text string) bool {
	ok, _ := s.SendDetailed(cfg, title, text)
	return ok
}

// SendDetailed 同 Send，但把每个渠道的失败原因带出来（前端「发送测试」直接显示给用户）
func (s *CertNotifyService) SendDetailed(cfg CertNotifyConfig, title, text string) (bool, []string) {
	ok := false
	var errs []string
	for _, ch := range cfg.Channels {
		name := strings.ToLower(strings.TrimSpace(ch))
		var err error
		switch name {
		case "tg", "telegram":
			name = "Telegram"
			err = s.sendTG(text)
		case "ntfy":
			err = s.sendNtfy(cfg, title, text)
		case "webhook":
			err = s.sendWebhook(cfg, title, text)
		default:
			continue
		}
		if err != nil {
			logger.Warningf("[证书提醒] %s 推送失败: %v", name, err)
			errs = append(errs, fmt.Sprintf("%s：%v", name, err))
		} else {
			ok = true
			logger.Infof("[证书提醒] %s 推送成功", name)
		}
	}
	return ok, errs
}

func (s *CertNotifyService) sendTG(text string) error {
	var ss SettingService
	token, _ := ss.GetTgBotToken()
	chatIds, _ := ss.GetTgBotChatId()
	apiServer, _ := ss.GetTgBotAPIServer()
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("未配置 Bot Token（面板设置 → Telegram机器人配置）")
	}
	chatIds = strings.TrimSpace(chatIds)
	if chatIds == "" {
		return fmt.Errorf("未配置 Chat ID（面板设置 → Telegram机器人配置），可先给机器人发个 /start 再用 @userinfobot 查 ID")
	}
	base := strings.TrimRight(strings.TrimSpace(apiServer), "/")
	if base == "" {
		base = "https://api.telegram.org"
	}
	if !strings.HasPrefix(base, "http") {
		base = "https://" + base
	}
	client := &http.Client{Timeout: 12 * time.Second}
	var errs []string
	for _, id := range strings.Split(chatIds, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		body, _ := json.Marshal(map[string]any{"chat_id": id, "text": text})
		resp, err := client.Post(base+"/bot"+token+"/sendMessage", "application/json", bytes.NewReader(body))
		if err != nil {
			errs = append(errs, fmt.Sprintf("连不上 %s（服务器无法访问 Telegram，或被墙/代理配置错误）: %v", base, err))
			continue
		}
		rb, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			errs = append(errs, fmt.Sprintf("chat_id=%s %s", id, telegramErr(resp.StatusCode, rb)))
			continue
		}
		return nil
	}
	if len(errs) == 0 {
		return fmt.Errorf("没有可用的 chat_id")
	}
	return fmt.Errorf("%s", strings.Join(errs, "；"))
}

// telegramErr 把 Telegram API 的错误翻译成可操作的提示
func telegramErr(status int, body []byte) string {
	var payload struct {
		Description string `json:"description"`
	}
	_ = json.Unmarshal(body, &payload)
	desc := payload.Description
	low := strings.ToLower(desc)
	switch {
	case status == 401 || strings.Contains(low, "unauthorized"):
		return "Bot Token 不对（401 Unauthorized）"
	case strings.Contains(low, "chat not found"):
		return "Chat ID 不存在（chat not found）：私聊要先给机器人发 /start；群组必须是 -100 开头的负数，且机器人已被拉进群"
	case strings.Contains(low, "bot was blocked"):
		return "机器人被你拉黑了（bot was blocked）：先在 Telegram 里解除屏蔽"
	case strings.Contains(low, "chat_id is empty"):
		return "Chat ID 为空"
	case status == 429:
		return "Telegram 限流（429），稍后再试"
	case desc != "":
		return fmt.Sprintf("Telegram 返回 %d：%s", status, desc)
	default:
		return fmt.Sprintf("Telegram 返回 %d", status)
	}
}

func (s *CertNotifyService) sendNtfy(cfg CertNotifyConfig, title, text string) error {
	if cfg.NtfyTopic == "" {
		return fmt.Errorf("ntfy topic 未填写")
	}
	server := strings.TrimRight(cfg.NtfyServer, "/")
	if server == "" {
		server = "https://ntfy.sh"
	}
	req, err := http.NewRequest("POST", server+"/"+cfg.NtfyTopic, bytes.NewBufferString(text))
	if err != nil {
		return err
	}
	req.Header.Set("Title", title)
	req.Header.Set("Tags", "warning")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy 返回 %d", resp.StatusCode)
	}
	return nil
}

func (s *CertNotifyService) sendWebhook(cfg CertNotifyConfig, title, text string) error {
	if cfg.WebhookURL == "" {
		return fmt.Errorf("Webhook 地址未填写")
	}
	payload := map[string]any{
		"title":    title,
		"text":     text,
		"source":   "x-panel",
		"type":     "cert-expiry",
		"sentAt":   time.Now().Format(time.RFC3339),
	}
	bs, _ := json.Marshal(payload)
	resp, err := http.Post(cfg.WebhookURL, "application/json", bytes.NewReader(bs))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("Webhook 返回 %d", resp.StatusCode)
	}
	return nil
}

func (s *CertNotifyService) loadState() map[string]string {
	state := map[string]string{}
	var ss SettingService
	if v, err := ss.getString(keyNotifyState); err == nil && v != "" {
		_ = json.Unmarshal([]byte(v), &state)
	}
	return state
}

func (s *CertNotifyService) saveState(state map[string]string) {
	bs, _ := json.Marshal(state)
	var ss SettingService
	if err := ss.setString(keyNotifyState, string(bs)); err != nil {
		logger.Warning("[证书提醒] 状态保存失败:", err)
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// port80InUse 80 端口是否被占用（续期会失败的最常见原因）
func port80InUse() bool {
	ln, err := net.Listen("tcp", ":80")
	if err != nil {
		return true
	}
	_ = ln.Close()
	return false
}

package controller

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"x-ui/web/service"
)

// 【开源增强】证书到期提醒：面板内配置渠道与提醒节点
type CertNotifyController struct {
	BaseController
	svc service.CertNotifyService
}

func NewCertNotifyController(g *gin.RouterGroup) *CertNotifyController {
	a := &CertNotifyController{}
	a.initRouter(g)
	return a
}

func (a *CertNotifyController) initRouter(g *gin.RouterGroup) {
	r := g.Group("/panel/api/certnotify")
	r.Use(a.checkLogin)

	r.POST("/status", a.status)
	r.POST("/save", a.save)
	r.POST("/test", a.test)
}

func (a *CertNotifyController) status(c *gin.Context) {
	st, err := a.svc.Status()
	jsonObj(c, st, err)
}

// 前端以逗号分隔的字符串提交，比表单数组简单可靠
type certNotifyForm struct {
	Enable     bool   `form:"enable" json:"enable"`
	Channels   string `form:"channels" json:"channels"`
	Days       string `form:"days" json:"days"`
	NtfyServer string `form:"ntfyServer" json:"ntfyServer"`
	NtfyTopic  string `form:"ntfyTopic" json:"ntfyTopic"`
	WebhookURL string `form:"webhookUrl" json:"webhookUrl"`
}

func splitCSV(v string) []string {
	out := make([]string, 0, 3)
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (a *CertNotifyController) save(c *gin.Context) {
	var form certNotifyForm
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, "参数错误", err)
		return
	}
	days := make([]int, 0, 3)
	for _, p := range splitCSV(form.Days) {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			days = append(days, n)
		}
	}
	if len(days) == 0 {
		days = []int{15, 7, 3}
	}
	cfg := service.CertNotifyConfig{
		Enable:     form.Enable,
		Channels:   splitCSV(form.Channels),
		Days:       days,
		NtfyServer: form.NtfyServer,
		NtfyTopic:  form.NtfyTopic,
		WebhookURL: form.WebhookURL,
	}
	if err := a.svc.Save(cfg); err != nil {
		jsonMsg(c, "保存失败", err)
		return
	}
	jsonMsg(c, "证书提醒配置已保存", nil)
}

func (a *CertNotifyController) test(c *gin.Context) {
	msg, err := a.svc.SendTest()
	jsonMsg(c, msg, err)
}

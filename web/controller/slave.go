package controller

import (
	"net/http"
	"os"
	"time"

	"x-ui/config"
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

// 【开源增强】被控端 API：主控面板凭 slaveKey 调用，不需要登录会话
type SlaveController struct {
	BaseController

	settingService service.SettingService
	xrayService    service.XrayService
	panelService   service.RemotePanelService
}

func NewSlaveController(g *gin.RouterGroup) *SlaveController {
	a := &SlaveController{}
	a.initRouter(g)
	return a
}

func (a *SlaveController) initRouter(g *gin.RouterGroup) {
	s := g.Group("/panel/api/slave")
	s.Use(a.authByKey)

	s.GET("/status", a.status)
	s.GET("/links", a.links)
	s.POST("/restartXray", a.restartXray)
}

// authByKey 校验请求头里的 X-Slave-Key
func (a *SlaveController) authByKey(c *gin.Context) {
	key := c.GetHeader("X-Slave-Key")
	if key == "" {
		key = c.Query("key")
	}
	if !a.settingService.CheckSlaveKey(key) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "msg": "被控端未配置 slaveKey 或密钥不匹配"})
		return
	}
	c.Next()
}

// status 返回被控端概况
func (a *SlaveController) status(c *gin.Context) {
	db := database.GetDB()
	var total, enabled int64
	db.Model(&model.Inbound{}).Count(&total)
	db.Model(&model.Inbound{}).Where("enable = ?", true).Count(&enabled)

	host, _ := os.Hostname()
	jsonObj(c, gin.H{
		"hostname":     host,
		"version":      config.GetVersion(),
		"xrayRunning":  a.xrayService.IsXrayRunning(),
		"inbounds":     total,
		"enabled":      enabled,
		"timestamp":     time.Now().UnixMilli(),
	}, nil)
}

// links 返回被控端全部节点链接
func (a *SlaveController) links(c *gin.Context) {
	var sub service.SubscribeService
	var tg service.Tgbot
	host := tg.ResolveLinkHost()
	nodes, err := sub.AllNodes(host)
	if err != nil {
		jsonMsg(c, "读取节点失败", err)
		return
	}
	lines := make([]string, 0, len(nodes))
	for _, n := range nodes {
		lines = append(lines, "【"+n.Remark+"】\r\n"+n.URI())
	}
	text := ""
	for i, l := range lines {
		if i > 0 {
			text += "\r\n\r\n"
		}
		text += l
	}
	jsonObj(c, gin.H{"count": len(lines), "text": text}, nil)
}

// restartXray 让被控端重启 xray
func (a *SlaveController) restartXray(c *gin.Context) {
	if err := a.xrayService.RestartXray(true); err != nil {
		jsonMsg(c, "重启 xray 失败", err)
		return
	}
	jsonObj(c, gin.H{"restarted": true}, nil)
}

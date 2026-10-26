package controller

import (
	"net/http"

	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

// 【开源增强】主控端：管理其他面板（需登录）
type PanelController struct {
	BaseController

	panelService   service.RemotePanelService
	settingService service.SettingService
}

func NewPanelController(g *gin.RouterGroup) *PanelController {
	a := &PanelController{}
	a.initRouter(g)
	return a
}

func (a *PanelController) initRouter(g *gin.RouterGroup) {
	// 登录后可访问
	r := g.Group("/panel/api/panels")
	r.Use(a.checkLogin)

	r.GET("/list", a.list)
	r.POST("/add", a.add)
	r.POST("/del", a.del)
	r.GET("/del", a.del) // 兼容浏览器直接访问
	r.GET("/status", a.status)
	r.GET("/links", a.links)
	r.POST("/restartXray", a.restartXray)
}

func (a *PanelController) list(c *gin.Context) {
	items, err := a.panelService.List()
	if err != nil {
		jsonMsg(c, "读取面板列表失败", err)
		return
	}
	key, _ := a.settingService.EnsureSlaveKey()
	jsonObj(c, gin.H{"panels": items, "slaveKey": key}, nil)
}

func (a *PanelController) add(c *gin.Context) {
	var p service.RemotePanel
	if err := c.ShouldBindJSON(&p); err != nil {
		jsonMsg(c, "参数错误", err)
		return
	}
	if err := a.panelService.Add(p); err != nil {
		jsonMsg(c, "保存失败", err)
		return
	}
	jsonObj(c, gin.H{"saved": p.Name}, nil)
}

func (a *PanelController) del(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		var body struct {
			Name string `json:"name"`
		}
		_ = c.ShouldBindJSON(&body)
		name = body.Name
	}
	if err := a.panelService.Remove(name); err != nil {
		jsonMsg(c, "删除失败", err)
		return
	}
	jsonObj(c, gin.H{"removed": name}, nil)
}

func (a *PanelController) status(c *gin.Context) {
	name := c.Query("name")
	p, err := a.panelService.Get(name)
	if err != nil {
		jsonMsg(c, "面板不存在", err)
		return
	}
	st, err := p.Status()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "msg": "连接失败: " + err.Error(), "obj": gin.H{"online": false}})
		return
	}
	st["online"] = true
	jsonObj(c, st, nil)
}

func (a *PanelController) links(c *gin.Context) {
	name := c.Query("name")
	p, err := a.panelService.Get(name)
	if err != nil {
		jsonMsg(c, "面板不存在", err)
		return
	}
	text, err := p.Links()
	if err != nil {
		jsonMsg(c, "拉取链接失败", err)
		return
	}
	jsonObj(c, gin.H{"text": text}, nil)
}

func (a *PanelController) restartXray(c *gin.Context) {
	name := c.Query("name")
	p, err := a.panelService.Get(name)
	if err != nil {
		jsonMsg(c, "面板不存在", err)
		return
	}
	if err := p.RestartXray(); err != nil {
		jsonMsg(c, "重启失败", err)
		return
	}
	jsonObj(c, gin.H{"restarted": name}, nil)
}

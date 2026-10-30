package controller

import (
	"time"

	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

// 【开源增强】面板证书：网页直接粘贴 / 上传，不需要 SSH，也不需要 Cloudflare API Key
type CertController struct {
	BaseController
	certService  service.CertService
	panelService service.PanelService
}

func NewCertController(g *gin.RouterGroup) *CertController {
	a := &CertController{}
	a.initRouter(g)
	return a
}

func (a *CertController) initRouter(g *gin.RouterGroup) {
	r := g.Group("/panel/api/cert")
	r.Use(a.checkLogin)

	r.POST("/status", a.status)
	r.POST("/save", a.save)
	r.POST("/clear", a.clear)
}

func (a *CertController) status(c *gin.Context) {
	info, err := a.certService.Status()
	if err != nil {
		jsonMsg(c, "读取证书状态失败", err)
		return
	}
	jsonObj(c, info, nil)
}

type certSaveForm struct {
	Domain string `json:"domain" form:"domain"`
	Cert   string `json:"cert" form:"cert"`
	Key    string `json:"key" form:"key"`
}

func (a *CertController) save(c *gin.Context) {
	var form certSaveForm
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, "参数错误", err)
		return
	}
	info, err := a.certService.Save(form.Domain, form.Cert, form.Key)
	if err != nil {
		jsonMsg(c, "保存证书失败", err)
		return
	}
	// 延迟重启让响应先返回
	go func() {
		time.Sleep(2 * time.Second)
		_ = a.panelService.RestartPanel(time.Second)
	}()
	jsonObj(c, info, nil)
}

func (a *CertController) clear(c *gin.Context) {
	if err := a.certService.Clear(); err != nil {
		jsonMsg(c, "清除证书失败", err)
		return
	}
	go func() {
		time.Sleep(2 * time.Second)
		_ = a.panelService.RestartPanel(time.Second)
	}()
	jsonMsg(c, "证书已清除，面板恢复为无证书模式（仅监听 127.0.0.1）", nil)
}

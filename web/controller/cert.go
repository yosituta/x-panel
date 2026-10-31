package controller

import (

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
	// 不自动重启：保存只写配置（用户要求重启由自己点「重启面板」触发）。
	// 之前这里 2 秒后自动重启，会和用户随后点的手动重启撞车，前端表现为 Network Error。
	jsonObj(c, info, nil)
}

func (a *CertController) clear(c *gin.Context) {
	if err := a.certService.Clear(); err != nil {
		jsonMsg(c, "清除证书失败", err)
		return
	}
	jsonMsg(c, "证书已清除。点「重启面板」后生效（当前连接仍用旧证书）", nil)
}

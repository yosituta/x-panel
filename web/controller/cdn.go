package controller

import (
	"net"
	"strings"

	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

// 【开源增强】CDN 域名：面板里查看 / 绑定 / 修改 / 清除
type CdnController struct {
	BaseController
	settingService service.SettingService
}

func NewCdnController(g *gin.RouterGroup) *CdnController {
	a := &CdnController{}
	a.initRouter(g)
	return a
}

func (a *CdnController) initRouter(g *gin.RouterGroup) {
	r := g.Group("/panel/api/cdn")
	r.Use(a.checkLogin)

	r.POST("/status", a.status)
	r.POST("/save", a.save)
	r.POST("/clear", a.clear)
}

type cdnStatus struct {
	Domain   string   `json:"domain"`
	Resolved bool     `json:"resolved"`
	IsCF     bool     `json:"isCf"`
	IPs      []string `json:"ips"`
	Auto     string   `json:"auto"` // 未手动设置时，自动嗅探到的域名
}

func (a *CdnController) buildStatus(domain string) cdnStatus {
	st := cdnStatus{Domain: domain}
	if domain == "" {
		var settingService service.SettingService
		certFile, _ := settingService.GetCertFile()
		_ = certFile
		return st
	}
	ips, err := net.LookupHost(domain)
	if err == nil {
		st.Resolved = true
		st.IPs = ips
	}
	st.IsCF = service.IsBehindCloudflare(domain)
	return st
}

func (a *CdnController) status(c *gin.Context) {
	domain, _ := a.settingService.GetCdnDomain()
	st := a.buildStatus(strings.TrimSpace(domain))
	jsonObj(c, st, nil)
}

type cdnSaveForm struct {
	Domain string `json:"domain" form:"domain"`
}

func (a *CdnController) save(c *gin.Context) {
	var form cdnSaveForm
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, "参数错误", err)
		return
	}
	domain := strings.TrimSpace(form.Domain)
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimSuffix(domain, "/")
	if domain == "" {
		jsonMsg(c, "域名不能为空", nil)
		return
	}
	if strings.ContainsAny(domain, " :/") {
		jsonMsg(c, "域名不要带端口或路径，只填主机名，例如 cdn.example.com", nil)
		return
	}
	if err := a.settingService.SetCdnDomain(domain); err != nil {
		jsonMsg(c, "保存失败", err)
		return
	}
	jsonObj(c, a.buildStatus(domain), nil)
}

func (a *CdnController) clear(c *gin.Context) {
	if err := a.settingService.SetCdnDomain(""); err != nil {
		jsonMsg(c, "清除失败", err)
		return
	}
	jsonMsg(c, "已清除 CDN 域名，节点将按直连生成", nil)
}

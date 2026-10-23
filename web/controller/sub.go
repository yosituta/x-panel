package controller

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

// 订阅控制器：对外提供 /sub/:subid，按 target 渲染成不同格式
type SubController struct {
	BaseController

	subService service.SubscribeService
}

func NewSubController(g *gin.RouterGroup) *SubController {
	a := &SubController{}
	a.initRouter(g)
	return a
}

func (a *SubController) initRouter(g *gin.RouterGroup) {
	g.GET("/sub/:subid", a.subscription)
}

func (a *SubController) subscription(c *gin.Context) {
	subID := strings.TrimSpace(c.Param("subid"))
	target := strings.ToLower(strings.TrimSpace(c.Query("target")))
	// 允许用后缀指定格式：/sub/xxx.yaml / xxx.json / xxx.conf
	for suffix, t := range map[string]string{".yaml": "clash", ".yml": "clash", ".json": "singbox", ".conf": "surge"} {
		if strings.HasSuffix(subID, suffix) {
			subID = strings.TrimSuffix(subID, suffix)
			if target == "" {
				target = t
			}
		}
	}

	host := strings.TrimSpace(c.Query("host"))
	if host == "" {
		host = c.Request.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
	}

	nodes, info, err := a.subService.NodesBySubID(subID, host)
	if err != nil {
		c.String(http.StatusNotFound, err.Error())
		return
	}

	title := fmt.Sprintf("X-Panel 订阅 %s（%d 个节点）", subID, len(nodes))
	var body, contentType string
	switch target {
	case "clash", "mihomo":
		body, contentType = service.ClashConfig(nodes, title), "text/yaml; charset=utf-8"
	case "singbox", "sing-box":
		body, contentType = service.SingboxConfig(nodes, title), "application/json; charset=utf-8"
	case "surge":
		body, contentType = service.SurgeConfig(nodes, title), "text/plain; charset=utf-8"
	default:
		body, contentType = service.LinksBase64(nodes), "text/plain; charset=utf-8"
	}
	if strings.TrimSpace(body) == "" {
		c.String(http.StatusNotFound, "该订阅没有可用的节点格式")
		return
	}

	// 客户端据此显示流量与到期时间
	c.Header("Subscription-Userinfo", fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d",
		info.Upload, info.Download, info.Total, info.Expire))
	c.Header("Profile-Update-Interval", "12")
	c.Header("Profile-Title", title)
	c.Header("Content-Type", contentType)
	c.String(http.StatusOK, body)
}

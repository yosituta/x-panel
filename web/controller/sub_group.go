package controller

import (
	"net"
	"net/http"
	"strconv"
	"strings"

	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

// 【开源增强】订阅系统控制器
//
// 面板内管理（需登录）：/panel/api/subgroup/*
// 对外订阅（靠 token 鉴权）：/subg/:token?target=clash|v2ray|singbox|surge
type SubGroupController struct {
	BaseController

	sgService service.SubGroupService
}

func NewSubGroupController(g *gin.RouterGroup) *SubGroupController {
	a := &SubGroupController{}
	a.initRouter(g)
	return a
}

func (a *SubGroupController) initRouter(g *gin.RouterGroup) {
	r := g.Group("/panel/api/subgroup")
	r.Use(a.checkLogin)

	r.GET("/list", a.list)
	r.POST("/add", a.add)
	r.POST("/update", a.update)
	r.POST("/delete", a.delete)

	// 公开订阅入口
	g.GET("/subg/:token", a.render)
}

func (a *SubGroupController) list(c *gin.Context) {
	groups, err := a.sgService.List()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	type item struct {
		Id        int    `json:"id"`
		Name      string `json:"name"`
		Token     string `json:"token"`
		Remark    string `json:"remark"`
		Nodes     string `json:"nodes"`
		NodeCount int    `json:"nodeCount"`
		UpdatedAt int64  `json:"updatedAt"`
	}
	out := make([]item, 0, len(groups))
	for _, g := range groups {
		out = append(out, item{
			Id: g.Id, Name: g.Name, Token: g.Token, Remark: g.Remark, Nodes: g.Nodes,
			NodeCount: a.sgService.CountNodes(g.Nodes), UpdatedAt: g.UpdatedAt,
		})
	}
	jsonObj(c, out, nil)
}

type subGroupForm struct {
	Id     int    `json:"id" form:"id"`
	Name   string `json:"name" form:"name"`
	Nodes  string `json:"nodes" form:"nodes"`
	Remark string `json:"remark" form:"remark"`
}

func (a *SubGroupController) add(c *gin.Context) {
	var f subGroupForm
	if err := c.ShouldBind(&f); err != nil {
		jsonMsg(c, "参数错误", err)
		return
	}
	g, err := a.sgService.Create(f.Name, f.Nodes, f.Remark)
	if err != nil {
		jsonMsg(c, err.Error(), err)
		return
	}
	jsonObj(c, g, nil)
}

func (a *SubGroupController) update(c *gin.Context) {
	var f subGroupForm
	if err := c.ShouldBind(&f); err != nil || f.Id <= 0 {
		jsonMsg(c, "参数错误", err)
		return
	}
	if err := a.sgService.Update(f.Id, f.Name, f.Nodes, f.Remark); err != nil {
		jsonMsg(c, err.Error(), err)
		return
	}
	jsonMsg(c, "修改成功", nil)
}

func (a *SubGroupController) delete(c *gin.Context) {
	var f subGroupForm
	if err := c.ShouldBind(&f); err != nil || f.Id <= 0 {
		jsonMsg(c, "参数错误", err)
		return
	}
	if err := a.sgService.Delete(f.Id); err != nil {
		jsonMsg(c, err.Error(), err)
		return
	}
	jsonMsg(c, "删除成功", nil)
}

func (a *SubGroupController) render(c *gin.Context) {
	token := strings.TrimSpace(c.Param("token"))
	target := strings.ToLower(strings.TrimSpace(c.Query("target")))
	for suffix, t := range map[string]string{".yaml": "clash", ".yml": "clash", ".json": "singbox", ".conf": "surge"} {
		if strings.HasSuffix(token, suffix) {
			token = strings.TrimSuffix(token, suffix)
			if target == "" {
				target = t
			}
		}
	}

	g, err := a.sgService.GetByToken(token)
	if err != nil {
		c.String(http.StatusNotFound, "订阅不存在")
		return
	}

	body, contentType := a.sgService.Render(g, target)
	if strings.TrimSpace(body) == "" {
		c.String(http.StatusNotFound, "该订阅没有可用节点")
		return
	}

	title := g.Name
	c.Header("Profile-Title", title)
	c.Header("Profile-Update-Interval", "12")
	c.Header("Content-Type", contentType)
	c.Header("Subscription-Userinfo", "upload=0; download=0; total=0; expire=0")
	c.String(http.StatusOK, body)
}

// 保留 net/strconv 引用，避免未来扩展时再次调整 import
var _ = net.SplitHostPort
var _ = strconv.Itoa

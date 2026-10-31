package controller

import (
	"github.com/gin-gonic/gin"
)

// 【开源增强】界面版本探针（公开访问，无需登录）
//
// 用途：升级面板后，浏览器/代理可能缓存旧页面。
// 打开 /panel/api/uiinfo 就能确认服务器实际提供的是哪一版界面：
//   - 返回的 ui 字段是新值  → 服务端已是新版，看到的旧界面是缓存
//   - 返回的 ui 字段是旧值  → 服务端确实没更新
const uiTag = "2026-10-31-cert-cdn-v5"

type UIController struct {
	BaseController
}

func NewUIController(g *gin.RouterGroup) *UIController {
	a := &UIController{}
	a.initRouter(g)
	return a
}

func (a *UIController) initRouter(g *gin.RouterGroup) {
	// 不挂在 checkLogin 组下，未登录也能看
	g.GET("/panel/api/uiinfo", a.info)
}

func (a *UIController) info(c *gin.Context) {
	jsonObj(c, gin.H{
		"ui":              uiTag,
		"cdnButtons":      true, // 证书/CDN 页含「绑定」「解绑」按钮
		"certUpload":      true, // 支持上传证书文件
		"panelRestartBtn": true, // 证书保存与重启分离
	}, nil)
}

package service

import (
	"fmt"
	"strings"
)

// 【开源增强】TG 多面板命令：/panels 列表，/panel <名称> status|links|restart

func (t *Tgbot) PanelsSummary() string {
	var ps RemotePanelService
	list, err := ps.List()
	if err != nil {
		return "读取面板列表失败：" + err.Error()
	}
	if len(list) == 0 {
		return "还没有添加被控端面板。\n\n用法：在面板后台调用 <code>POST /panel/api/panels/add</code> 添加，格式：\n<code>{\"name\":\"东京\",\"url\":\"https://ip:端口/路径\",\"key\":\"被控端slaveKey\"}</code>"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("🖥 共 %d 个被控端：\r\n\r\n", len(list)))
	for _, p := range list {
		st, err := p.Status()
		if err != nil {
			b.WriteString(fmt.Sprintf("🔴 %s —— 离线（%s）\r\n", p.Name, shortErr(err)))
			continue
		}
		b.WriteString(fmt.Sprintf("🟢 %s —— 在线｜入站 %v｜xray %v｜v%v\r\n",
			p.Name, st["inbounds"], st["xrayRunning"], st["version"]))
	}
	b.WriteString("\r\n查看详情：<code>/panel 名称 status</code>")
	return b.String()
}

func (t *Tgbot) PanelCommand(args []string) string {
	if len(args) == 0 {
		return "用法：<code>/panel 名称 status|links|restart</code>"
	}
	if len(args) == 1 {
		return t.PanelsSummary()
	}
	name := args[0]
	action := strings.ToLower(args[1])

	var ps RemotePanelService
	p, err := ps.Get(name)
	if err != nil {
		return "❗ " + err.Error()
	}

	switch action {
	case "status":
		st, err := p.Status()
		if err != nil {
			return "🔴 连接失败：" + shortErr(err)
		}
		return fmt.Sprintf("🖥 %s\r\n\r\n主机：%v\r\n版本：v%v\r\nXray：%v\r\n入站：%v（启用 %v）",
			name, st["hostname"], st["version"], st["xrayRunning"], st["inbounds"], st["enabled"])
	case "links":
		text, err := p.Links()
		if err != nil {
			return "❗ 拉取失败：" + shortErr(err)
		}
		if strings.TrimSpace(text) == "" {
			return "该面板没有可用节点"
		}
		return "🔗 " + name + " 的节点：\r\n\r\n" + text
	case "restart":
		if err := p.RestartXray(); err != nil {
			return "❗ 重启失败：" + shortErr(err)
		}
		return "✅ 已让 " + name + " 重启 Xray"
	}
	return "未知操作：" + action
}

func shortErr(err error) string {
	s := err.Error()
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}

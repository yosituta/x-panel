package service

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"x-ui/logger"
)

// 中文注释: 【开源增强】按端口动态管理防火墙
//
// 问题背景：服务器一旦启用 ufw（默认 deny incoming），
// 面板端口、随机生成的节点端口、订阅端口全部会被丢包，
// 表现为「面板一直转圈」「节点建好了却连不上」。
//
// 这里统一处理：需要用到某个端口时，才放行那个端口；
// ufw 与 firewalld 都支持，且只在防火墙处于启用状态时才动手。

var fwMu sync.Mutex
var fwOpened = map[int]bool{}

// fwBackend 返回当前生效的防火墙类型："ufw" / "firewalld" / ""
func fwBackend() string {
	if _, err := exec.LookPath("ufw"); err == nil {
		if out, err := exec.Command("ufw", "status").Output(); err == nil &&
			strings.Contains(strings.ToLower(string(out)), "status: active") {
			return "ufw"
		}
	}
	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		if err := exec.Command("firewall-cmd", "--state").Run(); err == nil {
			return "firewalld"
		}
	}
	return ""
}

// EnsureFirewallPort 放行单个 TCP 端口（幂等，进程内缓存）
func EnsureFirewallPort(port int) {
	if port <= 0 || port > 65535 {
		return
	}
	fwMu.Lock()
	defer fwMu.Unlock()
	if fwOpened[port] {
		return
	}

	var err error
	switch fwBackend() {
	case "ufw":
		err = exec.Command("ufw", "allow", strconv.Itoa(port)+"/tcp").Run()
	case "firewalld":
		_ = exec.Command("firewall-cmd", "--add-port", strconv.Itoa(port)+"/tcp", "--permanent").Run()
		err = exec.Command("firewall-cmd", "--reload").Run()
	default:
		return // 没启用防火墙，不需要处理
	}

	if err != nil {
		logger.Warningf("防火墙放行端口 %d 失败: %v", port, err)
		return
	}
	fwOpened[port] = true
	logger.Infof("防火墙已放行端口 %d/tcp", port)
}

// EnsureFirewallPorts 批量放行
func EnsureFirewallPorts(ports ...int) {
	for _, p := range ports {
		EnsureFirewallPort(p)
	}
}

// FirewallStatus 给面板展示用：当前防火墙类型 + 已放行端口数
func FirewallStatus() string {
	backend := fwBackend()
	if backend == "" {
		return "未启用防火墙"
	}
	fwMu.Lock()
	n := len(fwOpened)
	fwMu.Unlock()
	return fmt.Sprintf("%s（本次运行已自动放行 %d 个端口）", backend, n)
}

package service

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// 【开源增强】TG /webssh 命令：报告网页版 SSH（ttyd）的地址与状态

const webSSHEnvFile = "/etc/x-panel-ttyd.env"

// WebSSHInfo 读取 ttyd 配置，返回给管理员看的文本
func (t *Tgbot) WebSSHInfo() string {
	f, err := os.Open(webSSHEnvFile)
	if err != nil {
		return "🖥 网页版 SSH 未安装。\n\n安装方式：服务器上执行 <code>x-ui</code> → 选择 <b>26</b> → 安装并启动，\n或直接运行仓库里的 <code>scripts/ttyd.sh install</code>。"
	}
	defer f.Close()

	conf := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			conf[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}

	port := conf["TTYD_PORT"]
	user := conf["TTYD_USER"]
	pass := conf["TTYD_PASS"]
	if port == "" {
		return "🖥 网页版 SSH 配置不完整，请重新执行 scripts/ttyd.sh install"
	}

	running := portOpen("127.0.0.1", port)
	state := "🔴 未运行"
	if running {
		state = "🟢 运行中"
	}

	scheme := "http"
	if _, err := os.Stat("/etc/x-panel-ttyd.cert"); err == nil {
		scheme = "https"
	}
	host := hostname
	if host == "" {
		host = "<服务器IP>"
	}

	return fmt.Sprintf("🖥 网页版 SSH %s\n\n地址：<code>%s://%s:%s</code>\n用户名：<code>%s</code>\n密码：<code>%s</code>\n\n⚠️ 用完请及时卸载（x-ui → 26 → 卸载）",
		state, scheme, host, port, user, pass)
}

func portOpen(host, port string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 2*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

package service

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// 只在真实服务器上跑（需要 root + ufw/firewalld）：
//   XUI_FW_TEST=1 go test ./web/service -run TestFirewallAddRemove -v
func TestFirewallAddRemove(t *testing.T) {
	if os.Getenv("XUI_FW_TEST") != "1" {
		t.Skip("需要 root + 真实防火墙，设置 XUI_FW_TEST=1 才执行")
	}
	port := 18999

	EnsureFirewallPort(port)
	if !fwHasPort(port) {
		t.Fatalf("放行 %d 后仍不在规则里", port)
	}

	RemoveFirewallPort(port)
	if fwHasPort(port) {
		t.Fatalf("关闭 %d 后规则仍然存在", port)
	}
}

func fwHasPort(port int) bool {
	ps := itoa(port)
	if _, err := exec.LookPath("ufw"); err == nil {
		out, _ := exec.Command("ufw", "status").Output()
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), ps) {
				return true
			}
		}
		return false
	}
	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		out, _ := exec.Command("firewall-cmd", "--list-ports").Output()
		return strings.Contains(string(out), ps+"/tcp")
	}
	return false
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

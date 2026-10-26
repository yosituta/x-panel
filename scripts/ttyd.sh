#!/usr/bin/env bash
# X-Panel 开源分支：网页版 SSH（ttyd）
# 用法：
#   bash ttyd.sh install [端口]     安装并启动（默认 7681）
#   bash ttyd.sh uninstall          卸载
#   bash ttyd.sh status             查看状态与登录信息
# 说明：使用 basic auth + 面板证书（有则启用 HTTPS），账号密码写在 /etc/x-panel-ttyd.env

set -euo pipefail

ENV_FILE="/etc/x-panel-ttyd.env"
UNIT_FILE="/etc/systemd/system/x-panel-ttyd.service"
DEFAULT_PORT=7681

red='\033[0;31m'; green='\033[0;32m'; yellow='\033[0;33m'; plain='\033[0m'

info() { echo -e "${green}$*${plain}"; }
warn() { echo -e "${yellow}$*${plain}"; }
err()  { echo -e "${red}$*${plain}"; }

need_root() {
  if [[ $EUID -ne 0 ]]; then err "请用 root 运行"; exit 1; fi
}

install_pkg() {
  if command -v ttyd >/dev/null 2>&1; then return 0; fi
  info "安装 ttyd ..."
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update -qq || true
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq ttyd >/dev/null 2>&1 || true
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y ttyd >/dev/null 2>&1 || true
  fi
  if ! command -v ttyd >/dev/null 2>&1; then
    warn "软件源里没有 ttyd，改用官方静态二进制"
    local arch url
    arch=$(uname -m)
    case "$arch" in
      x86_64) url="https://github.com/tsl0922/ttyd/releases/latest/download/ttyd.x86_64" ;;
      aarch64) url="https://github.com/tsl0922/ttyd/releases/latest/download/ttyd.aarch64" ;;
      *) err "不支持的架构: $arch"; exit 1 ;;
    esac
    curl -fsSL --retry 3 -o /usr/local/bin/ttyd "$url"
    chmod +x /usr/local/bin/ttyd
  fi
  command -v ttyd >/dev/null || { err "ttyd 安装失败"; exit 1; }
}

panel_cert() {
  # 复用面板证书：优先读面板设置里的路径，其次按域名目录猜
  local cert key
  cert=$(/usr/local/x-ui/x-ui setting -getCert true 2>/dev/null | grep -Eo 'cert: .+' | awk '{print $2}' || true)
  key=$(/usr/local/x-ui/x-ui setting -getCert true 2>/dev/null | grep -Eo 'key: .+' | awk '{print $2}' || true)
  if [[ -n "${cert:-}" && -n "${key:-}" && -f "$cert" && -f "$key" ]]; then
    echo "$cert|$key"
  fi
}

do_install() {
  need_root
  local port="${1:-$DEFAULT_PORT}"
  install_pkg

  local user pass
  user="admin"
  pass=$(head -c 18 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 14)

  local tls_args="" scheme="http"
  local pair; pair=$(panel_cert || true)
  if [[ -n "$pair" ]]; then
    local cert="${pair%%|*}" key="${pair##*|}"
    tls_args="-s -C $cert -K $key"
    scheme="https"
  else
    warn "未找到面板证书，将以 HTTP 暴露（不建议公网使用）"
  fi

  cat > "$ENV_FILE" <<EOF
TTYD_PORT=$port
TTYD_USER=$user
TTYD_PASS=$pass
EOF
  chmod 600 "$ENV_FILE"

  cat > "$UNIT_FILE" <<EOF
[Unit]
Description=X-Panel Web SSH (ttyd)
After=network.target

[Service]
Type=simple
EnvironmentFile=$ENV_FILE
ExecStart=/bin/sh -c 'exec \$(command -v ttyd) -p \$TTYD_PORT -i 0.0.0.0 -c \$TTYD_USER:\$TTYD_PASS $tls_args -W bash'
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable x-panel-ttyd >/dev/null 2>&1 || true
  systemctl restart x-panel-ttyd

  local ip
  ip=$(curl -fsS --max-time 5 https://api.ipify.org 2>/dev/null || hostname -I | awk '{print $1}')
  sleep 1
  if systemctl is-active --quiet x-panel-ttyd; then
    info "✅ 网页版 SSH 已启动"
    echo -e "   地址:   ${yellow}$scheme://$ip:$port${plain}"
    echo -e "   用户名: ${yellow}$user${plain}"
    echo -e "   密码:   ${yellow}$pass${plain}"
    echo -e "   配置:   $ENV_FILE（改完 systemctl restart x-panel-ttyd 生效）"
    warn "⚠️ 记得在防火墙/安全组放行 $port 端口；不用时建议 bash ttyd.sh uninstall"
  else
    err "启动失败，查看日志：journalctl -u x-panel-ttyd -n 50"
  fi
}

do_uninstall() {
  need_root
  systemctl stop x-panel-ttyd >/dev/null 2>&1 || true
  systemctl disable x-panel-ttyd >/dev/null 2>&1 || true
  rm -f "$UNIT_FILE" "$ENV_FILE"
  systemctl daemon-reload
  info "✅ 已卸载（ttyd 程序本身保留，可继续给别的用途）"
}

do_status() {
  if ! systemctl list-unit-files 2>/dev/null | grep -q '^x-panel-ttyd'; then
    warn "尚未安装，执行：bash ttyd.sh install"
    return 0
  fi
  local state
  state=$(systemctl is-active x-panel-ttyd 2>/dev/null || true)
  echo -e "服务状态: ${yellow}$state${plain}"
  if [[ -f "$ENV_FILE" ]]; then
    # shellcheck disable=SC1090
    source "$ENV_FILE"
    echo -e "端口: ${yellow}$TTYD_PORT${plain}  用户: ${yellow}$TTYD_USER${plain}  密码: ${yellow}$TTYD_PASS${plain}"
  fi
}

case "${1:-}" in
  install) shift || true; do_install "${1:-$DEFAULT_PORT}" ;;
  uninstall) do_uninstall ;;
  status) do_status ;;
  *) echo "用法: bash ttyd.sh {install [端口]|uninstall|status}" ;;
esac

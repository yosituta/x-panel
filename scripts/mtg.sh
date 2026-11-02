#!/usr/bin/env bash
# ==========================================================
# Telegram MTProto 代理（9seconds/mtg v2，FakeTLS）
#   install [域名] [端口]   安装/修复
#   status                  查看状态
#   link                    打印连接链接（tg:// 与 t.me）
#   uninstall               卸载
# ==========================================================
set -uo pipefail

red='\033[0;31m'; green='\033[0;32m'; yellow='\033[0;33m'; plain='\033[0m'
BIN=/usr/local/bin/mtg
CONF=/etc/mtg.toml
UNIT=/etc/systemd/system/mtg.service
DEFAULT_PORT=8443

log()  { echo -e "${green}$*${plain}"; }
warn() { echo -e "${yellow}$*${plain}"; }
err()  { echo -e "${red}$*${plain}"; }

arch_asset() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "linux-amd64" ;;
        aarch64|arm64) echo "linux-arm64" ;;
        armv7l)        echo "linux-armv7" ;;
        armv6l)        echo "linux-armv6" ;;
        *)             echo "" ;;
    esac
}

current_domain() {
    if [[ -f /root/cert/.panel_domain ]]; then
        tr -d '[:space:]' < /root/cert/.panel_domain
    fi
}

current_port() {
    if [[ -f "$CONF" ]]; then
        awk -F'"' '/bind-to/ {split($2,a,":"); print a[length(a)]}' "$CONF" 2>/dev/null | head -1
    fi
}

do_install() {
    local domain="${1:-}" port="${2:-$DEFAULT_PORT}"
    local arch; arch=$(arch_asset)
    if [[ -z "$arch" ]]; then err "不支持的架构：$(uname -m)"; return 1; fi

    if [[ -z "$domain" ]]; then
        domain=$(current_domain)
    fi
    if [[ -z "$domain" ]]; then
        read -rp "  用于伪装的域名（FakeTLS 的 SNI，例如 hk.example.com）: " domain
    fi
    [[ -z "$domain" ]] && { err "必须提供域名（FakeTLS 需要一个真实域名做 SNI）"; return 1; }

    if [[ -z "$port" ]]; then
        read -rp "  监听端口 [${DEFAULT_PORT}]: " port
        port="${port:-$DEFAULT_PORT}"
    fi

    log "---- 1/5 下载 mtg（$arch）----"
    local tag url tmp
    tag=$(curl -fsSL --max-time 20 https://api.github.com/repos/9seconds/mtg/releases/latest 2>/dev/null \
          | grep '"tag_name":' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
    tag="${tag:-v2.2.8}"
    url="https://github.com/9seconds/mtg/releases/download/${tag}/mtg-${tag#v}-${arch}.tar.gz"
    tmp=$(mktemp -d)
    if ! curl -fsSL --max-time 120 -o "$tmp/mtg.tar.gz" "$url"; then
        err "下载失败：$url"
        rm -rf "$tmp"; return 1
    fi
    tar -xzf "$tmp/mtg.tar.gz" -C "$tmp" || { err "解包失败"; rm -rf "$tmp"; return 1; }
    install -m 0755 "$(find "$tmp" -type f -name mtg | head -1)" "$BIN"
    rm -rf "$tmp"
    log "  已安装：$($BIN --version 2>&1 | head -1)"

    log "---- 2/5 生成 FakeTLS secret ----"
    local secret
    secret=$("$BIN" generate-secret "$domain" 2>/dev/null | head -1)
    if [[ -z "$secret" ]]; then err "generate-secret 失败"; return 1; fi
    log "  伪装域名：$domain"

    log "---- 3/5 写配置 ----"
    cat > "$CONF" <<EOF
# X-Panel 生成：Telegram MTProto 代理（FakeTLS，伪装域名 $domain）
secret = "$secret"
bind-to = "0.0.0.0:$port"

# 并发与超时（默认值已够用，需要时再调）
# concurrency = 8192
# idle-timeout = "5m"
EOF
    chmod 600 "$CONF"
    log "  $CONF"

    log "---- 4/5 写 systemd 单元并启动 ----"
    cat > "$UNIT" <<'EOF'
[Unit]
Description=mtg - Telegram MTProto proxy (FakeTLS)
Documentation=https://github.com/9seconds/mtg
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/mtg run /etc/mtg.toml
Restart=always
RestartSec=3
# 不能用 DynamicUser：配置里含 secret，权限是 600 root，动态用户读不到（会报 permission denied）
User=root
NoNewPrivileges=true
LimitNOFILE=65536
AmbientCapabilities=CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable mtg >/dev/null 2>&1
    systemctl restart mtg
    sleep 2

    log "---- 5/5 防火墙 ----"
    if command -v ufw >/dev/null 2>&1; then
        ufw allow "${port}/tcp" >/dev/null 2>&1 && log "  ufw 已放行 ${port}/tcp"
    elif command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
        firewall-cmd --add-port="${port}/tcp" --permanent >/dev/null 2>&1
        firewall-cmd --reload >/dev/null 2>&1 && log "  firewalld 已放行 ${port}/tcp"
    else
        warn "  未检测到 ufw/firewalld，云厂商安全组请自行放行 ${port}/tcp"
    fi

    if systemctl is-active --quiet mtg; then
        log "mtg 已启动"
    else
        err "mtg 启动失败，看日志：journalctl -u mtg -n 30 --no-pager"
        return 1
    fi

    do_link
}

do_link() {
    if [[ ! -f "$CONF" ]]; then err "尚未安装，先执行：mtg.sh install"; return 1; fi
    local out
    out=$("$BIN" access "$CONF" 2>/dev/null)
    local tg tme
    tg=$(echo "$out" | grep -o '"tg_url": *"[^"]*"' | head -1 | sed -E 's/.*"(tg:[^"]*)"/\1/')
    tme=$(echo "$out" | grep -o '"tme_url": *"[^"]*"' | head -1 | sed -E 's/.*"(https:[^"]*)"/\1/')
    echo
    log "手机 Telegram 点这个链接即可添加代理："
    [[ -n "$tg"  ]] && echo -e "  ${green}${tg}${plain}"
    [[ -n "$tme" ]] && echo -e "  备用：$tme"
    echo
    echo "  端口：$(current_port)    配置文件：$CONF"
    echo "  也可手动填：Telegram → 设置 → 高级 → 连接类型 → 代理 → MTProto"
}

do_status() {
    echo "==== mtg 状态 ===="
    if [[ ! -x "$BIN" ]]; then err "未安装"; return 0; fi
    systemctl is-active mtg >/dev/null 2>&1 && log "服务：运行中" || err "服务：未运行"
    echo "  端口：$(current_port)"
    echo "  secret：$(awk -F'"' '/^secret/ {print substr($2,1,16)"...(共"length($2)"字符)"}' "$CONF" 2>/dev/null)"
    echo "  最近日志："
    journalctl -u mtg -n 5 --no-pager 2>/dev/null | sed 's/^/    /'
}

do_uninstall() {
    local port; port=$(current_port)
    echo "将卸载 mtg（停服务、删二进制与配置）"
    read -rp "  确认？[y/N]: " yn
    [[ "$yn" =~ ^[Yy] ]] || { echo "已取消"; return 0; }

    systemctl stop mtg >/dev/null 2>&1
    systemctl disable mtg >/dev/null 2>&1
    rm -f "$UNIT" "$CONF" "$BIN"
    systemctl daemon-reload >/dev/null 2>&1
    if [[ -n "$port" ]] && command -v ufw >/dev/null 2>&1; then
        ufw delete allow "${port}/tcp" >/dev/null 2>&1 && echo "  ufw 已撤销 ${port}/tcp"
    fi
    log "已卸载"
}

case "${1:-}" in
    install)   shift; do_install "${1:-}" "${2:-}" ;;
    status)    do_status ;;
    link)      do_link ;;
    uninstall) do_uninstall ;;
    *)
        echo "用法：mtg.sh {install [域名] [端口] | status | link | uninstall}"
        ;;
esac

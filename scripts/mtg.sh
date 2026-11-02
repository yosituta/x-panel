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

# 伪装域名池：都是中国境内可正常访问、支持 TLS 的大站。
# mtg 只借用它们的域名做 SNI / 探针回退，数据不会经过这些站点。
CLOAK_POOL=(
    www.1688.com
    www.taobao.com
    www.jd.com
    www.microsoft.com
    www.cloudflare.com
    www.bing.com
    www.apple.com
    www.qq.com
    www.dell.com
    www.intel.com
)

# 随机挑一个，并且确认从本机能正常 TLS 访问（挑不通就换）
random_cloak() {
    local d i
    for i in 1 2 3 4 5; do
        d="${CLOAK_POOL[$((RANDOM % ${#CLOAK_POOL[@]}))]}"
        if curl -sI --max-time 6 "https://$d" >/dev/null 2>&1; then
            echo "$d"
            return 0
        fi
    done
    echo "www.microsoft.com"
}

# 读上次用的伪装域名（写在 /etc/mtg.toml 的注释里），修复时沿用，避免 secret 无故变化
saved_cloak() {
    [[ -f /etc/mtg.toml ]] || return 0
    awk -F': ' '/^# cloak-domain:/ {print $2; exit}' /etc/mtg.toml 2>/dev/null | tr -d '[:space:]'
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

    # 伪装域名：不需要手输。优先用上次记录的，其次从中国可访问的大站里随机挑一个。
    # 想指定：mtg.sh install <域名> [端口]
    if [[ -z "$domain" ]]; then
        domain=$(saved_cloak)
        if [[ -n "$domain" ]]; then
            log "  沿用上次的伪装域名：$domain（想换：mtg.sh install <域名>）"
        else
            domain=$(random_cloak)
            log "  自动随机选择伪装域名：$domain"
        fi
    else
        log "  使用指定伪装域名：$domain"
    fi

    local panel_domain; panel_domain=$(current_domain)
    if [[ -n "$panel_domain" && "$domain" == "$panel_domain" ]]; then
        err "  不能用自己的面板域名（$domain）做伪装：它指向本机，探针会打回自己，伪装会失效。"
        err "  请换一个大站，例如：mtg.sh install www.microsoft.com"
        return 1
    fi

    port="${port:-$DEFAULT_PORT}"

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
# cloak-domain: $domain
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

    # 连接地址可以换成域名（只影响显示与客户端解析，不改变实际连到的机器）
    local linkhost ip
    linkhost=$(awk -F': ' '/^# link-host:/ {print $2; exit}' "$CONF" 2>/dev/null | tr -d '[:space:]')
    if [[ -n "$linkhost" ]]; then
        ip=$(echo "$tg" | sed -n 's/.*server=\([^&]*\).*/\1/p')
        [[ -n "$ip" ]] && tg="${tg//server=$ip/server=$linkhost}" && tme="${tme//server=$ip/server=$linkhost}"
        echo -e "  连接地址：${green}${linkhost}${plain}（域名，解析到本机）"
    else
        echo -e "  连接地址：$(echo "$tg" | sed -n 's/.*server=\([^&]*\).*/\1/p')（IP）"
        echo -e "  想用域名显示：mtg.sh sethost 你的域名"
    fi
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

do_sethost() {
    local host="${1:-}"
    if [[ ! -f "$CONF" ]]; then err "尚未安装，先执行：mtg.sh install"; return 1; fi
    if [[ -z "$host" ]]; then
        host=$(current_domain)
        [[ -z "$host" ]] && { err "请给出域名：mtg.sh sethost hk.example.com"; return 1; }
    fi
    # 去掉旧的记录再写新的
    sed -i '/^# link-host:/d' "$CONF"
    sed -i "1i # link-host: $host" "$CONF"
    log "连接地址已设为：$host（tg:// 链接里会用域名代替 IP）"
    echo "  注意：该域名必须解析到本机，否则客户端连不上。"
    echo "  还原成 IP：sed -i '/^# link-host:/d' $CONF"
}

case "${1:-}" in
    install)   shift; do_install "${1:-}" "${2:-}" ;;
    sethost)   shift; do_sethost "${1:-}" ;;
    status)    do_status ;;
    link)      do_link ;;
    uninstall) do_uninstall ;;
    *)
        echo "用法：mtg.sh {install [伪装域名] [端口] | link | status | sethost [域名] | uninstall}"
        ;;
esac

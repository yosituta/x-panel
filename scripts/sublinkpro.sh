#!/bin/bash
# ============================================================
# SublinkPro 订阅系统 安装 / 管理 / 独立域名
#   用法：
#     bash sublinkpro.sh install                       # 安装（沿用面板证书或自签，端口 15268）
#     bash sublinkpro.sh install --domain sub.example.com [--port 8443]   # 同时绑定独立域名
#     bash sublinkpro.sh domain sub.example.com [--port 8443]             # 只加/换独立域名
#     bash sublinkpro.sh status | uninstall
# ============================================================

GREEN='\033[0;32m'; YELLOW='\033[0;33m'; RED='\033[0;31m'; PLAIN='\033[0m'
APP_DIR=/opt/sublinkpro
BIN=$APP_DIR/sublinkPro
SERVICE=sublinkpro
SVC_FILE=/etc/systemd/system/$SERVICE.service
PORT=15268
BACKEND=8000
REPO=ZeroDeng01/sublinkPro
VER="${SUBLINKPRO_VER:-v1.2.22}"
CONF=/etc/nginx/sites-available/sublinkpro
DOMAIN_FILE=$APP_DIR/domain.conf   # 记录：SUB_DOMAIN / SUB_PORT

need_root() { [ "$(id -u)" -eq 0 ] || { echo -e "${RED}请用 root 运行${PLAIN}"; exit 1; }; }

find_cert() {
    local c
    c=$(ls -1 /etc/letsencrypt/live/*/fullchain.pem 2>/dev/null | head -1)
    [ -z "$c" ] && c=$(ls -1 /root/cert/*/fullchain.pem 2>/dev/null | head -1)
    # 排除订阅系统自己的自签证书
    case "$c" in /etc/ssl/sublinkpro/*) c="" ;; esac
    echo "$c"
}

cert_domain() { basename "$(dirname "$1")"; }

allow_port() {
    local p=$1
    command -v ufw >/dev/null 2>&1 && ufw allow "$p"/tcp >/dev/null 2>&1 && echo -e "  ${GREEN}ufw 已放行 $p/tcp${PLAIN}"
    if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
        firewall-cmd --add-port="$p"/tcp --permanent >/dev/null 2>&1
        firewall-cmd --reload >/dev/null 2>&1
        echo -e "  ${GREEN}firewalld 已放行 $p/tcp${PLAIN}"
    fi
}

# 读取已保存的域名配置；命令行显式给的值优先（CLI_DOMAIN / CLI_PORT）
load_domain_conf() {
    local cli_d="$CLI_DOMAIN" cli_p="$CLI_PORT" saved_d="" saved_p=""
    if [ -f "$DOMAIN_FILE" ]; then
        . "$DOMAIN_FILE"
        saved_d="$SUB_DOMAIN"; saved_p="$SUB_PORT"
    fi
    SUB_DOMAIN="${cli_d:-$saved_d}"
    SUB_PORT="${cli_p:-${saved_p:-8443}}"
}

save_domain_conf() {
    mkdir -p "$APP_DIR"
    printf 'SUB_DOMAIN=%s\nSUB_PORT=%s\n' "$SUB_DOMAIN" "$SUB_PORT" > "$DOMAIN_FILE"
}

# 申请域名证书（standalone 占用 80，期间短暂停 nginx）
issue_cert() {
    local domain=$1 email=$2
    if [ -s "/root/cert/${domain}/fullchain.pem" ]; then
        echo "  已有证书：/root/cert/${domain}/fullchain.pem"
        return 0
    fi
    command -v ~/.acme.sh/acme.sh >/dev/null 2>&1 || {
        curl -s https://get.acme.sh | sh >/dev/null 2>&1
    }
    [ -x /root/.acme.sh/acme.sh ] || { echo -e "  ${RED}acme.sh 安装失败${PLAIN}"; return 1; }
    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt >/dev/null 2>&1
    [ -z "$email" ] && email="admin@${domain#*.}"

    echo "  正在签发证书（占用 80 端口，nginx 暂停约 20 秒）..."
    systemctl stop nginx >/dev/null 2>&1
    if ~/.acme.sh/acme.sh --issue -d "$domain" --standalone --days 15 --force >/tmp/sublinkpro-acme.log 2>&1; then
        mkdir -p "/root/cert/${domain}"
        ~/.acme.sh/acme.sh --install-cert -d "$domain" \
            --key-file "/root/cert/${domain}/privkey.pem" \
            --fullchain-file "/root/cert/${domain}/fullchain.pem" \
            --reloadcmd "systemctl reload nginx" >/dev/null 2>&1
        systemctl start nginx >/dev/null 2>&1
        if [ -s "/root/cert/${domain}/fullchain.pem" ]; then
            echo -e "  ${GREEN}证书已签发：/root/cert/${domain}/${PLAIN}"
            # 确保续期定时任务存在（剩余 15 天自动续）
            crontab -l 2>/dev/null | grep -q "acme.sh --cron" || \
                ~/.acme.sh/acme.sh --install-cronjob >/dev/null 2>&1
            return 0
        fi
    fi
    systemctl start nginx >/dev/null 2>&1
    echo -e "  ${RED}证书签发失败，日志：/tmp/sublinkpro-acme.log${PLAIN}"
    tail -5 /tmp/sublinkpro-acme.log 2>/dev/null | sed 's/^/    /'
    return 1
}

# 生成 nginx 配置：15268（IP/端口访问）+ 可选独立域名块
write_nginx() {
    local base_cert=$1 base_domain=$2
    local key="${base_cert%fullchain.pem}privkey.pem"
    local domain_cert="" domain_key="" domain_host=""

    if [ -n "$SUB_DOMAIN" ] && [ -s "/root/cert/${SUB_DOMAIN}/fullchain.pem" ]; then
        domain_cert="/root/cert/${SUB_DOMAIN}/fullchain.pem"
        domain_key="/root/cert/${SUB_DOMAIN}/privkey.pem"
        domain_host="$SUB_DOMAIN"
    elif [ -n "$SUB_DOMAIN" ] && [ -s "/etc/ssl/sublinkpro/${SUB_DOMAIN}.crt" ]; then
        domain_cert="/etc/ssl/sublinkpro/${SUB_DOMAIN}.crt"
        domain_key="/etc/ssl/sublinkpro/${SUB_DOMAIN}.key"
        domain_host="$SUB_DOMAIN"
    fi

    {
        echo "server {"
        echo "    listen ${PORT} ssl http2;"
        echo "    server_name ${base_domain};"
        echo "    ssl_certificate     ${base_cert};"
        echo "    ssl_certificate_key ${key};"
        echo "    ssl_protocols TLSv1.2 TLSv1.3;"
        echo "    client_max_body_size 20m;"
        echo "    location / {"
        echo "        proxy_pass http://127.0.0.1:${BACKEND};"
        echo "        proxy_http_version 1.1;"
        echo "        proxy_set_header Host \$host;"
        echo "        proxy_set_header X-Real-IP \$remote_addr;"
        echo "        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;"
        echo "        proxy_set_header X-Forwarded-Proto https;"
        echo "        proxy_set_header Upgrade \$http_upgrade;"
        echo "        proxy_set_header Connection \"upgrade\";"
        echo "        proxy_read_timeout 300s;"
        echo "    }"
        echo "}"
        if [ -n "$domain_host" ]; then
            echo ""
            echo "server {"
            echo "    listen ${SUB_PORT} ssl http2;"
            echo "    server_name ${domain_host};"
            echo "    ssl_certificate     ${domain_cert};"
            echo "    ssl_certificate_key ${domain_key};"
            echo "    ssl_protocols TLSv1.2 TLSv1.3;"
            echo "    client_max_body_size 20m;"
            echo "    location / {"
            echo "        proxy_pass http://127.0.0.1:${BACKEND};"
            echo "        proxy_http_version 1.1;"
            echo "        proxy_set_header Host \$host;"
            echo "        proxy_set_header X-Real-IP \$remote_addr;"
            echo "        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;"
            echo "        proxy_set_header X-Forwarded-Proto https;"
            echo "        proxy_set_header Upgrade \$http_upgrade;"
            echo "        proxy_set_header Connection \"upgrade\";"
            echo "        proxy_read_timeout 300s;"
            echo "    }"
            echo "}"
        fi
    } > "$CONF"

    ln -sf "$CONF" /etc/nginx/sites-enabled/sublinkpro
    if nginx -t >/tmp/sublinkpro-nginx-test.log 2>&1; then
        systemctl reload nginx
        return 0
    fi
    echo -e "  ${RED}nginx 配置有误：${PLAIN}"
    tail -3 /tmp/sublinkpro-nginx-test.log | sed 's/^/    /'
    return 1
}

do_install() {
    need_root
    echo -e "${GREEN}==== 安装 SublinkPro 订阅系统 ====${PLAIN}"
    load_domain_conf

    echo "---- 1/5 下载 ----"
    mkdir -p "$APP_DIR"
    if [ -x "$BIN" ]; then
        echo "  已存在：$BIN（跳过下载）"
    else
        curl -fL --retry 3 -o "$BIN" \
            "https://github.com/${REPO}/releases/download/${VER}/sublinkPro-linux-amd64" || {
            echo -e "${RED}下载失败，检查网络${PLAIN}"; return 1; }
        chmod +x "$BIN"
        echo "  $(ls -lh "$BIN" | awk '{print $5}')"
    fi

    echo "---- 2/5 注册服务 ----"
    cat > "$SVC_FILE" <<EOF
[Unit]
Description=SublinkPro 订阅管理系统
After=network.target

[Service]
Type=simple
WorkingDirectory=${APP_DIR}
ExecStart=${BIN}
Restart=always
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable --now "$SERVICE" >/dev/null 2>&1
    sleep 4
    echo "  服务状态: $(systemctl is-active $SERVICE)"

    echo "---- 3/5 反向代理 ----"
    local cert domain
    cert=$(find_cert)
    if [ -n "$cert" ]; then
        domain=$(cert_domain "$cert")
        echo "  使用面板证书：$cert"
    else
        local ip
        ip=$(curl -s --max-time 5 https://api.ipify.org 2>/dev/null)
        [ -z "$ip" ] && ip=$(hostname -I 2>/dev/null | awk '{print $1}')
        [ -z "$ip" ] && ip=127.0.0.1
        mkdir -p /etc/ssl/sublinkpro
        [ -s /etc/ssl/sublinkpro/fullchain.pem ] || \
            openssl req -x509 -nodes -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 \
                -keyout /etc/ssl/sublinkpro/privkey.pem -out /etc/ssl/sublinkpro/fullchain.pem \
                -days 3650 -subj "/CN=${ip}" >/dev/null 2>&1
        cert=/etc/ssl/sublinkpro/fullchain.pem
        domain=$ip
        echo -e "  ${YELLOW}未找到面板证书 → 用自签证书（浏览器点“继续”）${PLAIN}"
    fi

    # 独立域名：有就签证书并加成第二个 server 块
    if [ -n "$SUB_DOMAIN" ]; then
        echo "  独立域名：${SUB_DOMAIN}:${SUB_PORT}"
        save_domain_conf
        issue_cert "$SUB_DOMAIN" "" || true
        allow_port "$SUB_PORT"
    fi

    write_nginx "$cert" "$domain" || return 1

    echo "---- 4/5 防火墙 ----"
    allow_port "$PORT"

    echo "---- 5/5 验证 ----"
    sleep 2
    local code
    code=$(curl -sk -o /dev/null -w "%{http_code}" --max-time 6 "https://127.0.0.1:${PORT}/" 2>/dev/null)
    echo "  https://127.0.0.1:${PORT} -> ${code}"
    if [ -n "$SUB_DOMAIN" ]; then
        echo -n "  https://${SUB_DOMAIN}:${SUB_PORT} -> "
        curl -sk -o /dev/null -w "%{http_code}\n" --max-time 6 "https://127.0.0.1:${SUB_PORT}/" 2>/dev/null
    fi

    local ip2
    ip2=$(curl -s --max-time 5 https://api.ipify.org 2>/dev/null)
    echo ""
    echo -e "${GREEN}========================================${PLAIN}"
    echo -e "${GREEN} SublinkPro 订阅系统就绪${PLAIN}"
    echo -e "${GREEN}========================================${PLAIN}"
    echo -e "  访问地址 : ${YELLOW}https://${domain}:${PORT}/${PLAIN}"
    if [ -n "$SUB_DOMAIN" ]; then
        echo -e "  独立域名 : ${YELLOW}https://${SUB_DOMAIN}:${SUB_PORT}/${PLAIN}"
        echo "             （若在 Cloudflare 开了橙云，这个端口要改成 8443/2083 等 CF 认可的端口，SSL 用 Full）"
    fi
    echo "  默认账号 : admin"
    echo "  默认密码 : 123456  （登录后请立即修改）"
    echo "  服务名   : ${SERVICE}   日志: journalctl -u ${SERVICE} -f"
}

do_domain() {
    need_root
    load_domain_conf
    [ -z "$SUB_DOMAIN" ] && { echo -e "${RED}用法: $0 domain sub.example.com [--port 8443]${PLAIN}"; return 1; }
    echo -e "${GREEN}==== 绑定独立域名 ${SUB_DOMAIN}:${SUB_PORT} ====${PLAIN}"
    issue_cert "$SUB_DOMAIN" "" || return 1
    allow_port "$SUB_PORT"
    save_domain_conf
    local cert domain
    cert=$(find_cert)
    if [ -z "$cert" ]; then cert=/etc/ssl/sublinkpro/fullchain.pem; fi
    domain=$(cert_domain "$cert")
    write_nginx "$cert" "$domain" || return 1
    echo -e "  ${GREEN}完成：https://${SUB_DOMAIN}:${SUB_PORT}/${PLAIN}"
}

do_status() {
    need_root
    load_domain_conf
    echo "服务: $(systemctl is-active $SERVICE 2>/dev/null)"
    echo "监听: 8000(后端) / ${PORT}(HTTPS)"
    [ -n "$SUB_DOMAIN" ] && echo "独立域名: ${SUB_DOMAIN}:${SUB_PORT}"
    command -v ufw >/dev/null && ufw status 2>/dev/null | grep -E "(${PORT}|${SUB_PORT})" | head -2
}

do_uninstall() {
    need_root
    systemctl disable --now "$SERVICE" >/dev/null 2>&1
    rm -f "$SVC_FILE" /etc/nginx/sites-enabled/sublinkpro "$CONF"
    systemctl daemon-reload
    nginx -t >/dev/null 2>&1 && systemctl reload nginx
    rm -rf "$APP_DIR"
    command -v ufw >/dev/null && ufw delete allow "$PORT"/tcp >/dev/null 2>&1
    echo "已卸载 SublinkPro"
}

# ---- 参数解析 ----
ACTION=""
CLI_DOMAIN=""; CLI_PORT=""
SUB_DOMAIN=""; SUB_PORT=8443
ARGS=()
while [ $# -gt 0 ]; do
    case "$1" in
        --domain) CLI_DOMAIN="$2"; shift 2 ;;
        --port)   CLI_PORT="$2"; shift 2 ;;
        *) ARGS+=("$1"); shift ;;
    esac
done
ACTION="${ARGS[0]:-install}"

case "$ACTION" in
    install)   do_install ;;
    domain)    do_domain ;;
    status)    do_status ;;
    uninstall) do_uninstall ;;
    *) echo "用法: $0 [install|domain|status|uninstall] [--domain 域名] [--port 端口]" ;;
esac

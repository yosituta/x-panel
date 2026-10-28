#!/bin/bash
# ============================================================
# SublinkPro 订阅系统 安装/管理脚本
#   用法：bash sublinkpro.sh [install|status|uninstall|update]
#   由 x-ui 菜单 25 或面板【订阅转换】按钮调用
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

need_root() { [ "$(id -u)" -eq 0 ] || { echo -e "${RED}请用 root 运行${PLAIN}"; exit 1; }; }

find_cert() {
    local c
    c=$(ls -1 /etc/letsencrypt/live/*/fullchain.pem 2>/dev/null | head -1)
    [ -z "$c" ] && c=$(ls -1 /root/cert/*/fullchain.pem 2>/dev/null | head -1)
    echo "$c"
}

cert_domain() {
    local c="$1"
    basename "$(dirname "$c")"
}

panel_port() {
    if [ -f /etc/x-ui/x-ui.db ] && command -v sqlite3 >/dev/null 2>&1; then
        sqlite3 /etc/x-ui/x-ui.db "select value from settings where key='webPort' limit 1" 2>/dev/null
    fi
}

allow_port() {
    local p=$1
    if command -v ufw >/dev/null 2>&1; then
        ufw allow "$p"/tcp >/dev/null 2>&1 && echo -e "  ${GREEN}ufw 已放行 $p/tcp${PLAIN}"
    fi
    if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
        firewall-cmd --add-port="$p"/tcp --permanent >/dev/null 2>&1
        firewall-cmd --reload >/dev/null 2>&1
        echo -e "  ${GREEN}firewalld 已放行 $p/tcp${PLAIN}"
    fi
}

write_nginx() {
    local cert="$1"
    local domain="$2"
    local key="${cert%fullchain.pem}privkey.pem"
    cat > /etc/nginx/sites-available/sublinkpro <<EOF
server {
    listen ${PORT} ssl http2;
    server_name ${domain};
    ssl_certificate     ${cert};
    ssl_certificate_key ${key};
    ssl_protocols TLSv1.2 TLSv1.3;
    client_max_body_size 20m;

    location / {
        proxy_pass http://127.0.0.1:${BACKEND};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 300s;
    }
}
EOF
    ln -sf /etc/nginx/sites-available/sublinkpro /etc/nginx/sites-enabled/sublinkpro
    nginx -t >/dev/null 2>&1 && systemctl reload nginx
}

do_install() {
    need_root
    echo -e "${GREEN}==== 安装 SublinkPro 订阅系统 ====${PLAIN}"

    echo "---- 1/5 下载 ----"
    mkdir -p "$APP_DIR"
    curl -fL --retry 3 -o "$BIN" \
        "https://github.com/${REPO}/releases/download/${VER}/sublinkPro-linux-amd64" || {
        echo -e "${RED}下载失败，检查网络${PLAIN}"; return 1; }
    chmod +x "$BIN"
    echo "  $(ls -lh "$BIN" | awk '{print $5}')"

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
    sleep 5
    echo "  服务状态: $(systemctl is-active $SERVICE)"

    echo "---- 3/5 反向代理 ----"
    local cert domain self_cert=""
    cert=$(find_cert)
    if [ -n "$cert" ]; then
        domain=$(cert_domain "$cert")
        write_nginx "$cert" "$domain"
        echo "  已用面板证书: $cert（域名 $domain）"
    else
        # 无域名/无证书：生成自签证书，端口与有域名时完全一致（15268），
        # 这样两种安装方式访问地址格式相同，后续补上域名会自动换成正签证书。
        local ip
        ip=$(curl -s --max-time 5 https://api.ipify.org 2>/dev/null)
        [ -z "$ip" ] && ip=$(hostname -I 2>/dev/null | awk '{print $1}')
        [ -z "$ip" ] && ip=127.0.0.1
        local sdir=/etc/ssl/sublinkpro
        mkdir -p "$sdir"
        if [ ! -s "$sdir/fullchain.pem" ]; then
            openssl req -x509 -nodes -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 \
                -keyout "$sdir/privkey.pem" -out "$sdir/fullchain.pem" -days 3650 \
                -subj "/CN=${ip}" >/dev/null 2>&1
        fi
        if [ -s "$sdir/fullchain.pem" ]; then
            write_nginx "$sdir/fullchain.pem" "$ip"
            self_cert="$sdir/fullchain.pem"
            echo -e "  ${YELLOW}未找到面板证书 → 已生成自签证书，仍用 https://${ip}:${PORT}${PLAIN}"
            echo -e "  ${YELLOW}浏览器会提示"不安全"，点"继续访问"即可；补上域名并申请证书后会自动换成正式证书${PLAIN}"
        else
            echo -e "  ${YELLOW}未能生成证书，退回 http://IP:${BACKEND} 直连${PLAIN}"
            allow_port "$BACKEND"
        fi
    fi

    echo "---- 4/5 防火墙 ----"
    allow_port "$PORT"

    echo "---- 5/5 验证 ----"
    sleep 2
    local code
    code=$(curl -sk -o /dev/null -w "%{http_code}" --max-time 6 "https://127.0.0.1:${PORT}/" 2>/dev/null)
    [ "$code" = "000" ] && code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 6 "http://127.0.0.1:${BACKEND}/" 2>/dev/null)

    local ip domain2
    ip=$(curl -s --max-time 5 https://api.ipify.org 2>/dev/null)
    domain2=$( [ -n "$cert" ] && cert_domain "$cert" )

    echo ""
    echo -e "${GREEN}========================================${PLAIN}"
    echo -e "${GREEN} SublinkPro 订阅系统安装完成${PLAIN}"
    echo -e "${GREEN}========================================${PLAIN}"
    if [ -n "$domain2" ]; then
        echo -e "  访问地址 : ${YELLOW}https://${domain2}:${PORT}/${PLAIN}"
    elif [ -n "$self_cert" ]; then
        echo -e "  访问地址 : ${YELLOW}https://${ip}:${PORT}/${PLAIN}  （自签证书，浏览器点"继续"）"
    else
        echo -e "  访问地址 : ${YELLOW}http://${ip}:${BACKEND}/${PLAIN}"
    fi
    echo "  默认账号 : admin"
    echo "  默认密码 : 123456  （登录后请立即修改）"
    echo "  服务名   : ${SERVICE}   日志: journalctl -u ${SERVICE} -f"
    echo ""
    echo "  用法：登录后添加各台服务器的节点链接，创建订阅并勾选节点，"
    echo "       即可得到一条包含全部节点的订阅链接。"
    if [ -n "$self_cert" ]; then
        echo ""
        echo -e "  ${YELLOW}补域名后：x-ui → 18 → 1 申请证书（15 天自动续签），证书装好后订阅系统会自动切到正式证书${PLAIN}"
    fi
    echo ""
}

do_status() {
    need_root
    echo "服务: $(systemctl is-active $SERVICE 2>/dev/null)"
    echo "端口: $(ss -tln 2>/dev/null | grep -c ":$BACKEND")  (8000=后端, ${PORT}=HTTPS)"
    command -v ufw >/dev/null && ufw status 2>/dev/null | grep "$PORT" | head -1
}

do_uninstall() {
    need_root
    systemctl disable --now "$SERVICE" >/dev/null 2>&1
    rm -f "$SVC_FILE" /etc/nginx/sites-enabled/sublinkpro /etc/nginx/sites-available/sublinkpro
    systemctl daemon-reload
    nginx -t >/dev/null 2>&1 && systemctl reload nginx
    rm -rf "$APP_DIR"
    command -v ufw >/dev/null && ufw delete allow "$PORT"/tcp >/dev/null 2>&1
    echo "已卸载 SublinkPro"
}

case "${1:-install}" in
    install) do_install ;;
    status) do_status ;;
    uninstall) do_uninstall ;;
    update) do_install ;;
    *) echo "用法: $0 [install|status|uninstall|update]" ;;
esac

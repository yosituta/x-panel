#!/bin/sh
# ============================================================
# 容器启动入口
#   首次启动按环境变量初始化面板账号/端口/路径，然后前台运行
# ============================================================
set -e

PORT="${XUI_PORT:-54321}"
USER_NAME="${XUI_USERNAME:-admin}"
PASS_WORD="${XUI_PASSWORD:-123456}"
BASE_PATH="${XUI_WEB_BASE_PATH:-/}"

mkdir -p /etc/x-ui /root/cert

if [ ! -f /etc/x-ui/x-ui.db ]; then
    echo "[x-panel] 首次启动，初始化设置：端口 ${PORT} / 账号 ${USER_NAME} / 路径 ${BASE_PATH}"
    /app/x-ui setting -port "${PORT}" >/dev/null 2>&1 || true
    /app/x-ui setting -username "${USER_NAME}" -password "${PASS_WORD}" >/dev/null 2>&1 || true
    /app/x-ui setting -webBasePath "${BASE_PATH}" >/dev/null 2>&1 || true
fi

# 证书挂载进来后自动绑定（/root/cert/<域名>/fullchain.pem）
if [ -n "${XUI_CERT_DOMAIN:-}" ] && [ -s "/root/cert/${XUI_CERT_DOMAIN}/fullchain.pem" ]; then
    /app/x-ui setting \
        -webCert "/root/cert/${XUI_CERT_DOMAIN}/fullchain.pem" \
        -webCertKey "/root/cert/${XUI_CERT_DOMAIN}/privkey.pem" >/dev/null 2>&1 || true
    echo "[x-panel] 已绑定证书: ${XUI_CERT_DOMAIN}"
fi

if [ "${PASS_WORD}" = "123456" ]; then
    echo "[x-panel] 提醒：当前密码为默认值 123456，登录后请立即修改"
fi

exec /app/x-ui

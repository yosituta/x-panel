#!/bin/bash
# ==========================================================
# acme 续期钩子：把正式证书自动挂到面板上
#   - 面板无证书        → 挂上
#   - 面板是自签证书    → 换成正式证书（自签本该只在应急时用）
#   - 面板已是别的正式证书 → 不动（避免覆盖用户手动选择的证书）
# 由 install.sh 写入 acme 的 --reloadcmd，续期成功后自动执行。
# ==========================================================
DOMAIN="${1:-}"
CERT="/root/cert/${DOMAIN}/fullchain.pem"
KEY="/root/cert/${DOMAIN}/privkey.pem"
XUI_BIN="/usr/local/x-ui/x-ui"

[ -n "$DOMAIN" ] || exit 0
[ -s "$CERT" ] && [ -s "$KEY" ] || exit 0
[ -x "$XUI_BIN" ] || exit 0

CUR=$("$XUI_BIN" setting -getCert true 2>/dev/null | grep -Eo 'cert: .+' | awk '{print $2}')

# 已经是这张：只需重启让新证书生效
if [ -n "$CUR" ] && [ "$CUR" = "$CERT" ]; then
    systemctl restart x-ui >/dev/null 2>&1
    exit 0
fi

# 无证书 或 自签 → 挂上
CUR_SELF=0
if [ -n "$CUR" ] && [ -s "$CUR" ]; then
    iss=$(openssl x509 -in "$CUR" -noout -issuer 2>/dev/null | sed 's/issuer=//')
    sub=$(openssl x509 -in "$CUR" -noout -subject 2>/dev/null | sed 's/subject=//')
    [ -n "$iss" ] && [ "$iss" = "$sub" ] && CUR_SELF=1
fi

if [ -z "$CUR" ] || [ "$CUR_SELF" = "1" ]; then
    "$XUI_BIN" cert -webCert "$CERT" -webCertKey "$KEY" >/dev/null 2>&1
    systemctl restart x-ui >/dev/null 2>&1
fi

exit 0

#!/bin/bash
# ============================================================
# Cloudflare 源站证书（Origin CA）管理
#
#   Origin 证书特点：
#     - 有效期最长 15 年，不用续签
#     - 只被 Cloudflare 信任：域名必须开橙云（Proxied）
#     - 直接用 IP / 灰云访问时浏览器会报不受信任
#     - 回源端口必须是 CF 支持的端口：443 2053 2083 2087 2096 8443
#
#   用法：
#     cfcert.sh paste  <域名> [--target panel|sub]      # 粘贴卡台/控制台里生成的证书
#     cfcert.sh api    <域名> [--target panel|sub]      # 用 API 自动签发（需 CF_API_TOKEN 或 CF_ORIGIN_KEY）
#     cfcert.sh self   <域名>                           # 自签证书（无 CF 时兜底）
#     cfcert.sh status [域名]
# ============================================================

GREEN='\033[0;32m'; YELLOW='\033[0;33m'; RED='\033[0;31m'; PLAIN='\033[0m'
CERT_DIR=/root/cert
CF_CONF=/root/.cf-originca
SERVICE_NAME=""
detect_service() {
    for s in x-ui x-panel; do
        [ -f "/etc/systemd/system/${s}.service" ] && SERVICE_NAME="$s" && return 0
    done
    SERVICE_NAME="x-ui"
    return 1
}

need_root() { [ "$(id -u)" -eq 0 ] || { echo -e "${RED}请用 root 运行${PLAIN}"; exit 1; }; }

valid_port() {
    case "$1" in
        443|2053|2083|2087|2096|8443) return 0 ;;
        *) return 1 ;;
    esac
}

panel_port() {
    local p
    p=$(/usr/local/x-ui/x-ui setting -show true 2>/dev/null || /opt/x-panel/x-ui setting -show true 2>/dev/null) \
        | grep -Eo 'port（端口号）: [0-9]+' | awk '{print $2}'
    echo "${p:-54321}"
}

warn_port() {
    local p=$1
    if ! valid_port "$p"; then
        echo -e "${YELLOW}⚠ 当前面板端口 ${p} 不在 Cloudflare 支持的端口内${PLAIN}"
        echo -e "${YELLOW}  CF 回源端口只能是：443 / 2053 / 2083 / 2087 / 2096 / 8443${PLAIN}"
        echo -e "${YELLOW}  开橙云后浏览器会连不上，请先把面板改到 8443 或 2096 之类${PLAIN}"
        return 1
    fi
    return 0
}

# 校验证书与私钥是否匹配、域名是否覆盖
validate_pair() {
    local cert=$1 key=$2 domain=$3
    openssl x509 -in "$cert" -noout >/dev/null 2>&1 || { echo -e "${RED}证书文件无效${PLAIN}"; return 1; }
    local pcert pkey
    pcert=$(openssl x509 -in "$cert" -noout -pubkey 2>/dev/null | openssl md5)
    pkey=$(openssl pkey -in "$key" -pubout 2>/dev/null | openssl md5)
    [ "$pcert" = "$pkey" ] || { echo -e "${RED}证书与私钥不匹配${PLAIN}"; return 1; }
    if ! openssl x509 -in "$cert" -noout -text | grep -q "$domain"; then
        echo -e "${YELLOW}⚠ 证书里没有找到域名 ${domain}，请确认申请的域名一致${PLAIN}"
    fi
    openssl x509 -in "$cert" -noout -issuer -dates | sed 's/^/  /'
    return 0
}

install_files() {
    local domain=$1 cert=$2 key=$3
    mkdir -p "${CERT_DIR}/${domain}"
    cp -f "$cert" "${CERT_DIR}/${domain}/fullchain.pem"
    cp -f "$key"  "${CERT_DIR}/${domain}/privkey.pem"
    chmod 600 "${CERT_DIR}/${domain}/privkey.pem"
    echo "  已安装到 ${CERT_DIR}/${domain}/"
}

bind_panel() {
    local domain=$1
    local bin=""
    for b in /usr/local/x-ui/x-ui /opt/x-panel/x-ui; do
        [ -x "$b" ] && bin="$b" && break
    done
    [ -z "$bin" ] && { echo -e "${RED}未找到面板二进制${PLAIN}"; return 1; }

    "$bin" setting -webCert "${CERT_DIR}/${domain}/fullchain.pem" \
                   -webCertKey "${CERT_DIR}/${domain}/privkey.pem" >/dev/null 2>&1 || {
        echo -e "${RED}绑定面板证书失败${PLAIN}"; return 1; }
    detect_service
    systemctl restart "${SERVICE_NAME}" >/dev/null 2>&1
    sleep 3
    echo "  面板已重启，证书生效（服务：${SERVICE_NAME}）"
}

bind_sub() {
    local domain=$1
    local script=""
    for s in /usr/local/x-ui/scripts/sublinkpro.sh /opt/x-panel/scripts/sublinkpro.sh; do
        [ -f "$s" ] && script="$s" && break
    done
    if [ -z "$script" ]; then
        echo -e "${YELLOW}未安装订阅系统，跳过 nginx 绑定${PLAIN}"
        return 0
    fi
    # 订阅系统按 /root/cert/<域名>/ 取证书，重跑安装即会重新生成 nginx 配置
    bash "$script" install --domain "$domain" >/dev/null 2>&1 && echo "  订阅系统 nginx 已切换到该证书"
}

do_paste() {
    local domain=$1 target=$2
    echo -e "${GREEN}==== 粘贴 Cloudflare 源站证书：${domain} ====${PLAIN}"
    echo "在 Cloudflare 控制台：SSL/TLS → Origin Server → Create Certificate"
    echo "  1) 填入域名 ${domain}（有效期选 15 年）"
    echo "  2) 复制 Origin Certificate 与 Private Key"
    echo ""

    local tmpdir; tmpdir=$(mktemp -d)
    echo -e "${YELLOW}请粘贴证书（以 -----BEGIN CERTIFICATE----- 开头，单独一行输入 END 结束）：${PLAIN}"
    : > "$tmpdir/cert.pem"
    while IFS= read -r line; do
        [ "$line" = "END" ] && break
        printf '%s\n' "$line" >> "$tmpdir/cert.pem"
    done
    echo -e "${YELLOW}请粘贴私钥（以 -----BEGIN PRIVATE KEY----- 开头，单独一行输入 END 结束）：${PLAIN}"
    : > "$tmpdir/key.pem"
    while IFS= read -r line; do
        [ "$line" = "END" ] && break
        printf '%s\n' "$line" >> "$tmpdir/key.pem"
    done

    validate_pair "$tmpdir/cert.pem" "$tmpdir/key.pem" "$domain" || { rm -rf "$tmpdir"; return 1; }
    install_files "$domain" "$tmpdir/cert.pem" "$tmpdir/key.pem"
    rm -rf "$tmpdir"

    case "$target" in
        sub) bind_sub "$domain" ;;
        *)   bind_panel "$domain" ;;
    esac
}

do_api() {
    local domain=$1 target=$2
    echo -e "${GREEN}==== API 申请 Cloudflare 源站证书：${domain} ====${PLAIN}"

    local token="${CF_API_TOKEN:-}" okey="${CF_ORIGIN_KEY:-}"
    [ -f "$CF_CONF" ] && . "$CF_CONF"

    if [ -z "$token" ] && [ -z "$okey" ]; then
        echo -e "${YELLOW}需要 Cloudflare 凭据，二选一：${PLAIN}"
        echo "  A) Origin CA Key（控制台 SSL/TLS → Origin Server → 查看 Origin CA Key）"
        echo "  B) API Token，权限：Zone → SSL and Certificates → Edit"
        echo ""
        read -rp "请选择 [A/B，回车跳过]：" choice
        case "$choice" in
            A|a) read -rp "粘贴 Origin CA Key：" okey ;;
            B|b) read -rp "粘贴 API Token：" token ;;
            *) echo "已取消"; return 1 ;;
        esac
        umask 077
        { [ -n "$okey" ] && echo "CF_ORIGIN_KEY=$okey"; [ -n "$token" ] && echo "CF_API_TOKEN=$token"; } > "$CF_CONF"
        umask 022
        echo "  凭据已保存到 $CF_CONF（600 权限）"
    fi

    local tmpdir; tmpdir=$(mktemp -d)
    openssl req -new -newkey rsa:2048 -nodes -keyout "$tmpdir/key.pem" \
        -out "$tmpdir/req.csr" -subj "/CN=${domain}" >/dev/null 2>&1 || {
        echo -e "${RED}生成 CSR 失败${PLAIN}"; rm -rf "$tmpdir"; return 1; }

    local payload resp
    payload=$(python3 - "$domain" "$tmpdir/req.csr" <<'PY'
import json, sys
domain, csr_path = sys.argv[1], sys.argv[2]
csr = open(csr_path).read()
print(json.dumps({
    "hostnames": [domain],
    "requested_validity": 5475,   # 15 年
    "request_type": "origin-rsa",
    "csr": csr,
}))
PY
)

    local url="https://api.cloudflare.com/client/v4/certificates"
    if [ -n "$token" ]; then
        resp=$(curl -sS --max-time 40 -X POST "$url" \
            -H "Authorization: Bearer ${token}" \
            -H "Content-Type: application/json" --data "$payload" 2>&1)
    else
        resp=$(curl -sS --max-time 40 -X POST "$url" \
            -H "X-Auth-User-Service-Key: ${okey}" \
            -H "Content-Type: application/json" --data "$payload" 2>&1)
    fi

    local cert
    cert=$(printf '%s' "$resp" | python3 -c "
import json,sys
try:
    d=json.load(sys.stdin)
except Exception:
    print(''); sys.exit(0)
if not d.get('success'):
    errs=d.get('errors') or []
    print('', file=sys.stderr)
    for e in errs: print(e.get('message',''), file=sys.stderr)
    sys.exit(0)
print(d['result']['certificate'])
" 2>"$tmpdir/err.txt")

    if [ -z "$cert" ]; then
        echo -e "${RED}申请失败：${PLAIN}"
        cat "$tmpdir/err.txt" 2>/dev/null | sed 's/^/  /'
        echo "  原始响应：$(printf '%s' "$resp" | head -c 300)"
        rm -rf "$tmpdir"
        return 1
    fi

    printf '%s\n' "$cert" > "$tmpdir/cert.pem"
    validate_pair "$tmpdir/cert.pem" "$tmpdir/key.pem" "$domain" || { rm -rf "$tmpdir"; return 1; }
    install_files "$domain" "$tmpdir/cert.pem" "$tmpdir/key.pem"
    rm -rf "$tmpdir"

    case "$target" in
        sub) bind_sub "$domain" ;;
        *)   bind_panel "$domain" ;;
    esac
}

do_self() {
    local domain=$1
    mkdir -p "${CERT_DIR}/${domain}"
    openssl req -x509 -nodes -newkey rsa:2048 \
        -keyout "${CERT_DIR}/${domain}/privkey.pem" \
        -out "${CERT_DIR}/${domain}/fullchain.pem" \
        -days 3650 -subj "/CN=${domain}" >/dev/null 2>&1
    chmod 600 "${CERT_DIR}/${domain}/privkey.pem"
    echo "  自签证书已生成（10 年，浏览器会提示不受信任）"
}

do_status() {
    local d="${1:-}"
    if [ -n "$d" ]; then
        [ -f "${CERT_DIR}/${d}/fullchain.pem" ] || { echo "未找到 ${d} 的证书"; return 1; }
        openssl x509 -in "${CERT_DIR}/${d}/fullchain.pem" -noout -subject -issuer -dates | sed 's/^/  /'
    else
        echo "证书目录：${CERT_DIR}"
        ls -1 "${CERT_DIR}" 2>/dev/null | sed 's/^/  /'
    fi
}

need_root
ACTION="${1:-}"; shift || true
DOMAIN=""; TARGET="panel"
while [ $# -gt 0 ]; do
    case "$1" in
        --target) TARGET="$2"; shift 2 ;;
        *) [ -z "$DOMAIN" ] && DOMAIN="$1"; shift ;;
    esac
done

case "$ACTION" in
    paste) do_paste "$DOMAIN" "$TARGET" ;;
    api)   do_api   "$DOMAIN" "$TARGET" ;;
    self)  do_self  "$DOMAIN" ;;
    status) do_status "$DOMAIN" ;;
    *)
        echo "用法："
        echo "  $0 paste <域名> [--target panel|sub]   粘贴控制台生成的证书"
        echo "  $0 api   <域名> [--target panel|sub]   用 CF 凭据自动签发（15 年）"
        echo "  $0 self  <域名>                       自签兜底"
        echo "  $0 status [域名]"
        ;;
esac

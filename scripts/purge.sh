#!/usr/bin/env bash
# ==========================================================
# X-Panel 彻底卸载（含数据库、证书、订阅系统、acme、日志）
# 用法：
#   bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/scripts/purge.sh)
#   bash purge.sh --check     # 只体检，不删除
# ==========================================================
set -uo pipefail

red='\033[0;31m'; green='\033[0;32m'; yellow='\033[0;33m'; plain='\033[0m'
CHECK=0
[[ "${1:-}" == "--check" ]] && CHECK=1

TARGETS=(
    "/usr/local/x-ui"
    "/opt/x-panel"
    "/etc/x-ui"
    "/opt/sublinkpro"
    "/root/cert"
    "/root/.acme.sh"
    "/etc/letsencrypt"
    "/etc/ssl/sublinkpro"
    "/usr/bin/x-ui"
    "/usr/local/bin/x-ui"
    "/etc/systemd/system/x-ui.service"
    "/etc/systemd/system/x-panel.service"
    "/etc/systemd/system/3x-ui.service"
    "/etc/systemd/system/sublinkpro.service"
    "/etc/systemd/system/multi-user.target.wants/x-ui.service"
    "/etc/systemd/system/multi-user.target.wants/x-panel.service"
    "/etc/systemd/system/multi-user.target.wants/sublinkpro.service"
    "/etc/nginx/sites-enabled/sublinkpro.conf"
    "/etc/nginx/sites-available/sublinkpro.conf"
    "/etc/nginx/conf.d/sublinkpro.conf"
    "/var/log/x-ui.log"
    "/var/log/x-ui"
)

# 安装包与临时文件（通配匹配）
for f in /usr/local/x-ui-linux-*.tar.gz /usr/local/x-ui-linux-*.tar.gz.* /tmp/x-ui*.log /tmp/xp-*.log; do
    [ -e "$f" ] && TARGETS+=("$f")
done

found=()
for t in "${TARGETS[@]}"; do
    if [ -e "$t" ]; then
        size=$(du -sh "$t" 2>/dev/null | awk '{print $1}')
        found+=("$t|${size:-?}")
    fi
done

echo "=============================="
echo " X-Panel 彻底卸载体检"
echo "=============================="
if [ ${#found[@]} -eq 0 ]; then
    echo " 没有发现任何 X-Panel 残留。"
else
    for row in "${found[@]}"; do
        printf "  [待删] %-58s %s\n" "${row%%|*}" "${row##*|}"
    done
fi

# 服务
for s in x-ui x-panel 3x-ui sublinkpro; do
    if systemctl list-unit-files 2>/dev/null | grep -q "^${s}\.service"; then
        echo "  [待停] 服务 ${s}.service"
    fi
done

# 进程
leftproc=$(pgrep -af "^(/usr/local/x-ui/|/opt/x-panel/|/opt/sublinkpro)" 2>/dev/null || true)
[ -n "$leftproc" ] && echo "  [待杀] 进程: $(echo "$leftproc" | awk '{print $1}' | tr '\n' ' ')"

# 防火墙
if command -v ufw >/dev/null 2>&1; then
    while IFS= read -r line; do
        [ -n "$line" ] && echo "  [待删] ufw 规则: $line"
    done < <(ufw status 2>/dev/null | grep -E "^(54321|15268|8000|8443)" || true)
fi

# cron / acme 续期
if crontab -l 2>/dev/null | grep -q "acme.sh\|x-ui"; then
    echo "  [待删] crontab 中的 acme/x-ui 条目"
fi

if [ "$CHECK" = "1" ]; then
    echo
    echo "（--check 模式：未做任何修改）"
    echo "执行删除：bash purge.sh"
    exit 0
fi

echo
echo -e "${red}这会删除上面列出的全部内容，不可恢复（含面板数据库、所有节点配置、证书）。${plain}"
read -rp "确认请输入 PURGE 后回车：" ans
[[ "$ans" != "PURGE" ]] && { echo "已取消"; exit 0; }

echo
echo "---- 1. 停止并禁用服务 ----"
for s in x-ui x-panel 3x-ui sublinkpro; do
    systemctl stop "$s" >/dev/null 2>&1 && echo "  stopped: $s"
    systemctl disable "$s" >/dev/null 2>&1
done
systemctl daemon-reload >/dev/null 2>&1
systemctl reset-failed >/dev/null 2>&1

echo "---- 2. 结束残留进程 ----"
for pid in $(pgrep -f "^(/usr/local/x-ui/|/opt/x-panel/|/opt/sublinkpro)" 2>/dev/null || true); do
    kill "$pid" 2>/dev/null && echo "  killed: $pid"
done
sleep 1

echo "---- 3. 删除文件与目录 ----"
for t in "${TARGETS[@]}"; do
    if [ -e "$t" ]; then
        rm -rf "$t" && echo "  removed: $t"
    fi
done

echo "---- 4. 撤销防火墙规则 ----"
if command -v ufw >/dev/null 2>&1; then
    for p in 54321 15268 8000 8443; do
        ufw delete allow "${p}/tcp" >/dev/null 2>&1 && echo "  ufw 已撤销 ${p}/tcp"
    done
fi
if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
    for p in 54321 15268 8000 8443; do
        firewall-cmd --remove-port="${p}/tcp" --permanent >/dev/null 2>&1
    done
    firewall-cmd --reload >/dev/null 2>&1 && echo "  firewalld 已更新"
fi

echo "---- 5. 清理 acme 续期 cron ----"
if crontab -l 2>/dev/null | grep -q "acme.sh"; then
    crontab -l 2>/dev/null | grep -v "acme.sh" | crontab - && echo "  已移除 acme 续期任务"
fi
rm -f /etc/cron.d/acme /etc/cron.daily/acme.sh 2>/dev/null || true

echo "---- 6. nginx：只清订阅反代，不动你其它站点 ----"
if command -v nginx >/dev/null 2>&1; then
    nginx -t >/dev/null 2>&1 && systemctl reload nginx >/dev/null 2>&1 && echo "  nginx 已重载"
fi

echo
echo "---- 7. 校验 ----"
left=0
for t in "${TARGETS[@]}"; do
    [ -e "$t" ] && { echo -e "  ${red}仍存在${plain}: $t"; left=$((left+1)); }
done
for s in x-ui x-panel 3x-ui sublinkpro; do
    if systemctl is-active "$s" >/dev/null 2>&1; then
        echo -e "  ${red}服务仍在运行${plain}: $s"; left=$((left+1))
    fi
done
p=$(pgrep -f "^(/usr/local/x-ui/|/opt/x-panel/|/opt/sublinkpro)" 2>/dev/null || true)
[ -n "$p" ] && { echo -e "  ${red}仍有进程${plain}: $p"; left=$((left+1)); }

echo
if [ "$left" -eq 0 ]; then
    echo -e "${green}彻底卸载完成，没有残留。${plain}"
else
    echo -e "${yellow}还有 $left 项未清掉，见上方标记。${plain}"
fi
echo
echo "重装：bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh)"

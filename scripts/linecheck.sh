#!/usr/bin/env bash
# X-Panel 开源分支：线路 / IP 质量检测
# 用法：bash linecheck.sh [目标域名或IP]
# 输出：出口信息、到各地延迟与丢包、回程路由（有 nexttrace/mtr 时）、IP 风险提示

set -uo pipefail
target="${1:-}"
red='\033[0;31m'; green='\033[0;32m'; yellow='\033[0;33m'; blue='\033[0;36m'; plain='\033[0m'

line() { echo -e "${blue}————————————————————————————————${plain}"; }
title() { echo; line; echo -e "${green}$*${plain}"; line; }

need_cmd() { command -v "$1" >/dev/null 2>&1; }

# ---------- 1. 出口信息 ----------
title "① 出口信息"
ipinfo=$(curl -fsS --max-time 8 https://ipinfo.io/json 2>/dev/null || true)
if [[ -n "$ipinfo" ]]; then
  extract() { echo "$ipinfo" | grep -oE "\"$1\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" | head -1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/'; }
  printf "%-7s: %s\n" "IP" "$(extract ip)"
  for k in city region country org; do
    v=$(extract "$k")
    [[ -n "$v" ]] && printf "%-7s: %s\n" "$k" "$v"
  done
  ip=$(extract ip)
else
  echo -e "${yellow}ipinfo.io 不可达，跳过${plain}"
  ip=""
fi

# IP 类型（原生/广播/机房/家宽）粗判
if [[ -n "$ip" ]]; then
  asn=$(curl -fsS --max-time 8 "https://api.ip.sb/geoip/$ip" 2>/dev/null || true)
  if [[ -n "$asn" ]]; then
    echo "$asn" | grep -oE '"asn_organization": *[^,]*' | sed 's/"asn_organization": *//;s/"//g' | sed 's/^/ASN    : /' || true
    echo "$asn" | grep -oE '"isp": *"[^"]*"' | sed -E 's/"isp": *"([^"]*)"/ISP    : \1/' || true
    echo "$asn" | grep -oE '"country": *"[^"]*"' | sed -E 's/"country": *"([^"]*)"/国家   : \1/' || true
  fi
fi

# ---------- 2. 延迟与丢包 ----------
title "② 延迟 / 丢包（10 次）"
ping_test() {
  local name="$1" host="$2"
  if ! need_cmd ping; then echo "缺少 ping"; return; fi
  local out
  out=$(ping -c 10 -W 2 "$host" 2>/dev/null)
  if [[ -z "$out" ]]; then printf "%-18s %s\n" "$name" "❌ 不通"; return; fi
  local loss avg
  loss=$(echo "$out" | grep -oE '[0-9]+(\.[0-9]+)?% packet loss' | head -1 | grep -oE '^[0-9.]+')
  avg=$(echo "$out" | tail -1 | awk -F'/' '{print $5}')
  local mark="🟢"
  [[ -z "$loss" ]] && loss=100
  awk -v l="$loss" 'BEGIN{exit !(l>=5)}' && mark="🟡"
  awk -v l="$loss" 'BEGIN{exit !(l>=20)}' && mark="🔴"
  printf "%-18s %s 丢包 %5s%%   平均 %sms\n" "$name" "$mark" "${loss:-100}" "${avg:-N/A}"
}
ping_test "Cloudflare" "1.1.1.1"
ping_test "Google DNS" "8.8.8.8"
ping_test "阿里 DNS" "223.5.5.5"
ping_test "腾讯 DNS" "119.29.29.29"
[[ -n "$target" ]] && ping_test "自定义: $target" "$target"

# ---------- 3. 回程路由 ----------
title "③ 回程路由"
if need_cmd nexttrace; then
  out=$(nexttrace -M -n 1.1.1.1 2>/dev/null)
  if [[ -n "$out" ]]; then echo "$out" | sed -n '1,20p'; else echo "nexttrace 执行失败"; fi
elif need_cmd mtr; then
  out=$(mtr -r -c 5 -n 1.1.1.1 2>/dev/null)
  if [[ -n "$out" ]]; then echo "$out" | sed -n '1,15p'; else echo "mtr 执行失败（可尝试 traceroute）"; fi
else
  echo -e "${yellow}未安装 nexttrace/mtr，可执行：${plain}"
  echo "  apt install -y mtr        # 或"
  echo "  bash -c \"\$(curl -Ls https://raw.githubusercontent.com/nxtrace/NTrace-core/main/install.sh)\""
  echo
  echo "退化方案：用 traceroute"
  need_cmd traceroute && traceroute -n -m 15 1.1.1.1 2>/dev/null | head -16 || echo "也未安装 traceroute"
fi

# ---------- 4. 带宽粗测 ----------
title "④ 出口带宽粗测"
speed=$(curl -fsS --max-time 20 -o /dev/null -w '%{speed_download}' https://speed.cloudflare.com/__down?bytes=20000000 2>/dev/null || echo 0)
if [[ "${speed%.*}" -gt 0 ]] 2>/dev/null; then
  echo -e "Cloudflare 单线程下载：${yellow}$(awk -v s="$speed" 'BEGIN{printf "%.2f", s*8/1000000}')${plain} Mbps"
else
  echo -e "${yellow}测速失败（可能是网络受限）${plain}"
fi

title "提示"
cat <<'EOF'
· 丢包 5% 以上、延迟 200ms 以上通常体感明显；
· 机房 IP（ISP 显示 IDC/Hosting）做代理更容易被识别，家宽/原生 IP 更稳；
· 同一 IP + 同一端口多省漫游是最典型的"机场特征"，建议多开入站分散。
EOF

#!/usr/bin/env bash
# X-Panel 开源分支：地区 DNS 检测
# 用法：bash dnscheck.sh [域名]
# 对比国内外多个公共 DNS 的解析结果，判断是否存在地区性劫持/污染

set -uo pipefail
domain="${1:-www.google.com}"
red='\033[0;31m'; green='\033[0;32m'; yellow='\033[0;33m'; blue='\033[0;36m'; plain='\033[0m'

command -v dig >/dev/null 2>&1 || { echo "需要 dig：apt install -y dnsutils"; exit 1; }

# 名称|地址|地区
SERVERS="
阿里DNS|223.5.5.5|中国
腾讯DNS|119.29.29.29|中国
114DNS|114.114.114.114|中国
百度DNS|180.76.76.76|中国
Cloudflare|1.1.1.1|海外
Google|8.8.8.8|海外
Quad9|9.9.9.9|海外
OpenDNS|208.67.222.222|海外
"

echo -e "${blue}————————————————————————————————${plain}"
echo -e "${green}DNS 检测：$domain${plain}"
echo -e "${blue}————————————————————————————————${plain}"

declare -A results
declare -A times
order=()

while IFS='|' read -r name addr region; do
  [[ -z "${name:-}" ]] && continue
  start=$(date +%s%N)
  # +timeout=3 +tries=1 避免长时间卡住
  ans=$(dig +short +timeout=3 +tries=1 @"$addr" "$domain" A 2>/dev/null | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' | sort | tr '\n' ' ')
  end=$(date +%s%N)
  ms=$(( (end - start) / 1000000 ))
  results["$name"]="$ans"
  times["$name"]=$ms
  order+=("$name|$addr|$region")
done <<< "$SERVERS"

# 输出
printf "%-12s %-16s %-6s %-6s %s\n" "DNS" "地址" "地区" "耗时" "解析结果"
printf "%s\n" "----------------------------------------------------------------------------"
for item in "${order[@]}"; do
  IFS='|' read -r name addr region <<< "$item"
  ans="${results[$name]:-}"
  ms="${times[$name]:-0}"
  if [[ -z "$ans" ]]; then
    printf "%-12s %-16s %-6s %-6s ${red}%s${plain}\n" "$name" "$addr" "$region" "${ms}ms" "无解析/超时"
  else
    printf "%-12s %-16s %-6s %-6s ${green}%s${plain}\n" "$name" "$addr" "$region" "${ms}ms" "$ans"
  fi
done

# 判定
echo
distinct=$(for item in "${order[@]}"; do
  IFS='|' read -r name _ _ <<< "$item"; echo "${results[$name]:-}"
done | tr ' ' '\n' | grep -E '^[0-9]+\.' | sort -u | wc -l)

if [[ "$distinct" -le 1 ]]; then
  echo -e "${green}✅ 各 DNS 解析一致，未发现明显的地区差异${plain}"
else
  echo -e "${yellow}⚠️  出现 $distinct 组不同结果，下面按结果分组：${plain}"
  for item in "${order[@]}"; do
    IFS='|' read -r name addr region <<< "$item"
    printf "  %s → %s\n" "$name" "${results[$name]:-无}"
  done
  cat <<'EOF'

说明：
· 国内外结果不同不一定是污染，CDN 会按地区返回不同 IP；
· 若国内 DNS 返回明显异常地址（如 0.0.0.0、127.0.0.1、Facebook/Twitter 常见污染段），
  则大概率被劫持或污染；
· 想逐跳定位可用：dig +trace 域名
EOF
fi

echo
echo -e "${blue}本机使用的 DNS：${plain}"
if command -v resolvectl >/dev/null 2>&1; then
  resolvectl status 2>/dev/null | grep -E "DNS Servers|DNS Domain" | head -4
else
  cat /etc/resolv.conf 2>/dev/null | grep -E '^nameserver' | head -5
fi

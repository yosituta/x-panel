#!/usr/bin/env bash
# X-Panel 开源分支：深度调优（BBR + FQ + TFO + 缓冲区）
# 用法：bash tune.sh [apply|status|revert]
# 全部幂等：重复执行不会叠加重复配置

set -uo pipefail
red='\033[0;31m'; green='\033[0;32m'; yellow='\033[0;33m'; blue='\033[0;36m'; plain='\033[0m'
CONF="/etc/sysctl.d/99-xpanel-tune.conf"

need_root() { [[ $EUID -eq 0 ]] || { echo -e "${red}请用 root 运行${plain}"; exit 1; }; }

show_status() {
  echo -e "${blue}————————————————————————————————${plain}"
  echo -e "${green}当前网络参数${plain}"
  echo -e "${blue}————————————————————————————————${plain}"
  printf "内核版本        : %s\n" "$(uname -r)"
  printf "拥塞控制        : %s\n" "$(sysctl -n net.ipv4.tcp_congestion_control 2>/dev/null)"
  printf "可用拥塞算法    : %s\n" "$(sysctl -n net.ipv4.tcp_available_congestion_control 2>/dev/null)"
  printf "队列规则(qdisc) : %s\n" "$(sysctl -n net.core.default_qdisc 2>/dev/null)"
  printf "TCP Fast Open   : %s\n" "$(sysctl -n net.ipv4.tcp_fastopen 2>/dev/null)"
  printf "rmem 上限       : %s\n" "$(sysctl -n net.core.rmem_max 2>/dev/null)"
  printf "wmem 上限       : %s\n" "$(sysctl -n net.core.wmem_max 2>/dev/null)"
  printf "backlog         : %s\n" "$(sysctl -n net.core.somaxconn 2>/dev/null)"
  echo
  if sysctl -n net.ipv4.tcp_congestion_control 2>/dev/null | grep -q bbr; then
    echo -e "BBR 状态        : ${green}已启用${plain}"
  else
    echo -e "BBR 状态        : ${yellow}未启用${plain}（执行 bash tune.sh apply）"
  fi
  [[ -f "$CONF" ]] && echo -e "配置文件        : ${green}$CONF 已存在${plain}" || echo "配置文件        : 尚未生成"
}

apply_tune() {
  need_root
  # 1) 内核是否支持 BBR
  if ! modprobe tcp_bbr 2>/dev/null; then
    echo -e "${yellow}⚠️  内核未内置 tcp_bbr 模块，BBR 可能不可用${plain}"
  fi
  local avail
  avail=$(sysctl -n net.ipv4.tcp_available_congestion_control 2>/dev/null)
  if ! echo "$avail" | grep -q bbr; then
    echo -e "${red}当前内核不支持 BBR（可用：$avail），建议升级到 4.9+ 内核${plain}"
  fi

  cat > "$CONF" <<'EOF'
# X-Panel 深度调优（由 scripts/tune.sh 生成，可安全删除）

# 拥塞控制：BBR + FQ 队列
net.core.default_qdisc = fq
net.ipv4.tcp_congestion_control = bbr

# TCP Fast Open（1=客户端，2=服务端，3=两者）
net.ipv4.tcp_fastopen = 3

# 缓冲区上限（高带宽高延迟线路收益明显）
net.core.rmem_max = 67108864
net.core.wmem_max = 67108864
net.ipv4.tcp_rmem = 4096 87380 67108864
net.ipv4.tcp_wmem = 4096 65536 67108864

# 连接队列
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 65535

# 复用与回收
net.ipv4.tcp_mtu_probing = 1
net.ipv4.tcp_slow_start_after_idle = 0
net.ipv4.tcp_notsent_lowat = 16384
EOF

  echo -e "${green}写入 $CONF${plain}"
  sysctl --system >/dev/null 2>&1 || sysctl -p "$CONF" >/dev/null 2>&1

  sleep 1
  echo
  if sysctl -n net.ipv4.tcp_congestion_control | grep -q bbr; then
    echo -e "${green}✅ BBR 已生效${plain}"
  else
    echo -e "${yellow}⚠️  BBR 未生效，请确认内核支持后重启系统${plain}"
  fi
  [[ "$(sysctl -n net.core.default_qdisc)" == "fq" ]] && echo -e "${green}✅ FQ 队列已生效${plain}"
  show_status
}

revert_tune() {
  need_root
  if [[ -f "$CONF" ]]; then
    rm -f "$CONF"
    sysctl --system >/dev/null 2>&1 || true
    echo -e "${green}✅ 已删除调优配置并重新加载系统默认值${plain}"
    echo -e "${yellow}注意：拥塞控制算法可能需要重启后彻底恢复${plain}"
  else
    echo "没有找到 $CONF，无需还原"
  fi
}

case "${1:-status}" in
  apply) apply_tune ;;
  revert) revert_tune ;;
  status) show_status ;;
  *) echo "用法: bash tune.sh [apply|status|revert]" ;;
esac

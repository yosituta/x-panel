#!/usr/bin/env bash
# 在无 root、无真防火墙的环境里验证 quick_open_port 的解析与调用逻辑
set -u
cd "$(dirname "$0")/.." || exit 1
SRC="x-ui.sh"

tmp=$(mktemp -d)
mkdir -p "$tmp/bin"
cat > "$tmp/bin/ufw" <<'EOF'
#!/usr/bin/env bash
case "$1" in
  status) echo "Status: active"; echo; echo "To    Action  From"; echo "--    ------  ----"; echo "22/tcp ALLOW Anywhere";;
  allow)  echo "Rule added: $2" >> "$UFW_LOG";;
  --force) echo "enable" >> "$UFW_LOG"; echo "Firewall is active and enabled";;
  *) ;;
esac
EOF
chmod +x "$tmp/bin/ufw"
export PATH="$tmp/bin:$PATH"
export UFW_LOG="$tmp/calls.log"
: > "$UFW_LOG"

# 抽取函数
awk '/^quick_open_port\(\) \{/,/^\}$/' "$SRC" > "$tmp/fn.sh"
# x-ui.sh 里的颜色变量，测试环境补上
green=$'\033[32m'; red=$'\033[31m'; yellow=$'\033[33m'; plain=$'\033[0m'
# shellcheck disable=SC1090
source "$tmp/fn.sh"

pass=0; fail=0
check() { # check <描述> <期望包含> <实际>
  local act; act=$(echo "$3" | sed -E 's/\x1b\[[0-9;]*m//g')
  if echo "$act" | grep -q -- "$2"; then echo "  PASS  $1"; pass=$((pass+1));
  else echo "  FAIL  $1"; echo "        期望包含: $2"; echo "        实际: $act"; fail=$((fail+1)); fi
}

run() { # run <输入> → 输出
  : > "$UFW_LOG"
  printf '%s\nn\n' "$1" | quick_open_port 2>&1
}

echo "=== 1. 单个端口 ==="
out=$(run "9999"); check "9999 放行 9999/tcp" "已放行 9999/tcp" "$out"
check "写入 ufw allow 9999/tcp" "Rule added: 9999/tcp" "$(cat "$UFW_LOG")"

echo "=== 2. 多个端口 ==="
out=$(run "443,8443"); check "443" "已放行 443/tcp" "$out"
check "8443" "已放行 8443/tcp" "$out"
check "两条规则" "Rule added: 8443/tcp" "$(cat "$UFW_LOG")"

echo "=== 3. UDP ==="
out=$(run "51820/udp"); check "51820/udp" "已放行 51820/udp" "$out"

echo "=== 4. 端口段 ==="
out=$(run "40000-40100"); check "40000-40100/tcp" "已放行 40000-40100/tcp" "$out"

echo "=== 5. 非法输入 ==="
out=$(run "abc"); check "拒绝 abc" "格式不对" "$out"
out=$(run "0"); check "拒绝 0" "端口超出范围" "$out"
out=$(run "70000"); check "拒绝 70000" "端口超出范围" "$out"
out=$(run "443&&rm"); check "拒绝注入" "格式不对" "$out"

echo "=== 6. 空输入直接返回 ==="
: > "$UFW_LOG"
out=$(printf '\n' | quick_open_port 2>&1); check "无输出" "^$" "$out"
check "未调用 ufw" "^$" "$(cat "$UFW_LOG")"

echo
echo "结果：$pass 通过 / $fail 失败"
rm -rf "$tmp"
[[ "$fail" -eq 0 ]]

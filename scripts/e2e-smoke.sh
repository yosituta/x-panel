#!/usr/bin/env bash
# 端到端冒烟测试：验证面板能启动、能登录、一键批量部署可用、周期重置字段可增删改。
#
# 用法：
#   1) 先启动面板（建议用独立目录，避免污染正式库）：
#        mkdir -p /tmp/xp && cd /tmp/xp && XUI_DEBUG=true /path/to/x-ui run
#   2) 另开终端运行：
#        BASE=http://127.0.0.1:13688 ./scripts/e2e-smoke.sh
#
# 依赖：curl、python3（Linux/macOS 自带；Windows Git-Bash 下需装 python）
set -uo pipefail

BASE="${BASE:-http://127.0.0.1:13688}"
USER_NAME="${XUI_USER:-admin}"
PASS="${XUI_PASS:-admin}"
JAR="${JAR:-./.e2e-cookie.txt}"
CURL="${CURL:-curl -sS}"   # 自签证书场景可传 CURL="$CURL -k"
PAYLOAD="./.e2e-payload.json"
CHECKFILE="./.e2e-check.json"
cleanup_files() { rm -f "$JAR" "$PAYLOAD" "$CHECKFILE"; }
trap cleanup_files EXIT

pass() { echo "  ✅ $1"; }
fail() { echo "  ❌ $1"; exit 1; }

# check <json> <python 表达式，变量 d 为解析后的 JSON>
check() {
    # 把 JSON 落到文件再交路径给 python：Windows 下命令行长度有限，
    # 入站多了以后直接把 JSON 当参数会 "Argument list too long"
    printf '%s' "$1" > "$CHECKFILE"
    python3 - "$CHECKFILE" "$2" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
ok = eval(sys.argv[2], {"__builtins__": {},
                        "len": len, "any": any, "all": all,
                        "str": str, "int": int, "sum": sum},
          {"d": d})
sys.exit(0 if ok else 1)
PY
}

echo "== 1. 登录 =="
R=$($CURL -c "$JAR" -X POST -d "username=$USER_NAME&password=$PASS" "$BASE/login")
check "$R" 'd["success"] is True' || fail "登录失败: $R"
pass "登录成功"

echo "== 2. 读取入站列表 =="
R=$($CURL -b "$JAR" "$BASE/panel/api/inbounds/list")
check "$R" 'd["success"] is True' || fail "列表接口异常: $R"
BEFORE=$(printf '%s' "$R" | python3 -c 'import json,sys;print(len(json.load(sys.stdin)["obj"]))')
pass "当前入站数：$BEFORE"

echo "== 3. 一键配置：批量部署 2 条 VLESS+Reality+Vision =="
R=$($CURL -b "$JAR" -H "Content-Type: application/json" -X POST \
    -d '{"count":2,"remarkPrefix":"e2e","host":"203.0.113.9"}' \
    "$BASE/panel/api/inbounds/batchDeployReality")
check "$R" 'd["success"] is True and len(d["obj"]["created"]) == 2 and len(d["obj"]["failed"]) == 0' \
    || fail "批量部署失败: $R"
echo "$R" | python3 -c 'import json,sys
for c in json.load(sys.stdin)["obj"]["created"]:
    print("     port=%s  sid=%s  link=%s..." % (c["port"], c["shortId"], c["link"][:60]))'
PORT1=$(printf '%s' "$R" | python3 -c 'import json,sys;print(json.load(sys.stdin)["obj"]["created"][0]["port"])')
pass "批量部署成功"

echo "== 4. 校验落库结果（数量 / 协议 / tag / Reality 配置） =="
R=$($CURL -b "$JAR" "$BASE/panel/api/inbounds/list")
check "$R" 'len(d["obj"]) == '"$((BEFORE + 2))" || fail "入站数量不正确"
check "$R" 'any(i["port"]=='"$PORT1"' and i["protocol"]=="vless" and "reality" in i["streamSettings"] and i["tag"].startswith("inbound-") for i in d["obj"])' \
    || fail "新入站字段不正确"
pass "入站已入库，协议与 Reality 配置正确"

echo "== 5. 周期重置字段：新建（每月 12 号） =="
RPORT=$(( 33000 + RANDOM % 3000 ))
EMAIL="e2e-$RPORT@local"
cat > "$PAYLOAD" <<JSON
{
  "enable": true,
  "remark": "e2e-reset",
  "total": 0,
  "expiryTime": 0,
  "listen": "",
  "port": $RPORT,
  "protocol": "vless",
  "settings": "{\"clients\":[{\"id\":\"11111111-2222-3333-4444-555555555555\",\"flow\":\"\",\"email\":\"$EMAIL\",\"limitIp\":0,\"totalGB\":0,\"expiryTime\":0,\"enable\":true,\"tgId\":0,\"subId\":\"\",\"comment\":\"\",\"reset\":0}],\"decryption\":\"none\",\"fallbacks\":[]}",
  "streamSettings": "{\"network\":\"tcp\",\"security\":\"none\"}",
  "sniffing": "{\"enabled\":false}",
  "resetPeriod": "monthly",
  "resetDay": 12
}
JSON
R=$($CURL -b "$JAR" -H "Content-Type: application/json" -X POST -d @"$PAYLOAD" "$BASE/panel/api/inbounds/add")
check "$R" 'd["success"] is True' || fail "新建入站失败: $R"
NEWID=$(printf '%s' "$R" | python3 -c 'import json,sys;print(json.load(sys.stdin)["obj"]["id"])')
R=$($CURL -b "$JAR" "$BASE/panel/api/inbounds/get/$NEWID")
check "$R" 'd["obj"]["resetPeriod"] == "monthly" and d["obj"]["resetDay"] == 12' \
    || fail "重置字段未保存: $R"
pass "新建时 resetPeriod=monthly / resetDay=12 已保存"

echo "== 6. 周期重置字段：编辑（改为每日） =="
sed -i.bak 's/"resetPeriod": "monthly"/"resetPeriod": "daily"/' "$PAYLOAD" 2>/dev/null || \
    python3 - "$PAYLOAD" <<'PY'
import sys, io
p = sys.argv[1]
s = io.open(p, encoding="utf-8").read().replace('"resetPeriod": "monthly"', '"resetPeriod": "daily"')
io.open(p, "w", encoding="utf-8").write(s)
PY
rm -f "$PAYLOAD.bak"
R=$($CURL -b "$JAR" -H "Content-Type: application/json" -X POST -d @"$PAYLOAD" "$BASE/panel/api/inbounds/update/$NEWID")
check "$R" 'd["success"] is True' || fail "编辑入站失败: $R"
R=$($CURL -b "$JAR" "$BASE/panel/api/inbounds/get/$NEWID")
check "$R" 'd["obj"]["resetPeriod"] == "daily"' || fail "编辑后字段未更新: $R"
pass "编辑后 resetPeriod=daily 已生效（此处曾因逐字段拷贝遗漏而静默失效）"

echo "== 7. 清理测试数据 =="
for id in $($CURL -b "$JAR" "$BASE/panel/api/inbounds/list" | python3 -c '
import json,sys
for i in json.loads(sys.stdin.read())["obj"]:
    if i["remark"].startswith("e2e"): print(i["id"])' | tr -d '\r'); do
    $CURL -b "$JAR" -X POST "$BASE/panel/api/inbounds/del/$id" >/dev/null
done
R=$($CURL -b "$JAR" "$BASE/panel/api/inbounds/list")
check "$R" 'len([i for i in d["obj"] if i["remark"].startswith("e2e")]) == 0' \
    || fail "清理后仍有 e2e 入站"
pass "已清理 e2e 测试入站"

echo
echo "全部通过 ✅"

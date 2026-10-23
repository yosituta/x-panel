#!/usr/bin/env bash
# 同步上游 xeefei/X-Panel 的最新提交到本仓库
# 用法: ./scripts/sync-upstream.sh [--check]
#   --check  只检查有没有新提交，不合并
set -euo pipefail

UPSTREAM_URL="${UPSTREAM_URL:-https://github.com/xeefei/X-Panel.git}"
UPSTREAM_REMOTE="${UPSTREAM_REMOTE:-upstream}"
BRANCH="${BRANCH:-main}"

cd "$(git rev-parse --show-toplevel)"

if ! git remote get-url "$UPSTREAM_REMOTE" >/dev/null 2>&1; then
    echo "==> 添加远端 $UPSTREAM_REMOTE -> $UPSTREAM_URL"
    git remote add "$UPSTREAM_REMOTE" "$UPSTREAM_URL"
fi

echo "==> 拉取上游 $BRANCH"
git fetch "$UPSTREAM_REMOTE" "$BRANCH" --tags --prune

RANGE="$UPSTREAM_REMOTE/$BRANCH"
if git rev-parse --verify -q "$RANGE" >/dev/null; then
    AHEAD=$(git rev-list --count "HEAD..$RANGE" 2>/dev/null || echo 0)
else
    AHEAD=0
fi

echo "==> 上游领先本仓库 $AHEAD 个提交"
if [ "$AHEAD" -eq 0 ]; then
    echo "已是最新。"
    exit 0
fi

echo
echo "==> 上游新提交:"
git log --oneline --no-decorate "HEAD..$RANGE" | head -40

if [ "${1:-}" = "--check" ]; then
    exit 0
fi

echo
echo "==> 变更文件统计:"
git diff --stat "HEAD..$RANGE" | tail -20

echo
echo "==> 开始合并（冲突时按提示手工解决，重点检查 web/service、web/html）"
if git merge --no-ff "$RANGE" -m "chore: 同步上游 xeefei/X-Panel $(date +%Y-%m-%d)"; then
    echo "==> 合并成功，请执行: go build ./... && go vet ./..."
else
    echo "!! 合并冲突，请手工解决后 git commit"
    echo "   提示: 我们改动集中的文件:"
    echo "     database/model/model.go"
    echo "     web/service/inbound.go  web/job/*.go"
    echo "     web/html/form/*.html    web/html/index.html"
    echo "     web/translation/translate.zh_CN.toml / en_US"
    exit 1
fi

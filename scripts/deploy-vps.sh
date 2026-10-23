#!/usr/bin/env bash
# 在测试服务器上安装并启动 X-Panel 开源版（systemd 管理）
# 用法：bash deploy.sh [端口]
set -euo pipefail

PORT="${1:-23333}"
DIR=/opt/x-panel
DBDIR=/etc/x-ui

echo "== 1. 目录与二进制 =="
mkdir -p "$DIR/bin" "$DIR/cert" "$DBDIR"
[ -x "$DIR/x-ui" ] || { echo "缺少 $DIR/x-ui，请先构建"; exit 1; }
chmod +x "$DIR/x-ui"

if [ ! -x "$DIR/bin/xray-linux-amd64" ]; then
    echo "-> 下载 xray-core"
    TMP=$(mktemp -d)
    V=$(curl -fsSL https://api.github.com/repos/XTLS/Xray-core/releases/latest | jq -r .tag_name)
    curl -fsSL -o "$TMP/xray.zip" "https://github.com/XTLS/Xray-core/releases/download/$V/Xray-linux-64.zip"
    unzip -o -q "$TMP/xray.zip" -d "$TMP/pkg"
    install -m 755 "$TMP/pkg/xray" "$DIR/bin/xray-linux-amd64"
    cp -f "$TMP/pkg/geoip.dat" "$TMP/pkg/geosite.dat" "$DIR/bin/" 2>/dev/null || true
    rm -rf "$TMP"
fi

echo "== 2. 自签证书（让面板可在公网访问；生产请换真证书）=="
if [ ! -f "$DIR/cert/cert.pem" ]; then
    openssl req -x509 -newkey rsa:2048 -nodes -days 3650 \
        -keyout "$DIR/cert/key.pem" -out "$DIR/cert/cert.pem" \
        -subj "/CN=x-panel-open" >/dev/null 2>&1
    chmod 600 "$DIR/cert/key.pem"
fi

echo "== 3. systemd 单元 =="
cat > /etc/systemd/system/x-panel.service <<UNIT
[Unit]
Description=X-Panel Open Source Panel
After=network.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=$DIR
ExecStart=$DIR/x-ui run
Restart=on-failure
RestartSec=3
LimitNOFILE=65535
Environment="XUI_DB_FOLDER=$DBDIR"
Environment="XUI_BIN_FOLDER=$DIR/bin"

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable x-panel >/dev/null 2>&1 || true

echo "== 4. 首启（生成数据库）=="
systemctl restart x-panel
sleep 5
systemctl is-active x-panel || { journalctl -u x-panel -n 30 --no-pager; exit 1; }

echo "== 5. 写入面板设置（端口 / 证书 / 监听）=="
"$DIR/x-ui" setting -port "$PORT" -listenIP "0.0.0.0" >/dev/null 2>&1 || true
sqlite3 "$DBDIR/x-ui.db" <<SQL
DELETE FROM settings WHERE key IN ('webCertFile','webKeyFile','webListen');
INSERT INTO settings (key, value) VALUES
  ('webCertFile','$DIR/cert/cert.pem'),
  ('webKeyFile','$DIR/cert/key.pem'),
  ('webListen','0.0.0.0');
SQL
systemctl restart x-panel
sleep 5

echo "== 6. 状态 =="
systemctl is-active x-panel
ss -lntp | grep "$PORT" || true
echo "-> https://<IP>:$PORT/"

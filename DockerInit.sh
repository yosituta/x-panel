#!/bin/sh
# ============================================================
# 构建阶段：下载并放置 Xray 内核
#   用法：DockerInit.sh <TARGETARCH> <XRAY_VERSION>
#   TARGETARCH: amd64 / arm64
# ============================================================
set -e

TARGET_ARCH="${1:-amd64}"
XRAY_VERSION="${2:-v25.9.11}"

case "${TARGET_ARCH}" in
    amd64)
        XRAY_ARCH="64"
        FNAME="amd64"
        ;;
    arm64 | aarch64 | armv8)
        XRAY_ARCH="arm64-v8a"
        FNAME="arm64"
        ;;
    *)
        echo "不支持的架构: ${TARGET_ARCH}" >&2
        exit 1
        ;;
esac

mkdir -p build/bin
cd build/bin

URL="https://github.com/XTLS/Xray-core/releases/download/${XRAY_VERSION}/Xray-linux-${XRAY_ARCH}.zip"
echo "下载 Xray 内核: ${URL}"
curl -fsSL --retry 3 -o xray.zip "${URL}"
unzip -q xray.zip
rm -f xray.zip geoip.dat geosite.dat
mv xray "xray-linux-${FNAME}"
chmod +x "xray-linux-${FNAME}"

echo "已放置: build/bin/xray-linux-${FNAME} ($(./"xray-linux-${FNAME}" -version 2>/dev/null | head -1))"

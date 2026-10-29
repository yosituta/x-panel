# syntax=docker/dockerfile:1
# ============================================================
# X-Panel 容器镜像
#   - 构建阶段：Go + CGO 编译面板二进制，并内置固定版本的 Xray 内核
#   - 运行阶段：alpine 精简镜像，数据目录挂载到 /etc/x-ui
#
#   注意：Xray 内核版本固定为 v25.9.11。
#   26.7+ 的 REALITY 改用 X25519MLKEM768，老客户端（Shadowrocket 等）会认证失败，
#   详见 CHANGELOG 与 XTLS/Xray-core#6477，请勿随意调高。
# ============================================================

ARG GO_VERSION=1.27
ARG XRAY_VERSION=v25.9.11

# ---------- 构建阶段 ----------
FROM golang:${GO_VERSION}-alpine AS builder
ARG TARGETARCH
ARG XRAY_VERSION

WORKDIR /app

RUN apk add --no-cache build-base gcc curl unzip

COPY . .

ENV CGO_ENABLED=1 \
    CGO_CFLAGS="-D_LARGEFILE64_SOURCE"

RUN go build -trimpath -ldflags "-s -w" -o build/x-ui main.go

# 拉取指定版本的 Xray 内核（与 release.yml 保持一致）
RUN sh ./DockerInit.sh "${TARGETARCH:-amd64}" "${XRAY_VERSION}"

# ---------- 运行阶段 ----------
FROM alpine:3.20

ENV TZ=Asia/Shanghai
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata bash curl openssl \
    && mkdir -p /etc/x-ui /app/bin

COPY --from=builder /app/build/ /app/
COPY --from=builder /app/DockerEntrypoint.sh /app/
COPY --from=builder /app/x-ui.sh /usr/bin/x-ui

RUN chmod +x /app/x-ui /app/DockerEntrypoint.sh /usr/bin/x-ui \
    && ln -sf /app/x-ui /usr/local/bin/x-ui

# 面板数据（SQLite、配置、证书缓存）
VOLUME ["/etc/x-ui"]

EXPOSE 54321

# 面板是 https（有证书）或 http（无证书），两种都探一下
HEALTHCHECK --interval=30s --timeout=5s --start-period=25s --retries=3 \
    CMD sh -c 'curl -fsSk -o /dev/null "https://127.0.0.1:${XUI_PORT:-54321}${XUI_WEB_BASE_PATH:-/}" || curl -fsS -o /dev/null "http://127.0.0.1:${XUI_PORT:-54321}${XUI_WEB_BASE_PATH:-/}"'

ENTRYPOINT ["/app/DockerEntrypoint.sh"]

# Docker 部署

可以跑，但要先接受几个前提：**容器里没有 systemd、没有 ufw、没有 nginx**，所以脚本安装的一部分能力用不了。

| 能力 | 脚本安装 | Docker |
| --- | --- | --- |
| 面板本体 / 节点管理 | ✅ | ✅ |
| 自动申请证书（acme.sh） | ✅ | ❌ 需在宿主机签发后挂载 |
| 自动放行防火墙 | ✅ | ❌ 在宿主机放行 |
| x-ui 菜单（证书/BBR/ttyd/订阅系统） | ✅ | ❌ 菜单能开，但多数项依赖 systemd |
| 订阅系统 SublinkPro | ✅ | ❌ 需另起容器或在宿主机装 |
| 数据持久化 | `/etc/x-ui` | 挂载 `./db` |

> 想省事、想用菜单里那些运维功能，还是用一键脚本；Docker 适合"我就想要个面板 + 节点，别的不要"。

---

## 快速开始

```bash
git clone https://github.com/yosituta/x-panel.git
cd x-panel

mkdir -p db cert
docker compose up -d --build
```

首次启动会按环境变量初始化，然后打印：

```
[x-panel] 首次启动，初始化设置：端口 54321 / 账号 admin / 路径 /
```

访问 `https://服务器IP:54321`（无证书时浏览器提示不安全，点继续即可）。
账号 `admin`，密码 `123456`，**登录后立刻改**。

看日志：

```bash
docker logs -f x-panel
```

---

## 配置项（docker-compose.yml 的 environment）

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `XUI_PORT` | `54321` | 面板端口 |
| `XUI_USERNAME` | `admin` | 账号（仅首次启动生效） |
| `XUI_PASSWORD` | `123456` | 密码（仅首次启动生效） |
| `XUI_WEB_BASE_PATH` | `/` | 访问路径 |
| `XUI_CERT_DOMAIN` | 空 | 填域名后，启动时自动绑定 `/root/cert/<域名>/` 下的证书 |
| `XUI_LOG_LEVEL` | `info` | 日志级别 |
| `TZ` | `Asia/Shanghai` | 时区 |

> 账号密码只在**数据库不存在时**写入。想改：`docker compose down && rm -rf db && docker compose up -d`，或在面板里改。

---

## 网络：必须 host 模式

节点要直接占用宿主机端口（443、2053、以及随机的高位端口），所以 compose 里用的是：

```yaml
network_mode: host
```

用 host 模式后 `ports:` 段无效，面板端口完全由 `XUI_PORT` 决定。别改成 bridge + `-p`，否则节点端口全部要手写映射，还会因为端口随机而失效。

---

## 证书怎么给

容器里没有 acme.sh，证书在宿主机签发，然后挂进去：

```bash
# 宿主机（不用装面板，只装 acme.sh）
curl https://get.acme.sh | sh
~/.acme.sh/acme.sh --issue -d panel1.example.com --standalone --days 75
mkdir -p cert/panel1.example.com
~/.acme.sh/acme.sh --install-cert -d panel1.example.com \
  --key-file       cert/panel1.example.com/privkey.pem \
  --fullchain-file cert/panel1.example.com/fullchain.pem \
  --reloadcmd      "docker restart x-panel"
```

然后在 compose 里加：

```yaml
environment:
  XUI_CERT_DOMAIN: "panel1.example.com"
```

`docker compose up -d` 重启后即为 HTTPS。

---

## 升级

```bash
git pull
docker compose up -d --build
```

数据在 `./db`，不会丢。

## 备份

```bash
docker compose down
tar czf x-panel-backup-$(date +%F).tar.gz db cert
docker compose up -d
```

---

## 常见问题

**构建失败：`go.mod requires go >= 1.27`**
镜像里的 Go 版本低于 `go.mod` 声明。本仓库 Dockerfile 已用 `ARG GO_VERSION=1.27`，不要降。

**Reality 节点手机连不上**
镜像固定 Xray **v25.9.11**（`Dockerfile` 的 `ARG XRAY_VERSION`）。26.7+ 的 REALITY 改了密钥交换，Shadowrocket 等老客户端会 `authentication failed`。不要随手调高。

**面板起来了但打不开**
面板的协议取决于有没有证书：**配了证书是 HTTPS，没证书是 HTTP**。用错协议就打不开，两个都试一下。
配证书前先看 `docker logs x-panel` 有没有 `Web server running HTTP/HTTPS on`。

**节点能建但不通**
host 网络下不需要映射端口，检查宿主机防火墙：`ufw allow 443/tcp` 等。容器内改防火墙不会生效到宿主机。

**想用订阅系统（SublinkPro）**
它是独立服务，容器里没有 nginx/systemd。两个办法：在宿主机用 `scripts/sublinkpro.sh` 单独装；或者自己写个 compose 服务跑 `ZeroDeng01/sublinkPro` 镜像，再挂 nginx。Docker 模式下不提供菜单 25。

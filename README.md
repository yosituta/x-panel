# X-Panel 开源增强版

[简体中文](README.md) · **[English](README.en.md)**

基于 [xeefei/X-Panel](https://github.com/xeefei/X-Panel) 的 **GPLv3 开源分支**。上游把「一键配置」等实用功能移入付费 Pro 版并停止向免费版更新，本分支按公开功能说明重新实现，并补上了作者自己需要的运维能力。

- 面板本体：Go + Gin + SQLite，Xray-core 作为子进程托管
- 一键安装、systemd 托管、单二进制、无外部依赖
- **不包含**任何上游私有代码；**不实现**授权码/积分/购买机器人等商业闭环

> 本仓库与上游作者无隶属关系。上游名称与标识仅用于说明来源与致敬。

## 一条命令安装

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh)
```

交互式安装，按提示填写；直接回车使用默认值：

- 面板端口 `54321`
- 访问路径 `/`
- 账号 `admin` / 密码 `123456`（**登录后请立即修改**）

### 无人值守安装（推荐，多台服务器一致）

**A. 有域名（自动 HTTPS，推荐生产用）**

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh) \
  --auto --domain panel1.example.com
```

- 自动申请 Let's Encrypt 证书并绑定，**剩余 15 天自动续签**（acme.sh cron）
- 邮箱不用填，内部自动用 `admin@主域` 注册
- 自动放行防火墙端口；管理菜单装到 `/usr/bin/x-ui`

**B. 无域名（纯 IP，自签证书）**

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh) --auto
```

- 不写 `--domain` 就自动按此模式安装，**不需要**额外加 `--no-cert`（写了也等价）
- 面板与订阅系统端口和 A 方案**完全一致**，只是证书是自签的，浏览器提示"继续访问"点一下即可

| | A. 有域名 | B. 无域名 |
| --- | --- | --- |
| 面板 | `https://你的域名:54321` | `https://IP:54321`（自签） |
| 订阅系统 | `https://你的域名:15268` | `https://IP:15268`（自签） |
| 证书 | Let's Encrypt，15 天自动续签 | 自签 10 年，装上即可用 |
| CDN / Reality | 都支持 | 都支持（CDN 仍需域名） |

**B 方案以后补域名**：把域名 A 记录指到本机 → `x-ui → 18 → 1` 申请证书（输入域名即可，自动签发、自动绑定、剩余 15 天自动续签）
→ 订阅系统会自动换成正式证书，地址从 `https://IP:15268` 变成 `https://你的域名:15268`，数据不受影响。

先看计划不执行：

```bash
bash install.sh --auto --domain panel1.example.com --dry-run
```

其它参数：`--panel-port` / `--path` / `--user` / `--pass` / `--no-cert` / `--version`。

完整部署说明（域名规划、多台批量、升级回滚、排错）：[docs/DEPLOY.md](docs/DEPLOY.md)

### 端口与排序

- 批量部署节点时，端口在 **40000-65000** 之间**随机**分配（互不重复），不再从 20000 递增，避免被顺序扫描
- 手动指定起始端口时仍按指定端口顺序分配
- 入站列表按**创建先后（ID 升序）**排列，不按端口大小排序


## 功能

### 代理与节点

| 功能 | 说明 |
| --- | --- |
| 多协议入站 | VLESS / VMess / Trojan / Shadowsocks，TCP / WS / gRPC / HTTP |
| 一键批量部署 | 一次生成 N 条节点，独立端口/UUID/密钥，自动热加载并给出链接 |
| 预设体系 | `auto`（默认）/ TLS / Trojan / Reality（Reality 收在「高级预设」里） |
| Reality 随机域名 | 并发探测 TLS1.3 存活后再挑，避免配到已死的伪装域名 |
| 内置订阅转换 | `/sub/<subId>` 直接输出 V2Ray / Clash / sing-box / Surge，无需第三方 subconverter |
| 订阅系统（SublinkPro） | `x-ui` 菜单 25 一键安装；登录后添加各台节点、自由建订阅、勾选节点，得一条总订阅链接（15268） |
| 一键中转节点 | 本机入口 → 远程 Socks5/HTTP 上游 → 出网，自动写 outbound 与路由规则 |

### 流量与用户管理

| 功能 | 说明 |
| --- | --- |
| 周期重置流量 | 每日 / 每周 / 每月 N 号 / 从不，短月与闰年已处理 |
| 流量与到期告警 | 用量达阈值（默认 80%）或到期前 N 天推 Telegram，同阈值只提醒一次 |
| 节点上下线通知 | 比对 `lastOnline`，状态变化才推送，首次运行只建基线不刷屏 |

### 运维

| 功能 | 说明 |
| --- | --- |
| 数据快照 + 急救还原 | `VACUUM INTO` 生成一致性快照；面板崩了也能用 CLI 一条命令还原 |
| 网页版 SSH | ttyd + basic auth + 复用面板证书走 HTTPS，`x-ui` 菜单 26 |
| 线路/IP 质量检测 | 出口信息、四大 DNS 延迟丢包、回程路由、带宽粗测，菜单 27 |
| 地区 DNS 检测 | 8 个国内外 DNS 对比解析，自动提示污染风险，菜单 28 |
| 深度调优 | BBR + FQ + TFO + 缓冲区，幂等 apply/status/revert，菜单 29 |

### 界面与协作

| 功能 | 说明 |
| --- | --- |
| 首页 5 套主题 | 标准 / 炫彩 / 深海 / 暮光 / 幽林，选定即记住 |
| 多面板主从 | 一个面板管理其他面板，被控端 `X-Slave-Key` 鉴权，默认关闭 |
| 定时报告 | 每天/每周/每月指定时间推送，内容项可勾选，支持预览与立即发送 |
| Telegram | `/link` 取节点链接、`/panels` `/panel` 管多面板、`/webssh` 查网页终端 |

## 仓库地址

本项目仓库：**https://github.com/yosituta/x-panel**

`install.sh` / `x-ui.sh` 里的 `X_PANEL_REPO` 已指向该仓库。想临时装上游原版：

```bash
X_PANEL_REPO=xeefei/x-panel bash install.sh
```

发布新版本：改完代码 `git tag v1.0.1 && git push origin v1.0.1`，GitHub Actions 会自动编译 amd64/arm64（CGO）并把产物挂到 Release。

发布前自检：

```bash
go build ./... && go vet ./... && go test ./...
bash -n install.sh x-ui.sh scripts/*.sh
git ls-files | grep -E "\.db$|\.pem$|\.key$|\.env$" || echo "无敏感文件"
```

## 快速开始

### 一键安装（推荐）

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh)

无人值守安装（多台服务器推荐，可自动申请证书）：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh) \
  --auto --domain panel1.example.com --email you@example.com
```

完整部署说明（域名规划、多台批量、排错）：[docs/DEPLOY.md](docs/DEPLOY.md)
```

安装完成后按提示设置端口、路径、账号密码。管理脚本：

```bash
x-ui            # 打开管理菜单（含证书、备份、调优、网页 SSH 等）
```

### 手动编译

需要 **Go 1.27+** 与 CGO（sqlite 驱动依赖）：

```bash
# Linux
sudo apt install -y build-essential git
CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o x-ui .

# Windows（需 MinGW-w64 或 zig）
set CGO_ENABLED=1
go build -trimpath -ldflags "-s -w" -o x-ui.exe .
```

1 核 512MB 的小机器不建议本地编译，用 GitHub Actions 或交叉编译更省事：

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 \
  CC="zig cc -target x86_64-linux-gnu" \
  go build -trimpath -ldflags "-s -w" -o x-ui .
```

## 首次使用建议

1. **改掉默认端口与访问路径**：`x-ui` → 9 / 7
2. **配 HTTPS**：`x-ui` → 18（申请证书），或手动填证书路径
3. **一键配置选「自动」**：有证书且用域名访问时生成 TLS 节点，兼容性最好
4. **备份先做一次**：设置 → 数据快照 → 立即备份；或 `x-ui backup`
5. 不用网页 SSH 时记得卸载（`x-ui` → 26 → 卸载）

## 订阅与格式转换

每个客户端自带 16 位 `subId`，一条地址四种格式：

```
https://面板地址/路径/sub/<subId>                 # V2Ray（base64）
https://面板地址/路径/sub/<subId>?target=clash    # Clash / Mihomo
https://面板地址/路径/sub/<subId>?target=singbox  # sing-box
https://面板地址/路径/sub/<subId>?target=surge    # Surge
```

也可用后缀：`.yaml` / `.json` / `.conf`。响应头带 `Subscription-Userinfo`，客户端可直接显示已用流量与到期时间。

## 命令行

```bash
x-ui                      # 交互菜单
x-ui setting -show        # 查看面板设置
x-ui setting -port 23333  # 改端口
x-ui backup               # 立即生成数据快照
x-ui backup -list         # 列出快照
x-ui backup -restore x-ui-20260101-090000.db.gz   # 还原（面板需停止）
x-ui -v                   # 版本
```

## 与上游同步

```bash
git remote add upstream https://github.com/xeefei/X-Panel.git   # 仅首次
./scripts/sync-upstream.sh --check   # 看上游有没有新提交
./scripts/sync-upstream.sh           # 合并上游代码
```

合并后必做：

```bash
go build ./... && go vet ./... && go test ./...
```

冲突高发文件：`database/model/model.go`、`web/service/inbound.go`、`web/html/index.html`、`web/html/inbounds.html`、`web/translation/*.toml`。

## 开发与测试

```bash
go test ./web ./web/service -run Test      # 单元测试
bash scripts/e2e-smoke.sh                  # 端到端冒烟（需可运行的面板）
bash scripts/deploy-vps.sh                 # 部署到 VPS
```

覆盖：周期重置时间逻辑、节点与订阅四种格式渲染、告警阈值、上下线判定、快照压缩与 SQLite 头校验、模板解析。

## 目录结构

```
main.go              入口（run / setting / migrate / backup）
web/                 面板：controller、service、html 模板、assets
xray/                Xray 进程与 gRPC API 封装
database/            SQLite 模型与 DAO
config/              配置项
scripts/             运维脚本（调优、DNS、线路、ttyd、e2e、部署、同步）
docs/                路线图、上游 README、分析笔记
```

## 常见问题

**Reality 节点测延迟正常但打不开网页？**
服务端日志若出现 `REALITY: authentication failed`，是客户端 Reality 实现与服务端不兼容（实测部分 Shadowrocket 版本如此）。换 `自动` 或 `TLS` 预设即可，不属于服务端配置问题。

**订阅里的地址是 127.0.0.1？**
地址取值顺序：订阅域名 → 面板证书目录名 → 机器名。在设置里填「订阅域名」，或访问面板时用域名而非 IP。

**一键中转节点创建后连不上？**
先确认上游 Socks5/HTTP 本身可用；再看 `xray` 日志里有没有 `failed to dial`。删除用 `POST /panel/api/inbounds/relay/remove/<端口>`，会同步清理 outbound 与路由。

**大文件上传到 VPS 老是断？**
分片上传（`split -b 2M`）+ 逐片校验，或直接在服务器上编译。

## 致谢与许可

- 面板骨架来自 [xeefei/X-Panel](https://github.com/xeefei/X-Panel)（GPLv3）与 [XTLS/Xray-core](https://github.com/XTLS/Xray-core)
- 本分支同样以 **GPLv3** 发布，详见 [LICENSE](LICENSE) 与 [NOTICE.md](NOTICE.md)
- 按 GPLv3 要求：二次分发需保留版权声明与许可，并标明修改内容（见 [CHANGELOG.md](CHANGELOG.md)）

# 项目 README（开源分支）

> 本仓库是 [xeefei/X-Panel](https://github.com/xeefei/X-Panel) 的 **GPLv3 开源分支**，
> 不是官方版本，也不包含官方“付费 Pro 版”的任何私有代码。
> 上游 Pro 版以「授权码 + 远程脚本下发」形式分发，源码从不在公开仓库中——
> 因此本仓库中的增强功能，是依据上游 README 公开的功能描述**独立重新实现**的。

## 为什么有这个仓库

上游把越来越多的功能放进了付费 Pro 版（一键配置、主题切换、流量重置方式、
批量部署节点等），免费版只剩“购买引导”弹窗。本仓库的目标是：

1. 用 GPLv3 允许的方式，把这些功能**自己写一遍**，不再依赖授权码；
2. 保持与上游同步能力，上游更新后可以一条命令拉取并合并；
3. 全程可编译、可测试、可审计。

## 功能状态

详见 [`docs/ROADMAP.md`](docs/ROADMAP.md)。已实现：

| 功能 | 说明 |
| --- | --- |
| 入站周期重置流量 | 每日 / 每周 / 每月 N 号 / 从不；到期自动清零并恢复超量停用的客户端 |
| 首页 5 套主题 | 标准布局、炫彩动画、深海科技、暮光薰衣、幽林秘境；选定即记住，不再按单双号乱跳 |
| Reality 随机域名 | 后端并发探测 TLS1.3 存活后随机挑选，避免配到已挂的域名 |
| 一键配置 / 批量部署 | 一次生成 N 条节点，独立端口/UUID/密钥，自动热加载并给出分享链接；预设含 auto / TLS / Reality / Trojan |
| 内置订阅转换 | 每个客户端自带 `subId`，`/sub/<subId>` 直接输出 V2Ray / Clash / sing-box / Surge 四种格式，无需第三方 subconverter |

## 客户端兼容性（实测）

部分客户端（实测 iPhone Shadowrocket）无法与 xray 的 Reality 服务端完成握手，服务端日志表现为：

```
REALITY: processed invalid connection: authentication failed or validation criteria not met
```

此时客户端会收到伪造站点的真实证书后主动断开，表现为"测延迟正常、但打不开网页"。
已在服务端排除证书、端口、域名、shortId 编码、密钥格式、xray 版本等因素（同一节点用 xray 官方客户端可正常出网）。

**因此一键配置提供 `auto` 预设并作为默认**：面板配置了证书且通过域名访问时自动生成 TLS 节点（兼容性最好），否则退回 Reality。若你的客户端 Reality 不可用，请选 `VLESS + TCP + TLS` 或 `auto`。

## 与上游同步

```bash
git remote add upstream https://github.com/xeefei/X-Panel.git   # 仅首次
./scripts/sync-upstream.sh --check   # 看有没有新提交
./scripts/sync-upstream.sh           # 合并上游最新代码
```

## 构建

需要 **Go 1.27+** 与 **CGO**（sqlite 驱动依赖 cgo）：

```bash
# Linux（Debian/Ubuntu）
sudo apt install -y build-essential
CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o x-ui .

# Windows（需 MinGW-w64，Make 可选）
set CGO_ENABLED=1
go build -trimpath -ldflags "-s -w" -o x-ui.exe .
```

> `CGO_ENABLED=0` 能编译通过，但运行时会报
> `go-sqlite3 requires cgo`，**必须开启 CGO**。

## 测试

```bash
go vet ./...        # 已修复上游遗留的两处 vet 告警，现在全绿
go test ./...
```

## 许可

GPL-3.0，见 `LICENSE` 与 `NOTICE.md`。再分发时必须同样以 GPLv3 提供源码并保留上游版权声明。

# 功能路线图 / 进度表

对照上游 README 第 73–101 行公开的 32 项 Pro 功能，逐项落地。
状态：✅ 已完成并测试 ｜ 🚧 进行中 ｜ ⬜ 未开始 ｜ ❌ 不复刻（属对方商业模式）

## 第一批（已完成）

| # | 功能 | 状态 | 落地位置 |
| --- | --- | --- | --- |
| 15 | 创建入站可选重置流量方式（每日/每周/按月/从不） | ✅ | `database/model/model.go`、`web/service/traffic_reset.go`、`web/job/traffic_reset_job.go`、`web/html/form/inbound.html` |
| 30 | 每月 N 号重置（1–31，短月取最后一天） | ✅ | 同上（`resetDay`，含闰年处理，有单测） |
| 14 | 首页 5 套主题手动选定 | ✅ | `web/html/index.html`（主题管理器 + 3 套新主题 CSS） |
| 3 | Reality 随机更换偷用域名 | ✅ | `web/service/reality_domain.go`、`form/reality_settings.html` |
| 31 | 一键配置 / 批量部署 10 条 VLESS+TCP+Reality+Vision | ✅ | `web/service/batch_deploy.go`、`/panel/api/inbounds/batchDeployReality`、`inbounds.html` 一键配置弹窗 |

### 第一批实测要点

- 周期重置：`go test ./web/service -run 'Scheduled|IsDue'` 6 个用例全绿
  （每日/每周/每月、闰年、跨周期幂等、禁用入站不重置）。
- 批量部署：`buildVlessRealityInbound` 为纯函数，单测校验了 settings/streamSettings
  的完整 JSON 结构、GB→字节换算、tag 规则、分享链接参数。
- x25519 密钥用 `crypto/ecdh` 生成，输出格式与 `xray x25519` 一致
  （base64url raw，43 字符），**不依赖 xray 二进制**，因此可离线单测。

## 第二批（计划）

| # | 功能 | 说明 | 预估 |
| --- | --- | --- | --- |
| 19 | TG 获取节点链接（本机 + 被控端） | `tgbot.go` 增加命令，复用 `GenXrayInboundConfig` 生成链接 | 中 |
| 25 | 节点上/下线 TG 通知 | 调度器比对在线状态，变化时推群 | 中 |
| 16 | TG 每日报告可定制内容与时间 | `setting` 加字段 + `StatsNotifyJob` 改造 | 中 |
| 26 | 签到得积分（含排行榜） | 新表 `user_point` + TG 命令 + 每日签到 job | 大 |
| 22 | 流量/在线告警阈值细分 | 扩展 `job/check_*` | 小 |

## 第三批（脚本类，不动面板核心）

| # | 功能 | 做法 |
| --- | --- | --- |
| 8 | 线路和 IP 质量检测 | `nexttrace`/`mtr` 封装 |
| 9 | 地区服务器 DNS 检测 | 批量 `dig` + 着色输出 |
| 23 | 深度调优（BBR+FQ / TFO / 缓冲区） | 幂等 sysctl 脚本 |
| 29 | 数据快照 + 远程急救还原 | `sqlite3 .backup` + systemd 单元 |
| 7/10 | 网页版 SSH（ttyd）部署 | 安装脚本菜单项 + 面板入口 |

## 第四批（重工程）

| # | 功能 | 说明 |
| --- | --- | --- |
| 18 | 一个机器人管理多面板（主从） | 被控端 API 客户端 + 主控端命令路由，约 1500 行 |
| 20 | 一键部署中转节点 | 远程 Socks5 → 本机路由 → 入口 → 链接/二维码 |

## 不复刻

- 17「授权码联网验证」、32「购买机器人」、24「会员等级」：
  属于上游的收费业务，不是技术功能，本仓库天然不需要。
- 1/5/6/11/13/21/27/28：UI 标识与文案类，以及依附于积分/授权体系的子项。

## 每次合并上游后的检查清单

1. `go build ./...`
2. `go test -vet=off ./web ./web/service -run 'Test'`
3. 重点复查冲突高发文件：
   - `database/model/model.go`（我们给 Inbound 加了 3 个字段）
   - `web/service/inbound.go`、`web/job/`（新增文件，一般不冲突）
   - `web/html/index.html`（主题管理器替换了单双号切换脚本）
   - `web/html/inbounds.html`（一键配置弹窗被整体替换）
   - `web/translation/translate.zh_CN.toml` / `en_US.toml`

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
| 33 | 内置订阅转换（V2Ray / Clash / sing-box / Surge） | ✅ | `web/service/subscribe_node.go`、`subscribe_build.go`、`subscribe_render.go`、`web/controller/sub.go`、`GET /sub/:subid` |
| 34 | 一键配置「自动」预设（有证书走 TLS，否则 Reality） | ✅ | `batch_deploy.go` `PresetAuto`、`inbounds.html`（Reality 系列收进「高级预设」开关） |
| 35 | 流量与到期告警（Telegram 推送 + 同阈值去重） | ✅ | `web/service/alert.go`、`alert_run.go`、`web/job/alert_job.go`、`POST /panel/api/server/checkAlerts` |

### 第一批实测要点

全部在真实进程上跑过（Windows + MinGW/CGO 构建，Xray 未安装的降级场景）：

| 验证项 | 结果 |
| --- | --- |
| `scripts/e2e-smoke.sh` 全流程（登录 → 批量部署 → 落库校验 → 周期字段增改 → 清理） | 7/7 通过 |
| 批量部署 2 条 Reality 节点 | 返回 2 条真实 x25519 公钥 + shortId + vless 链接，入库可查 |
| 周期重置真实触发（up=123456 / down=654321） | 1 分钟内被清零，`lastResetTime` 写为当日 00:00 |
| 同周期幂等 | 再等 70 秒，`lastResetTime` 不变、流量仍为 0 |
| 编辑入站改重置方式 | `monthly/12` → `daily` 持久化成功 |


- 周期重置：`go test ./web/service -run 'Scheduled|IsDue'` 6 个用例全绿
  （每日/每周/每月、闰年、跨周期幂等、禁用入站不重置）。
- 批量部署：`buildVlessRealityInbound` 为纯函数，单测校验了 settings/streamSettings
  的完整 JSON 结构、GB→字节换算、tag 规则、分享链接参数。
- x25519 密钥用 `crypto/ecdh` 生成，输出格式与 `xray x25519` 一致
  （base64url raw，43 字符），**不依赖 xray 二进制**，因此可离线单测。

## 真实服务器实测（Ubuntu 24.04 / 1 核 458MB / KVM）

用 `scripts/deploy-vps.sh` 部署到一台干净 VPS（systemd 管理，面板 + xray 同进程树）：

| 验证项 | 结果 |
| --- | --- |
| 构建 | 服务器 1 核编译 xray-core 过慢（swap 抖动 20+ 分钟未完成），改用 `zig cc -target x86_64-linux-gnu` 在本地交叉编译，2.5 分钟出 67MB 二进制，服务器直接运行 |
| 服务 | `x-panel.service` active，面板监听 0.0.0.0:23333（HTTPS，自签证书），xray 作为子进程随面板启动 |
| e2e 脚本 | 服务器上 7/7 通过（`CURL="curl -sS -k"`） |
| 真实代理链路 | 用批量部署生成的节点建 xray 客户端 → socks5 → Reality 入站 → freedom 出网，`api.ipify.org` / `cloudflare/cdn-cgi/trace` 均返回成功；面板统计到 up=7353 / down=9654 |
| 周期重置（真实流量） | 把该入站改为 daily 后，1 分钟内 7353/9654 归零，`lastResetTime` 写入当日 00:00，日志 `周期重置流量: 入站 #1` |
| 前端 | 中英文两套文案都可正确渲染（主题选择器 5 项、入站表单重置项、一键配置弹窗） |

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
2. `go vet ./... && go test ./...`
3. 重点复查冲突高发文件：
   - `database/model/model.go`（我们给 Inbound 加了 3 个字段）
   - `web/service/inbound.go`、`web/job/`（新增文件，一般不冲突）
   - `web/html/index.html`（主题管理器替换了单双号切换脚本）
   - `web/html/inbounds.html`（一键配置弹窗被整体替换）
   - `web/translation/translate.zh_CN.toml` / `en_US.toml`

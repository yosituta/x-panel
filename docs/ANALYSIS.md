# X-Panel 上游代码分析报告

分析对象：`xeefei/X-Panel` 提交 `8f6bcd8`（版本号 v26.9.13），开源许可 **GPL-3.0**。

---

## 1. 技术栈与架构

| 层 | 技术 | 位置 |
| --- | --- | --- |
| 语言 | Go 1.27（`module x-ui`） | `go.mod` |
| Web 框架 | Gin + gin-contrib/sessions | `web/web.go` |
| ORM | GORM（SQLite 单文件库） | `database/` |
| 定时任务 | robfig/cron（秒级） | `web/web.go: startTask()` |
| 前端 | 服务端 Go template + Vue 2 + Ant Design Vue 1.x，无构建步骤 | `web/html/` |
| 静态资源 | `go:embed web/assets`（`-tags` 无则读磁盘） | `web/web.go` |
| 内核 | Xray-core，通过 gRPC API 交互 | `xray/` |
| 机器人 | telego（Telegram Bot） | `web/service/tgbot.go` |
| 多语言 | TOML 词条表 | `web/translation/translate.<locale>.toml` |

规模：Go 代码约 **1.73 万行**（66 个文件），HTML 模板 67 个（约 7000 行），`tgbot.go` 单文件 4314 行是最大模块。

### 请求链路

```
main.go  ── 解析命令行/设置 ──> web.Server (web/web.go)
                                   ├─ initRouter(): controller.NewXxxController(g)
                                   ├─ startTask(): 注册 cron（流量统计、到期检查、TG 推送）
                                   └─ Start(): 监听端口（无证书则强制 127.0.0.1，仅 SSH 转发可用）
controller/*  ── 绑定参数 ──>  service/*  ──> GORM / xray API
web/html/*    ── Go template 渲染（i18n 函数注入词条）
```

### 关键机制（改功能前必读）

1. **配置即 KV 表**：`model.Setting{Key,Value}`。新增一个设置项 = 在 `entity.AllSetting` 加字段 + 在 `setting.go: defaultValueMap` 加默认值，反射自动完成读写与前端绑定。**这是本项目扩展功能最省事的入口。**
2. **流量统计是“差值”模型**：`GetXrayTraffic()` 带 `GetTraffic(true)`，每次读取即清零内核计数器，只把增量累加到库。⇒ **直接清零数据库的 up/down 是安全的**，不会重复计费，也不会被下一次统计“补回来”。
3. **入站入库与 Xray 配置分离**：`model.Inbound` 存行，`settings/streamSettings/sniffing` 是 JSON 字符串；前端 `DBInbound` 负责二者互转（`web/assets/js/model/dbinbound.js`）。
4. **前端无打包器**：改 HTML/JS 后**不需要** npm；重新 `go build` 即生效（`assets` 走 embed）。开发时可用 `-tags` 或删 `web/assets` 内的 embed 走磁盘（见 `web/web.go:75-90`）。
5. **i18n 只覆盖 UI 文案**：新增文案要同时补 `translate.zh_CN.toml`、`translate.en_US.toml`，否则回落到 key 本身。

---

## 2. “免费版 / PRO 版”的真实边界（重要）

结论：**PRO 版的代码不在这个仓库里，一行都不在。**

证据：`install.sh:47-108` 的 `install_paid_version()`：

```bash
read -p "请输入您的授权码 (License Key):" auth_key
vps_ip=$(curl -s4m8 ip.sb -k | head -n 1)
vps_hwid=$(get_hwid)                        # 机器指纹
AUTH_SERVER_URL="https://auth.x-panel.vip/install_pro.php"
response=$(curl -sL -X POST -d "key=$auth_key&ip=$vps_ip&hwid=$vps_hwid" "$AUTH_SERVER_URL")
bash <(echo "$response")                    # 直接把服务器返回的脚本喂给 bash
```

即 PRO 版是「**授权码 + IP + 机器指纹 → 远程下发安装脚本**」的闭源交付物。公开仓库里只留下了两处“广告位”：

- `web/service/tgbot.go:575,1993`：`〔一键配置〕` 被替换成“请购买授权码”的提示文本；
- `web/html/inbounds.html:704-731`：`oneClickModal` 弹窗只剩购买引导；
- `install.sh:597`：安装菜单第 2 项跳去授权码流程。

⇒ **不存在“解锁”这回事**：没有可解锁的代码。所谓“拥有全部功能”，只能是**依据 README 公开的功能描述重新实现**。本项目即按此路径推进（GPLv3 允许，且不涉及任何逆向/破解）。

---

## 3. PRO 功能清单与实现方案

上游 README 第 73–101 行列出 32 项。按其实现性质分四类：

### A 类：纯后端/前端功能，可完整复刻（本仓库主战场）

| # | 功能 | 实现位置 | 难度 |
| --- | --- | --- | --- |
| 3 | Reality 随机更换所偷域名 | `form/reality_settings.html` + 后端域名池 | ★ |
| 14 | 首页 5 套主题手动选定 | `index.html` + 本地/设置持久化 | ★★ |
| 15 | 创建入站可选重置流量方式 | `model.Inbound` + `job/traffic_reset` + 表单 | ★★ |
| 30 | 每月 N 号重置（1–31） | 同上，`resetDay` | ★ |
| 31 | 批量部署 10 条 VLESS+TCP+Reality+Vision | 替换 `oneClickModal` + 批量建站接口 | ★★★ |
| 19 | TG 获取节点链接（本机/被控端） | `tgbot.go` 新命令 | ★★ |
| 25 | 节点上/下线 TG 通知 | 调度器比对 + `tgbot.go` | ★★ |
| 16 | TG 每日报告可定制内容/时间 | `setting` + `StatsNotifyJob` | ★★ |
| 26 | 签到得积分（含排行榜） | 新增表 `user_point` + TG 命令 | ★★★ |
| 22 | 流量/在线告警阈值细分 | `job/check_*` 扩展 | ★★ |
| 7/10 | 网页版 SSH 部署（ttyd） | 安装脚本 + 面板入口 | ★★ |

### B 类：脚本/运维类，需独立脚本（不动面板核心）

| # | 功能 | 做法 |
| --- | --- | --- |
| 8 | 线路和 IP 质量检测 | 封装 `nexttrace`/`mtr`/`ipquality` 一键跑 |
| 9 | 地区服务器 DNS 检测 | `dig` 批量 + 结果着色 |
| 23 | 深度调优（BBR+FQ、TFO、缓冲区） | 幂等 sysctl 脚本 |
| 29 | 数据快照 + 远程急救还原 | `sqlite3 .backup` + systemd 单元 |

### C 类：重工程，需要独立子系统

| # | 功能 | 说明 |
| --- | --- | --- |
| 18 | 一个机器人管理多面板（主从） | 被控端 API 客户端 + 主控端命令路由；约 1500 行 |
| 20 | 一键部署中转节点 | 远程 Socks5 创建 → 本机路由 → 入口 → 生成链接/二维码 |
| 17 | 授权码联网验证 | **不复刻**：属对方商业模式，且本项目不需要授权 |

### D 类：不复刻

- 12/24/32：授权码购买、会员等级、购买机器人 —— 这是对方的收费业务，不是技术功能。
- 1/5/6/11/13/21/27/28：UI 标识类、文案类、以及 27/28 依附于积分体系与授权体系的子功能。

---

## 4. 本仓库的推进方式

1. **只改必要文件**，每个功能独立提交，便于 `scripts/sync-upstream.sh` 合并上游更新。
2. 功能开关一律走 `AllSetting`（KV），不做编译期开关，避免与上游 diff 冲突。
3. 每个批次结束执行 `go build ./... && go vet ./...`，并用 SQLite 起一次真实服务验证。
4. 新增文案进 `translate.zh_CN.toml` + `translate.en_US.toml`。

批次状态见 `docs/ROADMAP.md`。

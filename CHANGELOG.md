# 变更记录

**[简体中文](CHANGELOG.md)** · [English](CHANGELOG.en.md)

本文件按 GPLv3 要求记录本分支相对上游 [xeefei/X-Panel](https://github.com/xeefei/X-Panel) 的修改。

基线：上游 `main`（2026-10 拉取）。面板版本号沿用上游 `26.9.13`，功能迭代以本文为准。

---

## v1.0.0（2026-10-26）

首个完整版本：上游公开的 22 项技术类 Pro 功能全部重新实现，另加 9 项自研能力。

### 新增 · 代理与节点

- **一键批量部署**：一次生成 N 条节点，独立端口 / UUID / x25519 密钥 / shortId，自动热加载，返回分享链接
- **协议预设体系**：`auto` / `tls` / `trojan` / `reality` / `reality-novision` / `reality-udp443` / `reality-legacy`
  - `auto` 为默认：面板有证书且通过域名访问 → TLS，否则 Reality
  - Reality 系列收进「高级预设」开关，避免误选到客户端不支持的协议
- **Reality 随机伪装域名**：并发探测候选域名的 TLS1.3 存活，剔除 CDN 前置域名后随机挑选
- **内置订阅转换**：`/sub/<subId>` 输出 V2Ray / Clash / sing-box / Surge 四种格式，带 `Subscription-Userinfo` 头
- **一键中转节点**：本机入口 → 远程 Socks5/HTTP → 出网；自动写入 xray 的 outbound 与路由规则，删除时同步清理

### 新增 · 流量与用户

- **周期重置流量**：每日 / 每周 / 每月 N 号 / 从不；短月与闰年处理，跨周期幂等
- **流量与到期告警**：用量 80% / 100%、到期前 3 天 / 1 天 / 已过期分级提醒，同阈值只推一次，条件恢复后可再次提醒
- **节点上下线通知**：基于 `clientTraffic.lastOnline` 判定，首次运行只建立基线

### 新增 · 运维

- **数据快照与急救还原**：`VACUUM INTO` 生成一致性快照（面板运行中安全）、gzip 保存、按份数轮转；还原前校验 SQLite 文件头并保留原库
  - CLI：`x-ui backup` / `-list` / `-restore <文件>`
  - API：`POST /panel/api/server/backup`、`GET /backups`、`POST /restore/:name`
  - 面板：设置 → 数据快照
- **网页版 SSH（ttyd）**：`scripts/ttyd.sh` + `x-ui` 菜单 26，basic auth + 复用面板证书 HTTPS
- **线路/IP 质量检测**：`scripts/linecheck.sh`，菜单 27
- **地区 DNS 检测**：`scripts/dnscheck.sh`，菜单 28
- **深度调优**：`scripts/tune.sh`，菜单 29（BBR + FQ + TFO + 缓冲区，幂等 apply/status/revert）

### 新增 · 界面与协作

- **首页 5 套主题**：标准 / 炫彩 / 深海 / 暮光 / 幽林，手动选定并持久化
- **多面板主从**：被控端 `/panel/api/slave/{status,links,restartXray}`（`X-Slave-Key` 鉴权，默认关闭）；主控端 `/panel/api/panels/*` + 设置页「多面板」
- **定时报告**：每天/每周/每月指定时间发送，内容项（服务器 / 流量与用量前五 / 入站与到期 / 在线客户端）可勾选，支持预览与立即发送
- **Telegram 命令扩展**：`/link [关键字]`、`/panels`、`/panel <名称> status|links|restart`、`/webssh`

### 修复

- `xray/api.go`：Xray 未初始化时直接解引用 gRPC 客户端导致 panic → 新增 `handlerClient()` 返回明确错误
- `web/service/xray.go`：`GetErr()` 返回 nil 时仍调用 `err.Error()` 导致崩溃
- 编辑入站时周期重置字段未落库（更新路径漏拷贝）
- 批量部署总流量单位错误（GB 未转字节）
- 一键配置生成的链接缺少 host 时为空
- 历史客户端缺少 `subId` → 启动时自动补齐

### 相对上游的删减

- 移除安装脚本中的「付费 Pro 版」分支与授权码购买提示
- 移除 TG 端「购买授权码机器人」相关文案，改为指向面板内一键配置
- 未实现（也不打算实现）：授权码联网验证、积分/签到/排行榜、积分换购、购买机器人、会员等级标识

### 已知限制

- Reality 与部分客户端的实现不兼容（实测某些 Shadowrocket 版本），服务端日志表现为
  `REALITY: authentication failed or validation criteria not met`。
  这是客户端侧问题，同一节点用 xray 官方客户端可正常出网；建议改用 `auto` / `tls` 预设
- Surge 不支持 Reality，订阅转换时会自动跳过该节点
- 网页 SSH 默认安装后即对外开放端口，用完请卸载

---

## 与上游的兼容性说明

合并上游更新时的冲突高发文件：

```
database/model/model.go            本分支给 Inbound 增加了周期重置字段
web/service/inbound.go             新增 subId 补齐逻辑
web/html/index.html                主题管理器替换了单双号切换
web/html/inbounds.html             一键配置弹窗整体替换 + 订阅导出改造
web/translation/translate.*.toml   新增文案
web/web.go                         新增定时任务与订阅/被控端路由
```

合并后执行：

```bash
go build ./... && go vet ./... && go test ./...
```

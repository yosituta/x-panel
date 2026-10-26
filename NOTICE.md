# NOTICE / 来源与许可声明

本仓库（下称“本项目”）是 **GPLv3 开源衍生项目**，非原创作品，也不是 X-Panel 官方版本。

## 上游来源

| 层级 | 项目 | 关系 |
| --- | --- | --- |
| 直接上游 | [xeefei/X-Panel](https://github.com/xeefei/X-Panel) | 本项目代码基底，逐次同步其 `main` 分支 |
| 源头项目 | [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui)（及 x-ui） | X-Panel 的上游，Xray 面板本体 |

## 许可

- 本项目整体遵循 **GNU General Public License v3.0**，全文见 `LICENSE`。
- 依 GPLv3 第 5 条，本项目**保留上游全部版权声明**，并同样以 GPLv3 发布；任何人再分发本项目时也必须遵守 GPLv3（提供完整对应源码、保留声明）。
- 本项目**不包含** X-Panel 官方“付费 Pro 版”的任何私有代码、二进制、授权服务器逻辑或授权码校验逻辑。上游 Pro 版是以「授权码 + 远程脚本下发」形式分发的闭源分发物，其代码从未出现在公开仓库中，本项目无法、也没有获取它。
- 因此，本项目中的“Pro 增强功能”是**依据上游 README 公开的功能描述，独立重新实现**的等价功能，不涉及逆向、破解或绕过任何技术保护措施。

## 商标与命名

- “X-Panel”“X-Panel Pro”为上游作者使用的名称，本项目仅作来源说明之用。
- 本项目使用独立名称，不自称官方版本、不冒充上游作者、不复用其收费渠道（授权码机器人、授权服务器等）。

## 修改说明（GPLv3 §5a）

本项目对上游代码做了实质性修改，逐项记录在 [`CHANGELOG.md`](CHANGELOG.md)。修改日期：2026-10 起持续维护。

涉及的文件主要是：

- 新增：`web/service/{traffic_reset,reality_domain,batch_deploy,subscribe_*,alert*,backup,presence,report,relay*,panels*,webssh}.go`、`web/controller/{sub,slave,panels}.go`、`web/job/*`、`backup_cmd.go`、`scripts/*`
- 修改：`main.go`、`web/web.go`、`web/controller/{inbound,server}.go`、`web/service/{inbound,setting,xray,tgbot}.go`、`database/model/model.go`、`web/html/{index,inbounds,settings}.html`、`web/translation/*.toml`、`install.sh`、`x-ui.sh`

## 免责

本项目为学习与技术研究用途的开源软件。使用者需自行遵守所在国家/地区法律法规，作者不对使用后果承担责任。

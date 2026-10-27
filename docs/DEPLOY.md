# 部署指南

面向「一台到多台服务器」的安装与维护。目标是：**一条命令装好，装好就能用，多台不串台。**

---

## 一、先理解两个域名

每台服务器建议配两个域名，用途完全不同：

```
panelN.example.com     灰云（DNS only）→ 直接解析到服务器 IP   → 用来开面板
cdnN.example.com       橙云（Proxied） → Cloudflare 边缘          → 用来连节点
```

- **面板不要走 CDN**：面板端口（如 23333）Cloudflare 不代理；走 CDN 只会 520。
- **节点域名建议走 CDN**：IP 被封时能靠 CDN 换入口。
- 一台服务器一个 `cdnN`，不要多个 A 记录轮询。

`panelN` 与 `cdnN` 用相同的序号，面板会自动配对（`panel2` ↔ `cdn2`），多台也不会串台。

---

## 二、单台安装

### 方式 A：交互式（和 3x-ui 一样）

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh)
```

按提示填账号、密码、端口、路径，回车用默认值。

### 方式 B：无人值守（推荐，多台一致）

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh) \
  --auto --domain panel1.example.com
```

默认值：端口 `54321`、路径 `/`、账号 `admin` / 密码 `123456`（登录后请立即修改）。安装结束会打印：

```
登录地址 : https://panel1.example.com:54321/
用户名   : admin
密码     : 123456        ← 默认值，登录后请立即修改
```

全部可用参数：

| 参数 | 说明 | 默认 |
| --- | --- | --- |
| `--auto` | 启用无人值守 | — |
| `--domain` | 面板域名，填了就自动申请 Let's Encrypt 证书 | 不申请 |
| `--email` | 一般不用填；内部自动用 `admin@<域名主域>` 注册 | `admin@<域名主域>` |
| `--panel-port` | 面板端口 | `54321` |
| `--path` | 访问路径 | `/`（不加密路径） |
| `--user` / `--pass` | 登录凭据 | `admin` / `123456`（登录后请立即改） |
| `--no-cert` | 明确不申请证书 | — |
| `--version` | 指定版本，如 `1.1.0` | 最新 Release |
| `--dry-run` | 只打印计划，不做任何修改 | — |

先看计划再执行：

```bash
bash install.sh --auto --domain panel1.example.com --dry-run
```

### 安装做了什么

1. 装依赖（curl/wget/tar/socat/cron）
2. 预装 acme.sh 并确保 cron 生效；证书剩余 **15 天**时自动续期（`--days 15`）
3. 从 GitHub Release 下载安装包（**Xray 内核随包固定，不追最新**）
4. 安装到 `/usr/local/x-ui`，注册 `x-ui.service` 并开机自启
5. 非交互写入端口 / 路径 / 账号 / 密码
6. 有域名则用 acme.sh standalone 签发证书并绑定到面板
7. 防火墙自动处理：ufw 装了就直接**启用**并放行面板端口 / 22 / 80；firewalld 同理
8. 启动服务并自检，打印登录地址

---

## 三、多台批量

命名先定好，后面全自动：

```
panel1.example.com  /  cdn1.example.com   →  服务器 A
panel2.example.com  /  cdn2.example.com   →  服务器 B
panel3.example.com  /  cdn3.example.com   →  服务器 C
```

在每台上执行（把域名换成该台的）：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh) \
  --auto --domain panel2.example.com
```

如果有 SSH 密钥，可以在一台机器上批量下发：

```bash
for n in 1 2 3; do
  ssh root@10.0.0.$n "bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh) \
    --auto --domain panel$n.example.com"
done
```

---

## 四、装完必做：生成节点

1. 浏览器打开 `https://panelN.example.com:<端口>/`
2. 入站列表 → **一键配置**
3. 选一个预设（无需任何填写）：
   - `1` VLESS + TCP + Reality + Vision — 直连最快
   - `2` VLESS + XHTTP + Reality — 抗封锁更强，仍直连
   - `3` VLESS Encryption + TLS — 可套 CDN
4. 生成后弹二维码，直接扫码导入

如果这台机器的 `cdnN` 已经在 Cloudflare 开了橙云，选 `3` 时结果里会多出一条 `CDN` 链接：

```
[2053]  venc-01        [复制]
[CDN]   优先使用这条    [复制]   ← 用它
```

端口会被自动挑成 `2053 / 2083 / 2087 / 2096 / 8443 / 443`——这是 Cloudflare 唯一能回源的端口段。

---

## 四之二、订阅系统（SublinkPro）

单台面板的 `/sub/<subId>` 只分发本机节点。要把**几十台服务器**的节点汇成一个订阅，用独立的订阅系统：

```bash
x-ui → 25
  1. 订阅系统安装/修复
  2. 订阅系统状态查看
  3. 订阅系统卸载
```

安装脚本会：下载 SublinkPro → 注册 systemd 服务（本机 8000）→ 用面板现有证书做 nginx 反代到 **15268** → 自动放行防火墙。

访问 `https://你的域名:15268/`，默认账号 `admin` / `123456`（登录后立即修改）。
登录后把各台服务器的分享链接粘进去，创建订阅并勾选节点，即可得到一条包含全部节点的订阅链接。
## 五、升级与回滚

```bash
# 升级到最新 Release（保留数据库与证书）
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh) --auto --version 1.1.1

# 数据快照（升级前建议先做）
x-ui backup

# 回滚到某个快照
x-ui backup -restore /etc/x-ui/backups/x-ui-YYYYmmdd-HHMMSS.db.gz
```

面板和 xray 在同一进程里，升级会短暂断流（几秒）。

---

## 六、内核版本策略（重要）

Xray 内核**固定在 25.9.11**，随 Release 打包，安装时不联网拉最新。

原因：Xray 26.7 起 REALITY 改用 X25519MLKEM768（后量子混合密钥交换），老客户端（iOS Shadowrocket、mihomo 1.19.x）握手失败，表现为「能连、延迟正常、但一个字节都传不出去」，服务端日志：

```
REALITY: processed invalid connection ... authentication failed or validation criteria not met
```

实测：`25.9.11` 可用；`26.9.9`、`26.10.21` 不行。详见 CHANGELOG 与 XTLS/Xray-core#6477。

需要新内核的用户可在面板【设置 → Xray 版本】自行切换，后果自负。

---

## 七、排错速查

| 现象 | 原因 | 处理 |
| --- | --- | --- |
| 打开面板报 **520** | 用 CDN 域名访问了面板（CF 把 443 回源到 VLESS 节点） | 改用 `panelN:端口/路径` |
| 打开面板报 **522** | CF 连不到源站 | 检查源站端口是否在 CF 支持列表、服务是否在跑 |
| 节点能连但**零流量** | 通常是 Reality 客户端不兼容 | 换预设 3（TLS），或确认内核没被升级 |
| 证书申请失败 | 域名没解析到本机 / 80 被占 / 被墙 | 用 `x-ui` 菜单 18 手动申请，或先用 `--no-cert` |
| 面板打不开、SSH 正常 | 浏览器 HSTS 缓存 / 本地代理 | 清 `chrome://net-internals/#hsts`，或把域名设为 DIRECT |
| `xray -test` 报 fallbacks 与 decryption 冲突 | VLESS 加密不能带 fallbacks | 已在本仓库修复，勿手改 |

看日志：

```bash
journalctl -u x-ui -f          # 面板与 xray 主日志
tail -f /usr/local/x-ui/error.log
```

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

### 没有域名的情况

`--auto` 不带 `--domain` 即自动按此模式安装，不需要额外加 `--no-cert`（写了也等价）。

端口和有域名时**完全一样**，区别只在证书：

```
有域名  → 面板 https://域名:54321    订阅系统 https://域名:15268   （Let's Encrypt，15 天自动续签）
无域名  → 面板 https://IP:54321      订阅系统 https://IP:15268     （自签证书，浏览器点"继续访问"）
```

- 自签证书由订阅系统安装脚本自动生成（`/etc/ssl/sublinkpro/`，有效期 10 年），不需要你操作
- 后端 8000 只监听本机，不对外暴露；对外始终是 15268
- 面板里的【订阅转换】按钮固定打开 `https://主机:15268`，两种模式一致

**以后补域名（推荐流程）**

1. 域名 A 记录指向本机 IP（建议灰云，不要开 CDN 代理）
2. 确认 80 端口可达：`ufw allow 80/tcp`
3. `x-ui → 18 (SSL 证书管理) → 1 (获取 SSL 证书)`，输入域名
4. 脚本自动完成：签发 → 安装到 `/root/cert/域名/` → 绑定面板 → 配置 acme.sh 定时任务（**剩余 15 天自动续签**）
5. 装完证书后订阅系统会自动重配 nginx，从自签证书换成正式证书，地址变为 `https://域名:15268`

全程不需要重装，节点与订阅数据都不受影响。

### 给订阅系统单独一个域名

除了 `https://面板域名:15268`，还可以给订阅系统绑一个更好记的域名（如 `sub.bbua.pp.ua`）：

```bash
x-ui → 25 → 4
  订阅系统域名 : sub.example.com
  端口         : 8443
```

脚本会自动：申请该域名的 Let's Encrypt 证书（剩余 15 天自动续签）→ 写 nginx 独立 server 块 → 放行端口。

```
https://sub.example.com:8443/      ← 走独立域名证书
https://面板域名:15268/            ← 原入口保留，两个都能用
```

**端口为什么不是 443？** 443 和 2053 通常已经被节点（VLESS+TLS / CDN）占用，nginx 抢不到。
可选 8443 / 2083 / 2087 / 2096（都在 Cloudflare 的回源端口白名单里）。

**想彻底不带端口？** 在 Cloudflare 给 `sub.example.com` 开橙云，源站端口填 8443，SSL 模式选 **Full**，
然后访问 `https://sub.example.com/` 即可（Cloudflare 帮你转发到 8443）。

前提：`sub.example.com` 的 A 记录要先指向本机 IP，80 端口可达（签发证书要用）。

## 证书方案怎么选（推荐顺序）

| 你的情况 | 用哪个 | 怎么装 |
| --- | --- | --- |
| **有域名**（绝大多数情况） | **Let's Encrypt** | `install.sh --auto --domain panel1.example.com` |
| **域名在 Cloudflare，想把源站藏起来** | **Cloudflare 源站证书**（15 年免续） | `x-ui → 18 → 8` |
| **没有域名 / 只是自己试** | **自签** | `install.sh --auto` 会自动生成 |

### 关键前提：节点不能套 Cloudflare

节点握手是 VLESS/TLS 私有协议，Cloudflare 不认，所以**节点域名必须灰云直连**。
面板和订阅系统则可以开橙云藏起来。三条线分开：

```
节点   us1.example.com     灰云    Let's Encrypt      ← 绝对不能开橙云
面板   panel.example.com   橙云    Cloudflare Origin  ← 15 年免续
订阅   sub.example.com     橙云    Cloudflare Origin  ← 15 年免续
```

如果只有一台机器、一个域名，就**全部用 Let's Encrypt + 灰云**，这是最省心、兼容性最好的方案。

### 三种证书对比

| | Let's Encrypt | Cloudflare Origin CA | 自签 |
| --- | --- | --- | --- |
| 有效期 | 90 天 | 最长 15 年 | 10 年（我们自己设的） |
| 续签 | 自动，剩余 15 天 | 不需要 | 不需要 |
| 浏览器信任 | 是 | 只有在 Cloudflare 后面才信任 | 否（提示不安全） |
| 需要域名 | 是 | 是 | 不需要 |
| 能否灰云直连 | 能 | 不能 | 能 |
| 端口限制 | 无 | 必须在 CF 白名单（443/2053/2083/2087/2096/8443） | 无 |
| 适合 | 通用 | 面板/订阅藏源站 | 测试、无域名 |

### 无域名（自签）时是什么样

- 面板：`https://IP:端口/路径`，自签证书，浏览器提示"不安全"点继续即可
- 订阅系统：`https://IP:15268`，同样是自签
- 面板密码不会明文传输（仍走 TLS）
- 之后补域名随时能升级：`x-ui → 18 → 1`（LE）或 `18 → 8`（CF），不用重装

### 换成正式证书后要做什么

1. 面板：`x-ui → 18 → 1`，输入域名即可，脚本自动签发、绑定、配置续签
2. 订阅系统：证书装好后重跑 `x-ui → 25 → 1`，会自动切到新证书
3. 如果面板端口不在 CF 白名单（比如 54321），开橙云前先改成 8443

## 用 Cloudflare 源站证书（15 年，不用续签）

域名托管在 Cloudflare 的话，可以不用 Let's Encrypt，直接用 **Origin CA 证书**：有效期最长 15 年，
不用管续签，Cloudflare 回源时天然信任。

代价是三个前提，缺一不可：

1. **域名必须开橙云（Proxied）** —— Origin 证书只被 Cloudflare 信任，浏览器直连源站会报"证书不受信任"
2. **面板端口必须是 CF 支持的回源端口** —— `443 / 2053 / 2083 / 2087 / 2096 / 8443`；
   默认的 `54321` 不在白名单里，开橙云后浏览器连不上，得先把面板改到 `8443`
3. **灰云 / IP 直连会失效** —— 这是原厂证书的固有行为，不是配置错误

### 操作

```
x-ui → 18（SSL 证书管理）→ 8（Cloudflare 源站证书）
```

两种方式：

- **1. 粘贴证书**（推荐，最稳）
  控制台 `SSL/TLS → Origin Server → Create Certificate`，填域名、有效期选 15 年，
  把 `Origin Certificate` 和 `Private Key` 分别粘进去，输入 `END` 结束。
  脚本会校验私钥与证书是否配对，再写入 `/root/cert/<域名>/` 并重启面板。
- **2. 用凭据自动签发**
  需要 Cloudflare 凭据，二选一：
  - **Origin CA Key**：控制台 `SSL/TLS → Origin Server` 页面可见
  - **API Token**：权限 `Zone → SSL and Certificates → Edit`

凭据保存在 `/root/cert/../.cf-originca`（600 权限），下次不用再输。

### 面板端口怎么改到 8443

```
x-ui → 9（修改面板端口）→ 输入 8443
```

改完记得在 Cloudflare 把 `panel1.example.com` 开橙云，然后访问 `https://panel1.example.com:8443`。
（`8443` 在 CF 白名单内，橙云能正常回源。）

### 订阅系统也能用这张证书

```bash
bash /usr/local/x-ui/scripts/cfcert.sh paste sub.example.com --target sub
```

它会把证书写到 `/root/cert/sub.example.com/`，再重配订阅系统的 nginx 指向它。

### 与 Let's Encrypt 怎么选

| | Cloudflare Origin（15 年） | Let's Encrypt（90 天） |
| --- | --- | --- |
| 续签 | 不需要 | 到期前 15 天自动续 |
| 必须开橙云 | ✅ | ❌ 灰云也能用 |
| 直连 IP 可用 | ❌ | ✅ |
| 端口限制 | 必须 CF 白名单端口 | 无 |
| 适用 | 面板/订阅系统全走 CF | 想直连、想减少依赖 |

### 端口分配与排序

- 批量部署：端口在 **40000-65000** 随机取，同一批内不重复
- 手动指定起始端口：从该端口顺序分配
- 入站列表：按创建先后（ID 升序）显示，与端口无关
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

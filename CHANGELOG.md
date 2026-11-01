# 变更记录

**[简体中文](CHANGELOG.md)** · [English](CHANGELOG.en.md)

本文件按 GPLv3 要求记录本分支相对上游 [xeefei/X-Panel](https://github.com/xeefei/X-Panel) 的修改。

基线：上游 `main`（2026-10 拉取）。面板版本号沿用上游 `26.9.13`，功能迭代以本文为准。

---

---

## 续期后自动换上证书（v1.4.10，2026-11-01）

- **修掉「证书签成功了、浏览器还是不安全」**：`18 → 3 强制更新` 以前只更新
  `/root/cert/<目录>` 里的文件，如果面板绑的是另一个目录（例如 `selfsigned`），
  面板根本没用上新证书。现在续期成功后会检测面板当前绑定：
  - 就是这张 → 提示「重启面板后生效」
  - 不是这张 → 打印两者路径，并询问「要把它设为面板证书吗？[Y/n]」，选 Y 立即绑定
- 根因说明：证书"存在于磁盘"和"面板正在用"是两件事，只有 `x-ui cert -webCert` 绑定 + 重启才生效。

实测（真实服务器，完整复现）：

```
初始：面板绑定 /root/cert/selfsigned（issuer=CN=us1.bbua.pp.ua，自签）
18 → 3 → 输入 y
  面板当前绑定的是：/root/cert/selfsigned/fullchain.pem
  刚续期的是：/root/cert/us1.bbua.pp.ua/fullchain.pem
  已绑定到面板 → 重启后生效
重启后面板实际提供：issuer=C=US, O=Let's Encrypt, CN=YE1（2027-01-30 到期）
```

---

---

---

---

---

## 修复设置页选项卡点不动（v1.4.15，2026-11-01）

- **根因**：v1.4.14 新增的「证书到期提醒」卡片，后端在未配置时把 `channels` 序列化成 `null`，
  前端模板对 `null` 调 `.includes()` 抛 TypeError → Vue 渲染中断 → 之后所有交互（含切换选项卡）全部失效。
- **修复**：后端保证 `channels`/`days`/`lastSent` 永远是空数组/空对象（不返回 null）；
  前端 `loadCertNotify` 统一归一化并在异常时兜底，接口出错也不影响整页。
- 教训：给 Vue 模板喂数据时，Go 的 nil slice 必须显式初始化。

## 证书到期提醒（v1.4.14，2026-11-01）

背景：acme standalone 续期要独占 80 端口，机器上常有 nginx/宝塔占着 → 自动续期会失败。
所以加"到期前主动提醒"，让人及时腾端口。

- 面板设置 → 证书 / CDN → 新增「证书到期提醒」卡片
- **渠道可多选**：Telegram（复用 Telegram 机器人配置里的 Token / Chat ID）、ntfy、Webhook
- **提醒天数自填**（如 `15,7,3`），同一提醒点每天最多推一次（状态记录在 `certNotifyState`）
- 状态卡实时显示证书剩余天数；检测到 80 端口被占用会提示先 `systemctl stop nginx`
- 「发送测试」立即验证渠道
- 后端每小时检查一次（启动后 30 秒开始），日志关键字 `[证书提醒]`

接口：

```
POST {basePath}panel/api/certnotify/status   查看配置/证书情况
POST {basePath}panel/api/certnotify/save     保存（channels、days 用逗号分隔字符串）
POST {basePath}panel/api/certnotify/test     发一条测试
```

Webhook 负载：`{title, text, source:"x-panel", type:"cert-expiry", sentAt}`；
ntfy：`POST <server>/<topic>` + `Title`/`Tags: warning` 头。

测试（Go 单测，本地 httptest 冒充接收端）：

```
--- PASS: TestCertNotifySendChannels   （ntfy 与 webhook 都收到请求，字段/头正确）
--- PASS: TestCertNotifyMissingConfig  （缺参数时返回错误，不静默成功）
```

## 修复「改访问路径保存了却不生效」（v1.4.13，2026-11-01）

- **根因**：面板设置页的重启按钮调用的是 `/panel/api/setting/restartPanel`，
  而真实路由是 `{basePath}panel/setting/restartPanel`（`/api` 那段根本不存在）。
  所以点「重启面板」实际是 404，面板没重启 → 新路径只写进了数据库，进程仍用旧路径。
- **修复**：
  - 重启接口改为 `/panel/setting/restartPanel`
  - 重启后自动跳转到「当前设置的访问路径」（改了路径也能自动落到新地址，不再停在旧 URL）
  - 轮询用的 `/panel/api/uiinfo` 不走 axios 的 baseURL，改为自己拼 basePath
- 实测（真实服务器）：

```
改成 /xp-test 并重启后：
  旧路径 /panel/api/uiinfo          → 404
  新路径 /xp-test/panel/api/uiinfo  → 200
  重启接口（新路径）                → 307（路由存在，需登录）
  登录页 /xp-test/                  → 200
恢复成 / 之后：/panel/api/uiinfo    → 200
```

## 卸载一并清理证书（v1.4.12，2026-11-01）

- **`x-ui → 5` 卸载现在会处理证书**：列出 `/root/cert` 里本项目用过的证书目录，
  确认后逐个删除、并在 CA 端撤销（Let's Encrypt 有记录时）；随后询问是否一并删除
  `/root/.acme.sh`、`/etc/letsencrypt` 和 acme 续期 cron。全部同意后 `/root/cert` 空目录也会被删掉。
- 仍保留一次确认（只回车 = 删除），避免误删你放在同一目录下、别的服务用的证书。
- README 中英各补两条常见问题：**80 端口被 nginx 占用时先停 nginx 再签**、**卸载是否会删证书**。

实测（真实服务器，卸载前后对比）：

```
卸载前 /root/cert: dummy-cert.example.com  selfsigned  us1.bbua.pp.ua
执行 5 → 确认删除
  已删除证书目录: /root/cert/dummy-cert.example.com
  已在 CA 撤销: us1.bbua.pp.ua
  已删除证书目录: /root/cert/us1.bbua.pp.ua
  已删除证书目录: /root/cert/selfsigned
  已删除 acme 状态、/etc/letsencrypt 与续期任务
  已删除空的 /root/cert
PASS: /root/cert 已删除 / PASS: /root/.acme.sh 已删除 / PASS: 程序目录已删除
```

## 证书自动挂载（v1.4.11，2026-11-01）

- **正式证书一签出来就自动挂到面板**，不用再手动 `18 → 7`：
  - 面板无证书 → 直接挂
  - 面板绑的是自签证书 → 直接换成正式证书（自签本该只在应急时用）
  - 面板已绑着另一张正式证书 → 不动，避免覆盖你自己选的证书（18→3 时会问一句）
- **自动续期也会挂上**：新增 `scripts/adopt-cert.sh`，install.sh 把它写进 acme 的 `--reloadcmd`，
  75 天续期成功后自动判断并换绑 + 重启面板。之前续期只换文件不换绑定，就会出现"证书签成、浏览器仍不安全"。
- 18→3 续期成功后顺带问一句「现在重启面板让证书立即生效？[Y/n]」。

实测（真实服务器）：

```
(1) 面板绑定 selfsigned → 实际提供 issuer=CN=us1.bbua.pp.ua（自签）
(2) 执行 adopt-cert.sh us1.bbua.pp.ua
    绑定变成 /root/cert/us1.bbua.pp.ua/fullchain.pem
    实际提供 issuer=C=US, O=Let's Encrypt, CN=YE1
(3) 再执行一次（已是正式证书）→ 绑定不变，服务正常
```

## 续期后自动换上证书（v1.4.10，2026-11-01）

- **撤销证书不再死循环**：`18 → 2` 以前只调用 `acme.sh --revoke`，本地目录 `/root/cert/<名字>` 留着，
  于是"撤销"完列表里还在；对自签/通配证书 acme 还会报 `is not an issued domain, skipping`。
  现在撤销会：CA 有记录就撤销 → **删除本地证书目录** → 若面板正绑着它则解绑 →
  清除 `/root/cert/.panel_domain`（同名时）。
- **`18 → 6` 列表不再只显示一个域名**：以前三张证书都显示成 `hk.bbua.pp.ua`（都走了记录域名），
  根本分不清。现在显示 `目录：bbua.pp.ua → 证书域名：*.bbua.pp.ua ← 当前面板在用`，
  撤销/更新时要输入的就是**目录名**。
- **强制更新 / 获取证书自动处理 80 端口**：以前只 `systemctl stop nginx`，宝塔那套停不掉就失败。
  现在统一助手函数：systemd → `/etc/init.d/nginx` → `/www/server/nginx/sbin/nginx -s stop` 逐个尝试，
  仍占用就明确提示（`bt stop` 或改用 18→8 CF DNS）并取消操作，**不会把证书改坏**；
  续期成功后会重新下发到 `/root/cert/<目录>`，并把之前停掉的 nginx 恢复。

实测（真实服务器）：

```
18→2 撤销 del-test.example.com
  CA 上没有该域名的签发记录（自签/通配属于正常），只删本地
  已删除本地证书目录：/root/cert/del-test.example.com
  PASS: 目录已删除，列表里不再出现

18→3 强制更新（80 被 python3 占住）
  80 端口仍被占用：users:(("python3",pid=981518))
  无法释放 80 端口，本次更新已取消（证书未改动）

18→3 强制更新（80 空闲）
  notBefore: Oct 31 18:18:47 → Nov 1 02:05:03   ← 真实续期成功
```

## 彻底卸载脚本（v1.4.8，2026-11-01）

- 新增 `scripts/purge.sh`：一条命令清干净面板、数据库、证书、订阅系统、acme、日志、防火墙规则、续期 cron。
  - `--check` 只体检并列出将删除的路径与大小，不动任何文件
  - 执行需手动输入 `PURGE` 确认
  - 不碰 nginx 本体和其它站点
- `x-ui → 5` 仍是普通卸载，按设计保留 `/root/cert`；要连证书一起删用 purge 脚本。
- README 中英各加「卸载」小节。

实测（测试机 182.255.80.150，完整跑一遍）：

```
删除 11 项（/opt/x-panel 653M、/etc/x-ui 264K、/opt/sublinkpro 74M、/root/cert、
            /root/.acme.sh 2.7M、/etc/letsencrypt、/usr/bin/x-ui、2 个 systemd 单元等）
撤销 ufw 54321/15268/8000/8443
校验：彻底卸载完成，没有残留
独立复查：目录全无、服务单元全无、54321 无监听、crontab 清空
随后用 install.sh --auto --domain us1.bbua.pp.ua 重装成功（LE 证书签发并绑定）
```

## 访问域名解析修正（v1.4.7，2026-11-01）

- **修掉「显示 bbua.pp.ua:54321」**：证书保存时通配证书 `*.bbua.pp.ua` 的目录会被规范成
  `/root/cert/bbua.pp.ua`，而 `install.sh` 的安装收尾仍用「证书目录名」当域名，于是打印成
  `https://bbua.pp.ua:54321/`（少了 `hk.`）。
  现在统一按优先级取：**证书里的具体域名 → `/root/cert/.panel_domain` → 通配名 → 目录名 → 公网 IP**，
  `x-ui.sh`（查看设置 / 18→6）与 `install.sh`（安装与更新收尾）用同一套规则。
- **面板绑定证书时记录域名**：在面板里保存证书时，若填的是具体域名（非通配），写入
  `/root/cert/.panel_domain`，后续安装/更新据此显示访问地址。
- **去掉 `wg-quick: command not found` 噪音**：WARP 是可选组件，没装时安装脚本不再调用
  `wg-quick`（顺带修正一处 `2A>&1` 的笔误）。

实测（真实服务器，三种证书 × 有无记录）：

```
无 .panel_domain：通配 → *.bbua.pp.ua   具体 → hk.bbua.pp.ua   自签CN=IP → 公网IP
有 .panel_domain=hk.bbua.pp.ua：三种情况全部 → hk.bbua.pp.ua
install.sh 的 panel_bound_host 结果与 x-ui.sh 完全一致
```

## 证书页收尾（v1.4.6，2026-11-01）

- **删掉证书区「保存」旁的「重启面板」按钮**：页面顶部已有同一个按钮，两个入口容易误点。
  顶部按钮现在也走「自动重连」流程（重启期间轮询 `/panel/api/uiinfo`，回来自动刷新）。
- **保存不自动重启**（后端已去掉 2 秒后自动重启），彻底消除连续两次 `Network Error`。
- **状态卡区分「浏览器是否信任」**：
  - Cloudflare 源站证书 → 明确提示「只在 CF 代理后可信，直接访问 `:54321` 必然显示不安全」
  - 自签证书 → 提示「点继续访问可用，要绿锁请换公共 CA 证书」
  - 公共 CA（Let's Encrypt / ZeroSSL / DigiCert 等）→ 不显示警告

> 常见误解：证书"保存成功 + 重启生效"不等于"浏览器绿锁"。
> Cloudflare 那套 15 年源站证书（Origin CA）只被 Cloudflare 边缘节点信任，
> 面板直接开在 `:54321` 上时，任何浏览器都会提示不安全 —— 这是证书类型决定的，不是面板没生效。

## 证书页体验修复（v1.4.5，2026-11-01）

- **「重启面板」不再弹 `Network Error`**：重启会中断当前连接，请求报错是必然的。
  现在点完就显示「正在重启…」，随后每 2 秒探测 `/panel/api/uiinfo`，服务回来自动刷新页面；
  超过 60 秒会提示手动刷新或 `x-ui → 14`。
- **保存证书不再自动重启**：后端原来在保存后 2 秒自动重启，和用户随后点的手动重启撞车，
  前端收到两次 `Network Error`；现在保存只写配置，重启只由「重启面板」触发。
- **修复空白确认框**：`$confirm('内容','标题')` 这种两参数写法在 ant-design-vue 1.x 下不生效，
  弹窗只有图标没有文字；改为 `$confirm({title, content})`（重启、移除证书、解绑 CDN 三处）。
- **Cloudflare 源站证书给出明确提示**：识别到颁发者是 Cloudflare 时，状态卡会说明
  「CF 源站证书只被 Cloudflare 代理信任，直接访问 `https://域名:54321` 浏览器仍提示不安全（不是没生效）」，
  并指向 Let's Encrypt 方案。
- **状态卡域名显示修正**：CF 源站证书的 CN 是 `CloudFlare Origin Certificate`，
  以前被当成域名显示；现在 CN 不是合法域名（含空格或是 IP）时回落到 SAN，再回落到 `/root/cert/.panel_domain`。

## 更新时保留域名（v1.4.4，2026-10-31）

- **`更新面板` 不带 `--domain` 时，沿用上次安装的域名**：域名记录在 `/root/cert/.panel_domain`。
  典型场景：用域名安装但证书签不下来（80 被宝塔 nginx 占用），退回自签；
  之后更新面板，终端里「登录地址」仍是 `https://你的域名:54321/`，`x-ui → 10` 也一样，
  不会再出现 `https://selfsigned:54321/` 或 IP 地址。
- 自签证书的 CN 同步用域名，`18 → 6` 里显示的也是域名。

实测（真实服务器，直接跑 GitHub 上的 install.sh）：

```
不带 --domain → 沿用上次安装的域名：hk-test.example.com
                HTTPS：是，域名 hk-test.example.com
带 --domain   → HTTPS：是，域名 us2.bbua.pp.ua（并更新记录）
```

## 证书访问地址修复（v1.4.3，2026-10-31）

- **登录地址不再显示「selfsigned」**：`x-ui → 10 查看面板设置` 原来把证书所在目录名当域名，
  自签兜底目录 `/root/cert/selfsigned` 就被显示成 `https://selfsigned:54321/`，根本打不开。
  现在从证书的 SAN/CN 解析真实域名；解析不到或是 IP 时回落到服务器公网 IP。
- **自签证书会明确提示**，并自动扫描机器上现存的正式证书（`/root/cert/*`、`/etc/letsencrypt/live/*`），
  直接把重新绑定的命令打出来，不用重新申请。
- **自签也记住你的域名**：证书申请失败退回自签时，自签证书的 CN 用你的域名（不是 IP），
  并写入 `/root/cert/.panel_domain`；登录地址优先读它，所以即使暂时是自签，
  显示的也是 `https://你的域名:54321/`，不会变成 IP 或 selfsigned。
- **`18 → 6 显示现有域名`** 改为显示证书里的真实域名，并标注「自签证书，仅用于应急访问」。
- **更新面板不再覆盖已绑定证书**：此前不带 `--domain` 重装/更新会用自签证书替换掉正式证书，
  现在只要已绑定的证书文件存在且不是自签兜底目录，就保留不动。

## 面板内证书 / CDN 页（v1.4.2，2026-10-31）

### 新增

- **设置 → 证书 / CDN 标签页**：证书与私钥各支持「上传本地文件」和「粘贴内容」两种方式，
  上传后显示文件名、字符数与开头片段；保存前校验私钥是否配对、证书是否覆盖域名，不通过则不写入。
- **保存与重启分离**：`[保存]` 只写配置，`[重启面板]` 单独触发；界面明确提示「保存后需重启面板才生效」。
- **CDN 域名绑定**：输入域名后 `[绑定]` 立即生效（生成节点时现读，无需重启面板）；
  `[解绑]` 清空域名与关联记录，节点恢复直连。
- **界面版本探针**：`GET /panel/api/uiinfo`（免登录）返回当前界面标记，
  用于区分「服务端是旧版」和「浏览器缓存了旧页面」。

### 修复

- **CDN「绑定 / 解绑」按钮不显示（根因）**：页面模板里 `<a-input ... />` 这类自闭合写法
  在浏览器中被当作「开标签」而非「空元素」，导致其后的兄弟节点全部变成它的子节点；
  antd 的 Input 组件不渲染子节点，于是按钮和状态行整块消失。
  已把 `settings.html`、`inbounds.html`、`index.html`、`servers.html`、`login.html`
  共 25 处自闭合组件改为显式闭合标签。
- **证书状态需手动点「刷新」才显示**：进入设置页时未自动拉取状态，改为 `mounted` 时自动加载。
- **CDN 输入框回车无效**：in-DOM 模板会把 `@pressEnter` 小写成 `@pressenter`，Vue 收不到，
  改为外层 div 承接 `keyup.enter`。
- **页面缓存**：页面类响应加 `Cache-Control: no-store, no-cache, must-revalidate`。

## 订阅系统 SublinkPro 与安装修复（2026-10-27）

- **订阅系统**：接入 SublinkPro（Go + React 单文件）。`x-ui` 菜单 25 一键安装/修复/查看/卸载；
  安装脚本自动用面板现有证书做 nginx 反代到 15268 并放行防火墙。登录后可添加各台服务器节点、
  自由创建订阅、在订阅内勾选节点，最终得到一条包含全部节点的订阅链接。
- **面板按钮**：入站页【订阅转换】直接打开 `https://<面板域名>:15268/`，并提示默认账号密码。
- **修复**：发布包此前未包含 `scripts/` 目录，新装机器上菜单 25/27/28/29 会报「未找到脚本」。
  现在 `scripts/` 随 Release 打包；`run_repo_script` 另加在线兜底，本地缺失时从仓库直接拉取。
- **文档**：README 顶部新增「一条命令安装」（交互式 / 无人值守 / --dry-run）；docs/DEPLOY.md 增补订阅系统小节。

## 一键安装与多机部署（2026-10-26）

### install.sh 新增无人值守模式

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh) \
  --auto --domain panel1.example.com --email you@example.com
```

- 支持 `--panel-port / --path / --user / --pass / --domain / --email / --no-cert / --version / --dry-run`
- 未指定时自动生成访问路径与 16 位随机密码，安装完成打印登录地址与凭据
- 预装 acme.sh（与 3x-ui 一致），有域名时直接用 standalone 模式签发并绑定证书
- 自动放行 ufw / firewalld 的面板端口
- 不带 `--auto` 时完全走原来的交互式流程，行为不变

### CDN 域名按序号配对

`panel2.example.com` 会优先嗅探 `cdn2.example.com`，其次才是 `cdn / cdn1 / cdn2 / proxy / cf / ws`，
多台服务器不会串到同一台。

### 新增 docs/DEPLOY.md

域名规划（面板灰云 / 节点橙云）、单台与多台安装、升级回滚、内核版本策略、排错速查。

## CDN 自动识别（2026-10-26）

TLS 系预设（TLS / Trojan / VLESS 加密）生成节点时，会自动判断面板域名是否在 Cloudflare 后面：

- **检测方式**：解析域名，比对 Cloudflare 官方公布的代理网段（22 个 CIDR，含 IPv6），结果缓存 10 分钟
- **命中时自动做的事**：
  1. 入站端口从 Cloudflare 支持的回源端口里挑（2053 / 2083 / 2087 / 2096 / 8443 / 443），否则 CDN 转发不进来
  2. 额外生成一条指向 CDN 域名的分享链接（`vless://…@域名:端口?security=tls&sni=域名&…`），在结果里以 `CDN` 标签列出
- **未命中时**：行为完全不变，不多问一句、不增加任何选项
- 有 CDN 链接时不再自动弹出直连二维码，而是把两条链接一起列出来，避免用户选错

Reality 系预设无法走 CDN（CDN 会终止 TLS），因此不做转换。

## 新增两个预设，一键配置扩到四个选项（2026-10-26）

与上游 `vless_reality / vless_xhttp_reality / vless_tls_encryption / switch_vision_seed_dev` 对齐：

| 序号 | 预设 | 说明 |
| --- | --- | --- |
| 1 | VLESS + TCP + Reality + Vision | 无需域名证书 |
| 2 | VLESS + XHTTP + Reality | 无需证书，路径与模式随机，抗封锁更强 |
| 3 | VLESS Encryption + XHTTP + TLS | 可选套 CDN，需面板已配置证书 |
| 4 | Switch + Vision Seed | 置灰，标注「开发中」 |

实现要点：

- 预设 2 的 `xhttpSettings` 与上游一致（`mode: packet-up`、`scMaxEachPostBytes: 1000000`、`xPaddingBytes: 100-1000` 等），路径随机 8 位
- 预设 3 的密钥对与 `xray vlessenc` 的 X25519 组等价：服务端 `mlkem768x25519plus.native.600s.<私钥>`，客户端 `mlkem768x25519plus.native.0rtt.<公钥>`
- **踩坑**：VLESS 加密（`decryption` 非 `none`）与 `fallbacks` 互斥，同时出现会让 xray 直接拒绝启动，生成时必须省略 `fallbacks`
- 两个新预设均已在 Xray 25.9.11 上用 `xray -test` 校验，并从独立客户端实际出网验证

## 一键配置简化 + 固定 Xray 内核（2026-10-26）

### 根因：REALITY 从 Xray 26.7 起改用后量子混合密钥交换

Reality 节点在本分支原有打包（Xray 26.10.21）下，iOS Shadowrocket 表现为「能连、延迟测试通过，但一个字节都传不出去」，服务端日志：

```
REALITY: processed invalid connection from x.x.x.x: authentication failed or validation criteria not met
```

同一台服务器、同一份入站配置、只更换内核的 A/B 实测：

| Xray 内核 | iOS Shadowrocket |
| --- | --- |
| 25.9.11 | ✅ 正常出网 |
| 26.9.9 | ❌ 认证失败 |
| 26.10.21 | ❌ 认证失败 |

配置侧已排除：`target` / `dest` 两种写法、单域名 / 双 serverNames、2 位 / 8 位 shortId，在 25.9.11 上都能通，在 26.x 上都失败。

真正原因见 Xray-core issue #6477：Xray 26.7 起 REALITY 的握手使用 **X25519MLKEM768**（X25519 + ML-KEM-768 后量子混合），老客户端（Shadowrocket、mihomo 1.19.x 等）只实现 X25519，双方派生的会话密钥不一致，服务端判定认证失败并按普通访客丢弃连接。

### 修复：固定内核到 v25.9.11

`.github/workflows/release.yml` 不再取 `releases/latest`，改为固定 `XRAY_VERSION: v25.9.11`，并在注释里写明实测表与 issue 链接。25.9.11 服务端对 26.x 客户端向后兼容，因此新客户端不受影响；需要新内核的用户可在面板【设置 → Xray 版本】自行切换。

### 变更：一键配置改为三选一

原来的「预设下拉 + 7 个高级选项 + 数量/端口/备注/流量/域名」全部收起，主界面只剩三个按钮，点一下即生成一条节点并直接弹出二维码：

1. VLESS + TCP + Reality + Vision（推荐，无需域名和证书）
2. VLESS + TCP + TLS（需面板已配置证书）
3. Trojan + TCP + TLS（需面板已配置证书）

数量、起始端口、备注前缀、流量、偷用域名等仍在「高级选项」折叠面板中，批量生成走原来的逻辑。

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

## 清理上游痕迹与推广内容（2026-10-26）

移除上游遗留的品牌推广、返利广告与数据上报，使本分支成为独立品牌的产品：

| 位置 | 移除内容 |
| --- | --- |
| `x-ui.sh` | 菜单尾部 9 条 VPS 返利广告、Telegram 交流群、上游博客教程链接 |
| `install.sh` | 安装完成提示中的同类广告与群链接 |
| `web/html/index.html` | 上游 releases 链接、TG 私聊机器人标签、TG 交流群标签 |
| `web/html/navigation.html` | 「交流群与安装教程」「推广赞助」「Pro 介绍」「授权码事宜」四个板块整体删除；保留实用技巧与代理软件说明 |
| `web/html/servers.html` | 「付费 Pro 版专属 / 购买授权码」弹窗改为指向本分支真实实现的【设置 → 多面板】 |
| `web/service/tgbot.go` | TG 菜单的「VPS 推荐」返利广告；中奖上报上游频道（`REPORT_BOT_TOKEN` / `REPORT_CHAT_IDS`，会把用户名、TG ID、主机名发往第三方）；抽奖文案里的上游品牌与交流群 |
| `x-ui.sh` 菜单 25 | 不再安装第三方 Sublink + Nginx，改为提示面板内置的 `/sub/<subId>` 订阅 |

NOTICE 同步精简为最小署名版本：仅声明 GPLv3 衍生关系与修改说明，不再包含任何商业或社区信息。

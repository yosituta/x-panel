import io

# ============ README.md ============
p = 'README.md'
s = io.open(p, encoding='utf-8', newline='').read()
nl = '\r\n' if '\r\n' in s else '\n'


def rep(old, new, label=''):
    global s
    o = old.replace('\n', nl)
    n = new.replace('\n', nl)
    assert o in s, 'anchor missing ' + (label or old[:50])
    s = s.replace(o, n, 1)


# 1) 头部：去掉来源叙述与重复的卖点罗列
rep('''# X-Panel 开源增强版

[简体中文](README.md) · **[English](README.en.md)**

基于 [xeefei/X-Panel](https://github.com/xeefei/X-Panel) 的 **GPLv3 开源分支**。上游把「一键配置」等实用功能移入付费 Pro 版并停止向免费版更新，本分支按公开功能说明重新实现，并补上了作者自己需要的运维能力。

- 面板本体：Go + Gin + SQLite，Xray-core 作为子进程托管
- 一键安装、systemd 托管、单二进制、无外部依赖
- **不包含**任何上游私有代码；**不实现**授权码/积分/购买机器人等商业闭环

> 本仓库与上游作者无隶属关系。上游名称与标识仅用于说明来源与致敬。
''',
    '''# X-Panel 开源增强版

[简体中文](README.md) · **[English](README.en.md)**

**GPLv3** 开源发布、可长期自维护的 Xray 面板。Go + Gin + SQLite，Xray-core 作为子进程托管，单二进制、一条命令安装，无授权码、无积分、无商业闭环。

''')

# 2) 中转节点表格里的「上游」是网络含义，改成「目标」
rep('| 一键中转节点 | 本机入口 → 远程 Socks5/HTTP 上游 → 出网，自动写 outbound 与路由规则 |',
    '| 一键中转节点 | 本机入口 → 远端 Socks5/HTTP 代理 → 出网，自动写 outbound 与路由规则 |')

# 3) 仓库地址：去掉「装原版」段落
rep('''`install.sh` / `x-ui.sh` 里的 `X_PANEL_REPO` 已指向该仓库。想临时装上游原版：

```bash
X_PANEL_REPO=xeefei/x-panel bash install.sh
```

''', '''`install.sh` / `x-ui.sh` 里的 `X_PANEL_REPO` 已指向该仓库。

''')

# 4) 快速开始：删掉与开头重复的安装命令（原来的代码块还残缺）
rep('''## 快速开始

### 一键安装（推荐）

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh)

无人值守安装（多台服务器推荐，可自动申请证书）：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh) \\
  --auto --domain panel1.example.com --email you@example.com
```

完整部署说明（域名规划、多台批量、排错）：[docs/DEPLOY.md](docs/DEPLOY.md)
```

安装完成后按提示设置端口、路径、账号密码。管理脚本：

```bash
x-ui            # 打开管理菜单（含证书、备份、调优、网页 SSH 等）
```
''',
    '''## 快速开始

安装方式见上方「一条命令安装」，装完执行：

```bash
x-ui            # 管理菜单：证书、防火墙、订阅系统、备份、调优、网页 SSH 等
```
''')

# 5) 删除「与上游同步」整节
start = s.index('## 与上游同步')
end = s.index('## 开发与测试')
s = s[:start] + s[end:]

# 6) 目录结构说明
rep('docs/                路线图、上游 README、分析笔记',
    'docs/                路线图、分析笔记')

# 7) FAQ 里网络含义的「上游」
rep('先确认上游 Socks5/HTTP 本身可用；',
    '先确认目标 Socks5/HTTP 代理本身可用；')

io.open(p, 'w', encoding='utf-8', newline='').write(s)
print('README.md OK')

# ============ README.en.md ============
p = 'README.en.md'
s = io.open(p, encoding='utf-8', newline='').read()
nl = '\r\n' if '\r\n' in s else '\n'


def rep2(old, new, label=''):
    global s
    o = old.replace('\n', nl)
    n = new.replace('\n', nl)
    assert o in s, 'anchor missing ' + (label or old[:50])
    s = s.replace(o, n, 1)


rep2('''# X-Panel Open Source Enhanced

[简体中文](README.md) · **[English](README.en.md)**

A **GPLv3 fork** of [xeefei/X-Panel](https://github.com/xeefei/X-Panel). Upstream moved practical features such as one-click deployment into a paid "Pro" edition and stopped updating the free edition. This fork re-implements them from the public feature list and adds the operations tooling we actually needed.

- Panel core: Go + Gin + SQLite, with Xray-core supervised as a child process
- One-line install, systemd managed, single binary, no external dependencies
- Contains **no** upstream proprietary code; does **not** implement license-key checks, points, or purchase bots

> Not affiliated with the upstream author. Upstream names are used only to credit the origin.
''',
     '''# X-Panel Open Source Enhanced

[简体中文](README.md) · **[English](README.en.md)**

An Xray panel released under **GPLv3** and built to be maintained long-term. Go + Gin + SQLite, Xray-core supervised as a child process, single binary, one-line install — no license keys, no points, no commercial paywall.

''')

rep2('| One-click relay node | Local entry → remote Socks5/HTTP upstream → internet; outbound and routing rules written automatically |',
     '| One-click relay node | Local entry → remote Socks5/HTTP proxy → internet; outbound and routing rules written automatically |')

rep2('''`X_PANEL_REPO` in `install.sh` / `x-ui.sh` already points here. To install the upstream original instead:

```bash
X_PANEL_REPO=xeefei/x-panel bash install.sh
```

''', '''`X_PANEL_REPO` in `install.sh` / `x-ui.sh` already points here.

''')

rep2('''## Quick start

### One-line install (recommended)

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh)

Unattended install (recommended for multiple servers; can obtain the certificate automatically):

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh) \\
  --auto --domain panel1.example.com --email you@example.com
```

Full deployment guide (domain layout, multiple servers, troubleshooting): [docs/DEPLOY.md](docs/DEPLOY.md)
```

Then set the port, base path and credentials when prompted. Management menu:

```bash
x-ui            # certificates, snapshots, tuning, web SSH, ...
```
''',
     '''## Quick start

See "One-command install" above; after installation run:

```bash
x-ui            # menu: certificates, firewall, subscription system, snapshots, tuning, web SSH, ...
```
''')

start = s.index('## Syncing with upstream')
end = s.index('## Development and testing')
s = s[:start] + s[end:]

rep2('docs/                roadmap, upstream README, analysis notes',
     'docs/                roadmap, analysis notes')

rep2('Confirm the upstream Socks5/HTTP proxy works on its own first,',
     'Confirm the target Socks5/HTTP proxy works on its own first,')

io.open(p, 'w', encoding='utf-8', newline='').write(s)
print('README.en.md OK')

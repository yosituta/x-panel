import io

p = 'README.en.md'
s = io.open(p, encoding='utf-8', newline='').read()
nl = '\r\n' if '\r\n' in s else '\n'


def rep(old, new, label=''):
    global s
    o = old.replace('\n', nl)
    n = new.replace('\n', nl)
    assert o in s, 'anchor missing ' + (label or old[:50])
    s = s.replace(o, n, 1)


rep('''# X-Panel Open Edition

**English** · [简体中文](README.md)

A **GPLv3 fork** of [xeefei/X-Panel](https://github.com/xeefei/X-Panel). Upstream moved practical features such as one-click deployment into a paid "Pro" edition and stopped updating the free edition. This fork re-implements them from the public feature list and adds the operations tooling we actually needed.

- Panel: Go + Gin + SQLite, Xray-core supervised as a child process
- One-line install, systemd managed, single binary, no external services
- Contains **no** upstream proprietary code; does **not** implement license-key checks, points, or purchase bots

> Not affiliated with the upstream author. Upstream names are used only to credit the origin.
''',
    '''# X-Panel Open Edition

**English** · [简体中文](README.md)

An Xray panel released under **GPLv3** and built to be maintained long-term. Go + Gin + SQLite, Xray-core supervised as a child process, single binary, one-line install — no license keys, no points, no commercial paywall.

''')

rep('| One-click relay node | Local entry → remote Socks5/HTTP upstream → internet; outbound and routing rules written automatically |',
    '| One-click relay node | Local entry → remote Socks5/HTTP proxy → internet; outbound and routing rules written automatically |')

rep('''`X_PANEL_REPO` in `install.sh` / `x-ui.sh` already points here. To install the upstream original instead:

```bash
X_PANEL_REPO=xeefei/x-panel bash install.sh
```

''', '''`X_PANEL_REPO` in `install.sh` / `x-ui.sh` already points here.

''')

rep('''## Quick start

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

rep('docs/                roadmap, upstream README, analysis notes',
    'docs/                roadmap, analysis notes')

rep('Confirm the upstream Socks5/HTTP proxy works on its own first,',
    'Confirm the target Socks5/HTTP proxy works on its own first,')

io.open(p, 'w', encoding='utf-8', newline='').write(s)
print('README.en.md OK')

# X-Panel Open Edition

**English** · [简体中文](README.md)

An Xray panel released under **GPLv3** and built to be maintained long-term. Go + Gin + SQLite, Xray-core supervised as a child process, single binary, one-line install — no license keys, no points, no commercial paywall.

> **Disclaimer**
>
> This project is provided for personal learning and exchange only. Please comply with your local laws and regulations, do not use it for illegal purposes, and do not use it in production environments.
>
> **Note**
>
> When using this project or its [tutorials], the user bears sole responsibility for any consequences arising from use that violates the rules stated above.

## One-command install

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh)
```

Interactive; press Enter to accept the defaults:

- panel port `54321`
- base path `/`
- user `admin` / password `123456` (**change it right after login**)

### Unattended install (recommended, identical across servers)

**A. With a domain (automatic HTTPS, recommended for production)**

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh) \
  --auto --domain panel1.example.com
```

- Issues a Let's Encrypt certificate automatically and **renews when 15 days remain**
- No email needed — `admin@your-domain` is used internally
- Opens firewall ports automatically; management menu installed to `/usr/bin/x-ui`

**B. Without a domain (plain IP, self-signed)**

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh) --auto
```

- Omitting `--domain` selects this mode automatically — **no** `--no-cert` needed (it is still accepted)
- The panel and subscription system use **exactly the same ports** as A; only the certificate is self-signed, so the browser asks you to click through once

| | A. With domain | B. No domain |
| --- | --- | --- |
| Panel | `https://your-domain:54321` | `https://IP:54321` (self-signed) |
| Subscription system | `https://your-domain:15268` | `https://IP:15268` (self-signed) |
| Certificate | Let's Encrypt, auto-renew at 15 days | self-signed, 10 years |
| CDN / Reality | both supported | both supported (CDN still needs a domain) |

**Adding a domain later (mode B)**: point the A record at the server → `x-ui → 18 → 1` to issue a certificate (automatic issue, automatic binding, auto-renew at 15 days)
→ the subscription system switches to the real certificate automatically, the URL becomes `https://your-domain:15268`, data is untouched.

Preview the plan without changing anything:

```bash
bash install.sh --auto --domain panel1.example.com --dry-run
```

Other flags: `--panel-port` / `--path` / `--user` / `--pass` / `--no-cert` / `--version`.

Full deployment guide (domain layout, multiple servers, upgrades, troubleshooting): [docs/DEPLOY.md](docs/DEPLOY.md)

## Docker

If you would rather not install the systemd stack:

```bash
git clone https://github.com/yosituta/x-panel.git
cd x-panel && mkdir -p db cert
docker compose up -d --build
```

Open `https://SERVER_IP:54321`, user `admin` / `123456` (change it right away).

- The image pins Xray **v25.9.11** (Reality compatibility — do not bump)
- `network_mode: host` is required so node ports bind directly
- The container has no systemd/ufw/nginx: **certificates, firewall and the subscription system** are handled on the host

Full guide (env vars, mounting certificates, upgrades, troubleshooting): [docs/DOCKER.md](docs/DOCKER.md)

### Ports and ordering

- Batch-deployed nodes get **random** ports in **40000-65000** (unique per batch) instead of incrementing from 20000
- If you pass an explicit start port, ports are allocated sequentially from it
- The inbound list is ordered by **creation order (ascending ID)**, not by port number


## Features

### Proxy & nodes

| Feature | Details |
| --- | --- |
| Multi-protocol inbounds | VLESS / VMess / Trojan / Shadowsocks over TCP / WS / gRPC / HTTP |
| Batch deploy | Generate N nodes at once, each with its own port/UUID/keys, hot-reloaded with share links |
| Presets | `auto` (default) / TLS / Trojan / Reality (Reality is tucked under "advanced presets") |
| Random Reality target | Concurrent TLS1.3 liveness probing before picking a disguise domain |
| Built-in subscription converter | `/sub/<subId>` emits V2Ray / Clash / sing-box / Surge — no third-party subconverter |
| One-click relay node | Local entry → remote Socks5/HTTP proxy → internet; outbound and routing rules written automatically |

### Traffic & users

| Feature | Details |
| --- | --- |
| Periodic quota reset | Daily / weekly / monthly on day N / never, with short-month and leap-year handling |
| Traffic & expiry alerts | Notify at 80% usage (configurable) or N days before expiry, once per threshold |
| Online/offline notices | Compares `lastOnline`, pushes only on state change, first run just builds a baseline |

### Operations

| Feature | Details |
| --- | --- |
| Snapshots + rescue restore | Consistent snapshots via `VACUUM INTO`; one CLI command restores even when the panel is down |
| Web SSH | ttyd + basic auth + panel certificate for HTTPS, `x-ui` menu 26 |
| Line / IP quality check | Egress info, latency and loss to four DNS resolvers, return path, bandwidth probe — menu 27 |
| Regional DNS check | Eight resolvers compared side by side, flags possible poisoning — menu 28 |
| Kernel tuning | BBR + FQ + TFO + socket buffers, idempotent apply/status/revert — menu 29 |

### UI & collaboration

| Feature | Details |
| --- | --- |
| Five dashboard themes | Standard / Vivid / Deep Sea / Twilight / Forest, remembered after selection |
| Multi-panel master/slave | One panel manages others; slaves authenticate via `X-Slave-Key`, disabled by default |
| Scheduled reports | Daily/weekly/monthly at a chosen time, selectable sections, preview and send-now |
| Telegram | `/link` for node links, `/panels` `/panel` for multi-panel, `/webssh` for web terminal info |

## Repository

This project lives at **https://github.com/yosituta/x-panel**.

`X_PANEL_REPO` in `install.sh` / `x-ui.sh` already points here.

Release flow: commit, `git tag v1.0.1 && git push origin v1.0.1`, and GitHub Actions builds amd64/arm64 (CGO) and attaches the artifacts to the Release.

Pre-release checks:

```bash
go build ./... && go vet ./... && go test ./...
bash -n install.sh x-ui.sh scripts/*.sh
git ls-files | grep -E "\.db$|\.pem$|\.key$|\.env$" || echo "no secrets tracked"
```

## Quick start

See "One-command install" above; after installation run:

```bash
x-ui            # menu: certificates, firewall, subscription system, snapshots, tuning, web SSH, ...
```

### Build from source

Requires **Go 1.27+** and CGO (the sqlite driver needs it):

```bash
# Linux
sudo apt install -y build-essential git
CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o x-ui .

# Windows (MinGW-w64 or zig required)
set CGO_ENABLED=1
go build -trimpath -ldflags "-s -w" -o x-ui.exe .
```

Avoid building on a 1-core / 512 MB box — cross-compile or use GitHub Actions instead:

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 \
  CC="zig cc -target x86_64-linux-gnu" \
  go build -trimpath -ldflags "-s -w" -o x-ui .
```

## After the first login

1. **Change the default port and base path** — `x-ui` → 9 / 7
2. **Enable HTTPS** — `x-ui` → 18 (certificate), or point to your own cert paths
3. **Pick the `auto` preset** — issues TLS nodes when a certificate and domain are available (best compatibility)
4. **Take a snapshot first** — Settings → Snapshots → Back up now, or `x-ui backup`
5. **Uninstall Web SSH when you are done with it** — `x-ui` → 26 → uninstall

## Subscriptions

Every client gets a 16-character `subId`; one URL, four formats:

```
https://panel/path/sub/<subId>                 # V2Ray (base64)
https://panel/path/sub/<subId>?target=clash    # Clash / Mihomo
https://panel/path/sub/<subId>?target=singbox  # sing-box
https://panel/path/sub/<subId>?target=surge    # Surge
```

Suffixes work too: `.yaml` / `.json` / `.conf`. Responses carry `Subscription-Userinfo`, so clients can show usage and expiry.

## CLI

```bash
x-ui                      # interactive menu
x-ui setting -show        # show panel settings
x-ui setting -port 23333  # change port
x-ui backup               # take a snapshot now
x-ui backup -list         # list snapshots
x-ui backup -restore x-ui-20260101-090000.db.gz   # restore (stop the panel first)
x-ui -v                   # version
```

## Certificate expiry reminders

acme renewals need exclusive use of port 80, which nginx or BaoTa often holds, so automatic renewal
can fail. The panel therefore **warns you before the certificate expires** so you can free the port in
time.

Settings → Certificate / CDN → "Certificate expiry reminders":

- channels are **multi-select**: Telegram (reuses the token and chat IDs from the Telegram bot
  settings), ntfy, Webhook
- reminder days are configurable, e.g. `15,7,3` fires 15, 7 and 3 days before expiry (at most once per
  day per reminder point)
- the card shows the days remaining for the current certificate and warns when port 80 is occupied
- "Send test" verifies the channels immediately

Webhook posts JSON:

```json
{ "title": "证书到期提醒", "text": "domain … expires … 3 days left …",
  "source": "x-panel", "type": "cert-expiry", "sentAt": "2026-11-01T12:00:00+08:00" }
```

ntfy is `POST <server>/<topic>` with `Title` and `Tags: warning` headers; subscribe to the same topic
in the ntfy app.

## Uninstall

**Normal uninstall** (`x-ui → 5`): stops the service, removes the binary, deletes the database in
`/etc/x-ui`, removes the menu command and the firewall rule for the panel port. It then lists the
certificates this project used under `/root/cert` and, after confirmation, **deletes them and revokes
them at the CA**, and asks whether to remove `/root/.acme.sh`, `/etc/letsencrypt` and the renewal cron.

**Full purge** (database, certificates, subscription system, acme state and logs included):

```bash
# dry run: list everything that would be deleted, touch nothing
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/scripts/purge.sh) --check

# execute (you must type PURGE to confirm)
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/scripts/purge.sh)
```

Removes `/usr/local/x-ui`, `/opt/x-panel`, `/etc/x-ui` (database, inbounds, accounts),
`/opt/sublinkpro`, `/root/cert`, `/root/.acme.sh`, `/etc/letsencrypt`, `/etc/ssl/sublinkpro`,
`/usr/bin/x-ui`, the related systemd units, the `54321/15268/8000/8443` firewall rules and the acme
renewal cron. It never touches nginx itself or your other sites.

## Development & tests

```bash
go test ./web ./web/service -run Test      # unit tests
bash scripts/e2e-smoke.sh                  # end-to-end smoke test (running panel required)
bash scripts/deploy-vps.sh                 # deploy to a VPS
```

Covered: quota-reset scheduling, node and subscription rendering in four formats, alert thresholds, online/offline detection, snapshot compression and SQLite header validation, template parsing.

## Layout

```
main.go              entry point (run / setting / migrate / backup)
web/                 panel: controllers, services, HTML templates, assets
xray/                Xray process and gRPC API wrapper
database/            SQLite models and DAO
config/              configuration
scripts/             ops scripts (tuning, DNS, line check, ttyd, e2e, deploy, sync)
docs/                roadmap, analysis notes
```

## FAQ

**Port 80 is held by nginx, so issuing or renewing a certificate keeps failing?**

acme's standalone mode needs exclusive use of port 80. Stop nginx before issuing or renewing, start it
again afterwards:

```bash
systemctl stop nginx     # before issuing/renewing (BaoTa users: stopping the panel is not enough)
                          #   then x-ui → 18 → 1 (issue) or 18 → 3 (force renew)
systemctl start nginx    # restore after the certificate is in place
```

The script now tries to stop nginx itself (systemd → `/etc/init.d/nginx` →
`/www/server/nginx/sbin/nginx -s stop`). If it cannot, the operation is cancelled without touching your
certificate and you are told to use `18 → 8` (Cloudflare DNS, no port 80 needed).

**Are certificates removed when I uninstall?**

Yes. `x-ui → 5` lists the certificates this project used under `/root/cert`, deletes them after
confirmation and revokes them at the CA (when a Let's Encrypt record exists); it also asks whether to
remove `/root/.acme.sh`, `/etc/letsencrypt` and the renewal cron. Use `scripts/purge.sh` to wipe
everything including the database and the subscription system.

**Reality nodes show a good ping but pages do not load.**
If the server log says `REALITY: authentication failed`, the client's Reality implementation is incompatible with xray (observed with some Shadowrocket builds). Switch to the `auto` or `TLS` preset — this is not a server misconfiguration.

**Typing the address without http/https doesn't load?**
No prefix needed. The panel detects it: with a certificate configured, a plain `http://` request returns `308` to the same URL over `https://`; without a certificate it is served over `http://` directly. After one visit by domain, HSTS makes the browser use https from then on.

**Subscription URLs contain 127.0.0.1.**
Address resolution order: subscription domain → panel certificate directory name → machine hostname. Set "subscription domain" in settings, or open the panel by domain rather than IP.

**A relay node was created but does not work.**
Confirm the target Socks5/HTTP proxy works on its own first, then look for `failed to dial` in the xray log. Remove it with `POST /panel/api/inbounds/relay/remove/<port>`, which also cleans the outbound and routing rule.

**Uploads to the VPS keep dying.**
Chunked upload (`split -b 2M`) with per-chunk verification, or build directly on the server.

## Credits & license

- Panel skeleton from [xeefei/X-Panel](https://github.com/xeefei/X-Panel) (GPLv3) and [XTLS/Xray-core](https://github.com/XTLS/Xray-core)
- This fork is released under **GPLv3** as well — see [LICENSE](LICENSE) and [NOTICE.en.md](NOTICE.en.md)
- Per GPLv3: redistributions must keep the copyright notices and license, and state the changes made (see [CHANGELOG.en.md](CHANGELOG.en.md))

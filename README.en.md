# X-Panel Open Edition

**English** · [简体中文](README.md)

A **GPLv3 fork** of [xeefei/X-Panel](https://github.com/xeefei/X-Panel). Upstream moved practical features such as one-click deployment into a paid "Pro" edition and stopped updating the free edition. This fork re-implements them from the public feature list and adds the operations tooling we actually needed.

- Panel: Go + Gin + SQLite, Xray-core supervised as a child process
- One-line install, systemd managed, single binary, no external services
- Contains **no** upstream proprietary code; does **not** implement license-key checks, points, or purchase bots

> Not affiliated with the upstream author. Upstream names are used only to credit the origin.

## Features

### Proxy & nodes

| Feature | Details |
| --- | --- |
| Multi-protocol inbounds | VLESS / VMess / Trojan / Shadowsocks over TCP / WS / gRPC / HTTP |
| Batch deploy | Generate N nodes at once, each with its own port/UUID/keys, hot-reloaded with share links |
| Presets | `auto` (default) / TLS / Trojan / Reality (Reality is tucked under "advanced presets") |
| Random Reality target | Concurrent TLS1.3 liveness probing before picking a disguise domain |
| Built-in subscription converter | `/sub/<subId>` emits V2Ray / Clash / sing-box / Surge — no third-party subconverter |
| One-click relay node | Local entry → remote Socks5/HTTP upstream → internet; outbound and routing rules written automatically |

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

`X_PANEL_REPO` in `install.sh` / `x-ui.sh` already points here. To install the upstream original instead:

```bash
X_PANEL_REPO=xeefei/x-panel bash install.sh
```

Release flow: commit, `git tag v1.0.1 && git push origin v1.0.1`, and GitHub Actions builds amd64/arm64 (CGO) and attaches the artifacts to the Release.

Pre-release checks:

```bash
go build ./... && go vet ./... && go test ./...
bash -n install.sh x-ui.sh scripts/*.sh
git ls-files | grep -E "\.db$|\.pem$|\.key$|\.env$" || echo "no secrets tracked"
```

## Quick start

### One-line install (recommended)

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh)
```

Then set the port, base path and credentials when prompted. Management menu:

```bash
x-ui            # certificates, snapshots, tuning, web SSH, ...
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

## Syncing with upstream

```bash
git remote add upstream https://github.com/xeefei/X-Panel.git   # first time only
./scripts/sync-upstream.sh --check   # any new upstream commits?
./scripts/sync-upstream.sh           # merge upstream
```

Always re-verify after a merge:

```bash
go build ./... && go vet ./... && go test ./...
```

Conflict-prone files: `database/model/model.go`, `web/service/inbound.go`, `web/html/index.html`, `web/html/inbounds.html`, `web/translation/*.toml`.

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
docs/                roadmap, upstream README, analysis notes
```

## FAQ

**Reality nodes show a good ping but pages do not load.**
If the server log says `REALITY: authentication failed`, the client's Reality implementation is incompatible with xray (observed with some Shadowrocket builds). Switch to the `auto` or `TLS` preset — this is not a server misconfiguration.

**Subscription URLs contain 127.0.0.1.**
Address resolution order: subscription domain → panel certificate directory name → machine hostname. Set "subscription domain" in settings, or open the panel by domain rather than IP.

**A relay node was created but does not work.**
Confirm the upstream Socks5/HTTP proxy works on its own first, then look for `failed to dial` in the xray log. Remove it with `POST /panel/api/inbounds/relay/remove/<port>`, which also cleans the outbound and routing rule.

**Uploads to the VPS keep dying.**
Chunked upload (`split -b 2M`) with per-chunk verification, or build directly on the server.

## Credits & license

- Panel skeleton from [xeefei/X-Panel](https://github.com/xeefei/X-Panel) (GPLv3) and [XTLS/Xray-core](https://github.com/XTLS/Xray-core)
- This fork is released under **GPLv3** as well — see [LICENSE](LICENSE) and [NOTICE.en.md](NOTICE.en.md)
- Per GPLv3: redistributions must keep the copyright notices and license, and state the changes made (see [CHANGELOG.en.md](CHANGELOG.en.md))

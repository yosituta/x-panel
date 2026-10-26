# Changelog

[简体中文](CHANGELOG.md) · **English**

Records every change this fork makes against upstream [xeefei/X-Panel](https://github.com/xeefei/X-Panel), as required by GPLv3.

Baseline: upstream `main` (pulled 2026-10). The panel version string still follows upstream `26.9.13`; this document is the source of truth for feature work.

---

## Automatic CDN detection (2026-10-26)

When a TLS-based preset (TLS / Trojan / VLESS Encryption) creates a node, the panel now checks whether its own domain sits behind Cloudflare:

- **Detection**: resolve the domain and match it against Cloudflare's published proxy ranges (22 CIDRs, IPv6 included); the result is cached for 10 minutes
- **When it matches**:
  1. the inbound port is chosen from Cloudflare's supported origin ports (2053 / 2083 / 2087 / 2096 / 8443 / 443) — otherwise the CDN cannot reach the origin
  2. an extra share link pointing at the CDN domain is emitted and listed with a `CDN` tag
- **When it does not match**: nothing changes — no extra questions, no extra options
- With a CDN link present, the direct-connect QR code is no longer opened automatically; both links are listed so the user does not pick the wrong one

Reality-based presets cannot go through a CDN (the CDN terminates TLS), so they are left untouched.

## Two more presets — one-click setup now offers four options (2026-10-26)

Aligned with upstream `vless_reality / vless_xhttp_reality / vless_tls_encryption / switch_vision_seed_dev`:

| # | Preset | Notes |
| --- | --- | --- |
| 1 | VLESS + TCP + Reality + Vision | no domain or certificate needed |
| 2 | VLESS + XHTTP + Reality | no certificate, random path and mode, harder to block |
| 3 | VLESS Encryption + XHTTP + TLS | CDN-friendly, requires a certificate in the panel |
| 4 | Switch + Vision Seed | disabled, marked "in development" |

Implementation notes:

- Preset 2 uses the same `xhttpSettings` as upstream (`mode: packet-up`, `scMaxEachPostBytes: 1000000`, `xPaddingBytes: 100-1000`), with a random 8-character path
- Preset 3 generates a key pair equivalent to `xray vlessenc`'s X25519 output: server `mlkem768x25519plus.native.600s.<private>`, client `mlkem768x25519plus.native.0rtt.<public>`
- **Pitfall**: VLESS encryption (`decryption` other than `none`) is mutually exclusive with `fallbacks`; having both makes Xray refuse to start, so `fallbacks` must be omitted
- Both presets were validated with `xray -test` on Xray 25.9.11 and exercised end-to-end from a separate client

## Simpler one-click setup + pinned Xray core (2026-10-26)

### Root cause: REALITY switched to a post-quantum hybrid key exchange in Xray 26.7

With this fork's previous bundle (Xray 26.10.21), Reality inbounds connect from iOS Shadowrocket and pass the latency test, but not a single byte gets through. The server logs:

```
REALITY: processed invalid connection from x.x.x.x: authentication failed or validation criteria not met
```

A/B test on one machine with an identical inbound, changing only the core:

| Xray core | iOS Shadowrocket |
| --- | --- |
| 25.9.11 | works |
| 26.9.9 | fails |
| 26.10.21 | fails |

Configuration was ruled out: `target` vs. `dest`, single vs. double `serverNames`, and 2-char vs. 8-char `shortId` all work on 25.9.11 and all fail on 26.x.

The actual cause is Xray-core issue #6477: since 26.7, REALITY negotiates **X25519MLKEM768** (a hybrid of X25519 and ML-KEM-768). Clients that only implement X25519 (Shadowrocket, mihomo 1.19.x and similar) derive a different session key, so the server treats them as failed authentication and falls back to serving the camouflage site.

### Fix: pin the core to v25.9.11

`.github/workflows/release.yml` no longer resolves `releases/latest`; it pins `XRAY_VERSION: v25.9.11` with the measurements and the issue link in a comment. A 25.9.11 server stays backward compatible with 26.x clients, so newer clients are unaffected; users who want a newer core can switch it under Settings → Xray version.

### Change: one-click setup reduced to three choices

The preset dropdown and the seven advanced fields are gone from the main dialog. Three buttons remain; one click creates a single inbound and opens its QR code immediately:

1. VLESS + TCP + Reality + Vision (recommended, no domain or certificate)
2. VLESS + TCP + TLS (requires a certificate in the panel)
3. Trojan + TCP + TLS (requires a certificate in the panel)

Count, start port, remark prefix, traffic quota and camouflage domain stay under an "Advanced" collapse for batch creation.

## v1.0.0 (2026-10-26)

First complete release: all 22 technical features described in the public upstream README are re-implemented, plus 9 features of our own.

### Added · proxy & nodes

- **Batch deploy**: create N nodes at once, each with its own port / UUID / x25519 key / shortId, hot-reloaded, share links returned
- **Preset system**: `auto` / `tls` / `trojan` / `reality` / `reality-novision` / `reality-udp443` / `reality-legacy`
  - `auto` is the default: TLS when a certificate and domain are present, otherwise Reality
  - The Reality family sits behind an "advanced presets" toggle so nobody picks a protocol their client cannot use
- **Random Reality disguise domain**: concurrent TLS1.3 liveness probing, CDN-fronted domains filtered out
- **Built-in subscription converter**: `/sub/<subId>` emits V2Ray / Clash / sing-box / Surge with a `Subscription-Userinfo` header
- **One-click relay node**: local entry → remote Socks5/HTTP → internet; outbound and routing rules are written automatically and removed together on delete

### Added · traffic & users

- **Periodic quota reset**: daily / weekly / monthly on day N / never; short months and leap years handled, idempotent across cycles
- **Traffic & expiry alerts**: tiered notifications at 80% / 100% usage and 3 days / 1 day / expired, once per threshold, re-armed once the condition clears
- **Online/offline notices**: derived from `clientTraffic.lastOnline`; the first run only records a baseline

### Added · operations

- **Snapshots and rescue restore**: consistent snapshots via `VACUUM INTO` (safe while the panel runs), gzip-compressed, rotated by count; restore validates the SQLite header and keeps the previous database
  - CLI: `x-ui backup` / `-list` / `-restore <file>`
  - API: `POST /panel/api/server/backup`, `GET /backups`, `POST /restore/:name`
  - UI: Settings → Snapshots
- **Web SSH (ttyd)**: `scripts/ttyd.sh` + `x-ui` menu 26, basic auth over HTTPS using the panel certificate
- **Line / IP quality check**: `scripts/linecheck.sh`, menu 27
- **Regional DNS check**: `scripts/dnscheck.sh`, menu 28
- **Kernel tuning**: `scripts/tune.sh`, menu 29 (BBR + FQ + TFO + buffers, idempotent apply/status/revert)

### Added · UI & collaboration

- **Five dashboard themes**: Standard / Vivid / Deep Sea / Twilight / Forest, selected manually and persisted
- **Multi-panel master/slave**: slaves expose `/panel/api/slave/{status,links,restartXray}` (`X-Slave-Key`, disabled by default); masters use `/panel/api/panels/*` plus a "Multi-panel" settings tab
- **Scheduled reports**: daily / weekly / monthly at a chosen time; sections (server, traffic and top five, inbounds and expiry, online clients) are selectable; preview and send-now supported
- **Telegram command extensions**: `/link [keyword]`, `/panels`, `/panel <name> status|links|restart`, `/webssh`

### Fixed

- `xray/api.go`: dereferencing the gRPC client before Xray was initialised caused a panic → added `handlerClient()` returning a clear error
- `web/service/xray.go`: calling `err.Error()` when `GetErr()` returned nil crashed the process
- Periodic-reset fields were silently dropped when editing an inbound (missing copy on the update path)
- Batch deploy used the wrong unit for quota (GB not converted to bytes)
- Generated one-click links were empty when no host was available
- Historical clients had no `subId` → backfilled automatically at startup

### Removed relative to upstream

- The "paid Pro edition" install branch and license-purchase prompts removed from the installer
- "Buy a license bot" copy removed from the Telegram bot; it now points at the in-panel one-click deploy
- Not implemented (and not planned): online license validation, points/check-in/leaderboards, points redemption, purchase bot, membership badges

### Known limitations

- Reality is incompatible with some client implementations (observed with certain Shadowrocket builds); the server log shows
  `REALITY: authentication failed or validation criteria not met`.
  This is client-side — the same node works with the official xray client. Use the `auto` or `tls` preset instead
- Surge does not support Reality; the subscription converter skips those nodes
- Web SSH opens a port to the internet once installed — uninstall it when you are done

---

## Upstream compatibility notes

Conflict-prone files when merging upstream:

```
database/model/model.go            this fork added periodic-reset fields to Inbound
web/service/inbound.go             added subId backfill
web/html/index.html                theme manager replaced the odd/even switch
web/html/inbounds.html             one-click modal replaced; subscription export reworked
web/translation/translate.*.toml   new strings
web/web.go                         new cron jobs and subscription/slave routes
```

Always re-verify after a merge:

```bash
go build ./... && go vet ./... && go test ./...
```

## Upstream traces and promotional content removed (2026-10-26)

The fork no longer carries the upstream project's branding, affiliate advertising, or telemetry:

| Location | Removed |
| --- | --- |
| `x-ui.sh` | nine VPS affiliate links in the menu footer, the Telegram group, and the upstream blog tutorial link |
| `install.sh` | the same advertising and group links in the completion message |
| `web/html/index.html` | upstream releases link, Telegram bot tag, Telegram group tag |
| `web/html/navigation.html` | the four tabs "community & install guide", "sponsorship", "Pro introduction" and "license keys"; the practical tips and client guides remain |
| `web/html/servers.html` | the "paid Pro only / buy a license key" dialog now points at this fork's real multi-panel feature under Settings |
| `web/service/tgbot.go` | the "VPS recommendations" affiliate entry in the bot menu; the win-reporting to upstream channels (`REPORT_BOT_TOKEN` / `REPORT_CHAT_IDS`, which sent usernames, Telegram IDs and hostnames to a third party); upstream branding in the lottery text |
| `x-ui.sh` menu 25 | no longer installs third-party Sublink + Nginx; it explains the built-in `/sub/<subId>` endpoint instead |

NOTICE was reduced to the minimum attribution required by GPLv3: the derivative-work statement and a pointer to the modification log.

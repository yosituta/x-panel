# Changelog

[简体中文](CHANGELOG.md) · **English**

Records every change this fork makes against upstream [xeefei/X-Panel](https://github.com/xeefei/X-Panel), as required by GPLv3.

Baseline: upstream `main` (pulled 2026-10). The panel version string still follows upstream `26.9.13`; this document is the source of truth for feature work.

---

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

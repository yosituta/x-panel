# Changelog

[简体中文](CHANGELOG.md) · **English**

Records every change this fork makes against upstream [xeefei/X-Panel](https://github.com/xeefei/X-Panel), as required by GPLv3.

Baseline: upstream `main` (pulled 2026-10). The panel version string still follows upstream `26.9.13`; this document is the source of truth for feature work.

---

---

## Auto-bind after renewal (v1.4.10, 2026-11-01)

- **"Certificate renewed but the browser still warns" fixed**: `18 → 3 renew` used to update only the
  files under `/root/cert/<dir>`; when the panel was bound to a different directory (e.g. `selfsigned`)
  it never used the new certificate. After a successful renewal the script now compares the panel's
  binding: same file → "restart to take effect"; different file → prints both paths and asks
  "set it as the panel certificate? [Y/n]", binding immediately on Y.
- Root cause: a certificate sitting on disk and the panel actually using it are two different things —
  only `x-ui cert -webCert` plus a restart flips what the panel serves.

Verified end-to-end on a live server:

```
initial: panel bound to /root/cert/selfsigned (issuer=CN=us1.bbua.pp.ua, self-signed)
18 → 3 → answer y
  panel currently bound: /root/cert/selfsigned/fullchain.pem
  just renewed:          /root/cert/us1.bbua.pp.ua/fullchain.pem
  bound to panel → restart required
after restart the panel serves: issuer=C=US, O=Let's Encrypt, CN=YE1 (expires 2027-01-30)
```

## Certificate revoke & renewal fixes (v1.4.9, 2026-11-01)

- **Revoking a certificate no longer loops forever**: `18 → 2` used to call only `acme.sh --revoke`,
  leaving `/root/cert/<name>` in place, so the entry stayed in the list; for self-signed or wildcard
  certificates acme also printed `is not an issued domain, skipping`. Revoking now revokes at the CA if
  a record exists, **deletes the local certificate directory**, unbinds it if the panel is using it and
  clears `/root/cert/.panel_domain` when it matches.
- **`18 → 6` lists every certificate distinctly**: all three used to render as `hk.bbua.pp.ua` (each
  resolved through the recorded domain), which made them indistinguishable. It now prints
  `目录：bbua.pp.ua → 证书域名：*.bbua.pp.ua ← 当前面板在用`, and the name to type when revoking or
  renewing is the **directory name**.
- **Renew/issue now handles port 80 automatically**: previously only `systemctl stop nginx` was tried, so
  BaoTa's nginx blocked issuance. One shared helper now tries systemd → `/etc/init.d/nginx` →
  `/www/server/nginx/sbin/nginx -s stop`; if it is still busy the operation is cancelled with a clear
  hint (`bt stop`, or use 18→8 Cloudflare DNS) and **the certificate is left untouched**. After a
  successful renewal the files are re-installed into `/root/cert/<dir>` and nginx is restored.

Verified on a live server:

```
18→2 revoke del-test.example.com
  no CA record (normal for self-signed/wildcard), local only
  removed /root/cert/del-test.example.com
  PASS: directory gone, entry no longer listed

18→3 renew with port 80 held by python3
  80 still busy: users:(("python3",pid=981518))
  cannot free port 80, renewal cancelled (certificate untouched)

18→3 renew with port 80 free
  notBefore: Oct 31 18:18:47 → Nov 1 02:05:03   (real renewal)
```

## Full purge script (v1.4.8, 2026-11-01)

- New `scripts/purge.sh`: one command that wipes the panel, database, certificates, subscription
  system, acme state, logs, firewall rules and the renewal cron.
  - `--check` only reports what would be deleted (with sizes) and touches nothing
  - executing requires typing `PURGE` to confirm
  - never touches nginx itself or other sites
- `x-ui → 5` remains the normal uninstall and keeps `/root/cert` by design; use the purge script when
  the certificates should go too.
- Added an "Uninstall" section to both READMEs.

Verified on the test machine (182.255.80.150), full cycle:

```
removed 11 targets (/opt/x-panel 653M, /etc/x-ui 264K, /opt/sublinkpro 74M, /root/cert,
                    /root/.acme.sh 2.7M, /etc/letsencrypt, /usr/bin/x-ui, 2 systemd units, …)
revoked ufw rules 54321/15268/8000/8443
self-check: "彻底卸载完成，没有残留"
independent re-check: no directories, no units, nothing on 54321, empty crontab
then reinstalled with install.sh --auto --domain us1.bbua.pp.ua (LE certificate issued and bound)
```

## Access-host resolution fixed (v1.4.7, 2026-11-01)

- **"bbua.pp.ua:54321" display fixed**: saving a wildcard certificate (`*.bbua.pp.ua`) normalises the
  directory to `/root/cert/bbua.pp.ua`, and the installer's closing summary still used that directory
  name as the host — printing `https://bbua.pp.ua:54321/` (missing `hk.`). Both `install.sh` and
  `x-ui.sh` now resolve in one order: **concrete host in the certificate → `/root/cert/.panel_domain`
  → wildcard name → directory name → public IP**.
- **Binding a certificate records the domain**: when a concrete (non-wildcard) host is entered in the
  panel, it is written to `/root/cert/.panel_domain` so later installs/updates show the right address.
- **`wg-quick: command not found` noise removed**: WARP is optional, so the installer no longer calls
  `wg-quick` when it is absent (also fixed a `2A>&1` typo).

Verified on a live server across three certificate types with and without the record:

```
no .panel_domain: wildcard → *.bbua.pp.ua   concrete → hk.bbua.pp.ua   self-signed CN=IP → public IP
with .panel_domain=hk.bbua.pp.ua: all three → hk.bbua.pp.ua
install.sh's panel_bound_host matches x-ui.sh exactly
```

## Certificate page wrap-up (v1.4.6, 2026-11-01)

- **Removed the "Restart panel" button next to "Save"** in the certificate card — the page header
  already has one, and having two entry points invited misclicks. The header button now uses the same
  auto-reconnect flow (polls `/panel/api/uiinfo` while restarting, reloads when the service is back).
- **Saving never auto-restarts** (the backend's 2-second delayed restart is gone), which eliminates the
  double `Network Error` toast.
- **Status card now says whether browsers trust the certificate**:
  - Cloudflare Origin certificate → explains it is only trusted behind Cloudflare's proxy, so hitting
    `:54321` directly will always show "not secure"
  - self-signed → "click through to continue; use a public CA certificate for a green lock"
  - public CA (Let's Encrypt / ZeroSSL / DigiCert …) → no warning

> Common misunderstanding: "saved + restarted" does not mean "green padlock".
> Cloudflare's 15-year Origin CA certificate is only trusted by Cloudflare's edge, so with the panel
> exposed directly on `:54321` every browser warns — that is the certificate type, not the panel.

## Certificate page UX (v1.4.5, 2026-11-01)

- **"Restart panel" no longer throws `Network Error`**: the restart drops the connection, so the request
  is bound to fail. The UI now shows "restarting…", polls `/panel/api/uiinfo` every 2 s and reloads the
  page automatically once the service is back (after 60 s it suggests a manual refresh or `x-ui → 14`).
- **Saving a certificate no longer auto-restarts the panel**: the backend used to restart 2 s after save,
  colliding with the user's own restart click and producing two `Network Error` toasts; saving now only
  writes the configuration, and the restart happens solely via "Restart panel".
- **Empty confirmation dialogs fixed**: `$confirm('content','title')` is not supported by
  ant-design-vue 1.x (the modal rendered with no text); switched to `$confirm({title, content})`
  for restart, remove-certificate and unbind-CDN.
- **Cloudflare Origin certificates are explained**: when the issuer is Cloudflare the status card says the
  certificate is only trusted behind Cloudflare's proxy, so hitting `https://domain:54321` directly still
  shows "not secure" (which does not mean the upload failed), and points at Let's Encrypt instead.
- **Status card domain fixed**: a CF Origin certificate's CN is `CloudFlare Origin Certificate`, which used
  to be displayed as the domain; when CN is not a valid hostname (spaces, or an IP) it now falls back to
  the SAN, then to `/root/cert/.panel_domain`.

## Domain preserved across updates (v1.4.4, 2026-10-31)

- **`Update panel` without `--domain` reuses the previously installed domain**, stored in
  `/root/cert/.panel_domain`. Typical case: installed with a domain but issuance failed (port 80 held
  by BaoTa's nginx) and the panel fell back to a self-signed certificate — after an update the printed
  login address and `x-ui → 10` both stay `https://your-domain:54321/` instead of
  `https://selfsigned:54321/` or a bare IP.
- The self-signed certificate uses the domain as CN, so `18 → 6` shows the domain too.

Verified against the real `install.sh` on GitHub from a live server:

```
no --domain  → 沿用上次安装的域名：hk-test.example.com
               HTTPS: yes, domain hk-test.example.com
with --domain → HTTPS: yes, domain us2.bbua.pp.ua (record updated)
```

## Certificate access URL (v1.4.3, 2026-10-31)

- **Login URL no longer shows "selfsigned"**: `x-ui → 10 Show panel settings` used the certificate's
  directory name as the host, so the self-signed fallback directory `/root/cert/selfsigned` produced
  `https://selfsigned:54321/`, which cannot resolve. The host is now parsed from the certificate's
  SAN/CN, falling back to the server's public IP when it is an IP or missing.
- **Self-signed certificates are called out explicitly**, and the script scans for existing real
  certificates on the machine (`/root/cert/*`, `/etc/letsencrypt/live/*`) and prints the exact
  re-bind command instead of asking you to re-issue.
- **The self-signed fallback remembers your domain**: when issuance fails, the fallback certificate
  uses your domain as CN (not the IP) and the domain is stored in `/root/cert/.panel_domain`.
  The login URL prefers that value, so the panel shows `https://your-domain:54321/` even while the
  certificate is self-signed, instead of an IP or "selfsigned".
- **`18 → 6 Show existing domains`** now prints the real domain from the certificate and marks
  self-signed ones as "emergency access only".
- **Updating/ reinstalling no longer overwrites a bound certificate**: previously running the installer
  without `--domain` replaced a real certificate with the self-signed fallback; now an existing
  non-self-signed binding is kept as-is.

## In-panel certificate / CDN page (v1.4.2, 2026-10-31)

### Added

- **Settings → Certificate / CDN tab**: certificate and key can each be uploaded from a local file
  or pasted as text; after upload the file name, character count and a preview are shown.
  Before saving, the key pair and domain coverage are validated — invalid input is not written.
- **Save and restart are separate**: `[Save]` only writes the configuration, `[Restart panel]` is a
  distinct action, and the UI states that a restart is required for a certificate to take effect.
- **CDN domain binding**: type a domain, press `[Bind]` — effective immediately, no restart needed
  (read at node-generation time). `[Unbind]` clears the domain and related state so nodes go direct again.
- **UI version probe**: `GET /panel/api/uiinfo` (no login required) returns the current UI marker,
  so you can tell "server is old" from "browser cached an old page".

### Fixed

- **CDN "Bind / Unbind" buttons were not rendered (root cause)**: self-closing component tags such as
  `<a-input ... />` are parsed by browsers as *open* tags, so every following sibling became a child of
  the input. antd's Input does not render child nodes, so the buttons and the status row vanished.
  All 25 self-closing component tags across `settings.html`, `inbounds.html`, `index.html`,
  `servers.html` and `login.html` now use explicit closing tags.
- **Certificate status needed a manual refresh**: state was not fetched on page load; now loaded in `mounted`.
- **Enter key in the CDN input did nothing**: in-DOM templates lowercase `@pressEnter` to `@pressenter`,
  which Vue never receives; a wrapper div now handles `keyup.enter`.
- **Page caching**: page responses now send `Cache-Control: no-store, no-cache, must-revalidate`.

## One-command install and multi-server deployment (2026-10-26)

### `install.sh` gains an unattended mode

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh) \
  --auto --domain panel1.example.com --email you@example.com
```

- Flags: `--panel-port / --path / --user / --pass / --domain / --email / --no-cert / --version / --dry-run`
- Generates the access path and a random 16-character password when omitted, then prints the URL and credentials
- Preinstalls acme.sh (same as 3x-ui) and issues + binds a certificate in standalone mode when a domain is given
- Opens the panel port in ufw / firewalld automatically
- Without `--auto` the original interactive flow is unchanged

### CDN domains pair by index

`panel2.example.com` probes `cdn2.example.com` first, then falls back to `cdn / cdn1 / cdn2 / proxy / cf / ws`,
so multiple servers do not pick each other's CDN hostname.

### New docs/DEPLOY.md

Domain layout (panel DNS-only, node proxied), single and multi-server install, upgrade and rollback,
the pinned-core policy, and a troubleshooting table.

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

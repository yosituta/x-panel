# Telegram MTProto 代理（mtg v2 / FakeTLS）

> English summary at the bottom.

Xray 没有 MTProto 协议，所以面板本身创建不出「MTProto 节点」。
本功能用独立服务 [9seconds/mtg](https://github.com/9seconds/mtg) 提供 **Telegram 专用**的
MTProto 代理，并把流量伪装成正常的 HTTPS。

## 什么时候用

| 场景 | 建议 |
| --- | --- |
| 只想过 Telegram，且网络会干扰普通代理 | ✅ 用这个（FakeTLS，DPI 看到的是 HTTPS） |
| 浏览器、MEGA、Foxmail、指纹浏览器 | ❌ 用 VLESS+Reality / socks，这个只服务 Telegram |
| 想要「用证书加密」 | ❌ 无关；FakeTLS 借用的是伪装域名的真实证书链，本地不签发证书 |

## 使用

```bash
x-ui → 30
  1. 安装 / 修复（自动生成 secret、写 systemd、放行端口）
  2. 查看连接链接（tg:// 一键添加）
  3. 状态
  4. 卸载
```

等价脚本：

```bash
bash /usr/local/x-ui/scripts/mtg.sh install [伪装域名] [端口]
bash /usr/local/x-ui/scripts/mtg.sh link
bash /usr/local/x-ui/scripts/mtg.sh status
bash /usr/local/x-ui/scripts/mtg.sh uninstall
```

- 端口默认 **8443**（443 通常被 Xray 的 Reality 节点占用）
- **安装时一定会问伪装域名**（回车才用默认 `www.microsoft.com`）：
  - 要选当地能正常访问的大站（`www.microsoft.com`、`www.cloudflare.com`）
  - **不要填自己的面板域名**：它指向本机，FakeTLS 的探针回退会落回自己，伪装效果差很多。
    脚本检测到这种情况会警告并要求确认，答 `n` 则取消且不改动现有配置。
  - 换域名/端口 = 重新执行 `30 → 1`，secret 会重新生成，客户端需要重新添加
- 客户端：Telegram → 设置 → 高级 → 连接类型 → 代理 → **MTProto**，
  填 `服务器IP:端口` + `secret`，或直接点 `tg://` 链接 / 扫二维码

## 验证方法

```bash
systemctl is-active mtg            # active
ss -tlnp | grep 8443               # LISTEN

# 模拟 DPI 看到的内容（应显示伪装域名的真实证书，且校验通过）
openssl s_client -connect <服务器IP>:8443 -servername www.microsoft.com </dev/null 2>&1 \
  | grep -E "subject=|issuer=|Verify return code"
```

正常输出形如：

```
subject=C=US, ST=WA, L=Redmond, O=Microsoft Corporation, CN=www.microsoft.com
issuer=C=US, O=Microsoft Corporation, CN=Microsoft TLS G2 RSA CA OCSP 04
Verify return code: 0 (ok)
```

> 注意：能通过 TLS 握手**不代表**这是一个可以浏览的网站——握手之后是 MTProto 协议，
> 浏览器访问会停在那里，这正是伪装想要的效果。

## 排错

| 现象 | 原因 / 处理 |
| --- | --- |
| `permission denied ... /etc/mtg.toml` | 旧版单元用了 `DynamicUser=true`，升级到 v1.4.20+ 已改为 `User=root` |
| 手机加不上代理 | 云厂商安全组没放行端口（脚本只能处理 ufw/firewalld） |
| 能连但很慢 | 伪装域名在当地不可达；换一个能正常访问的大站域名后重跑安装 |
| 想换端口/域名 | 重跑 `30 → 1`，secret 会重新生成，客户端要重新添加 |

## 文件位置

```
/usr/local/bin/mtg          二进制
/etc/mtg.toml               配置（secret + bind-to，权限 600）
/etc/systemd/system/mtg.service
```

---

## English summary

Xray has no MTProto protocol, so X-Panel cannot create an MTProto inbound. This feature runs
[9seconds/mtg](https://github.com/9seconds/mtg) as a separate **Telegram-only** proxy with FakeTLS,
so DPI sees an ordinary HTTPS session.

```bash
x-ui → 30            # 1 install / 2 show tg:// link / 3 status / 4 uninstall
bash /usr/local/x-ui/scripts/mtg.sh install [cloak-domain] [port]
```

- default port **8443** (443 is usually occupied by Xray/Reality)
- the cloak domain must be a reachable popular HTTPS site; mtg pulls its real certificate during the
  handshake, so no local certificate is issued
- verify with `openssl s_client -connect <ip>:8443 -servername <cloak-domain>` — it should show the
  cloak domain's genuine certificate with `Verify return code: 0`
- Telegram only: browsers, MEGA and mail clients cannot use it

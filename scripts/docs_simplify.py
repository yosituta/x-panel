import io

p = 'docs/DEPLOY.md'
s = io.open(p, encoding='utf-8', newline='').read()
nl = '\r\n' if '\r\n' in s else '\n'
s = s.replace('\r\n', '\n')

# 1) 删掉两段“多域名/CF 证书”大段：从「给订阅系统单独一个域名」到「端口分配与排序」之前
start = s.index('### 给订阅系统单独一个域名')
end = s.index('### 端口分配与排序')
s = s[:start] + s[end:]

# 2) 把「证书方案怎么选」整节换成简短推荐
start = s.index('## 证书方案怎么选（推荐顺序）')
end = s.index('### 端口分配与排序')
simple = '''## 证书怎么选（只用一个域名）

**默认就一条路：一个域名 + Let's Encrypt。**

```bash
bash <(curl -Ls https://raw.githubusercontent.com/yosituta/x-panel/main/install.sh) --auto --domain 你的域名
```

- 面板：`https://域名:54321/路径`
- 订阅系统：`https://域名:15268/`（同一张证书）
- 证书：Let's Encrypt，90 天有效，**剩余 15 天自动续签**，配好不用管

域名建议保持 **灰云**（直接指向本机 IP）。原因只有一个，但很关键：

> 节点握手是 VLESS/TLS 私有协议，Cloudflare 认不了。节点域名一旦开橙云，节点会全部失效。

**没有域名**：`install.sh --auto` 会自动生成自签证书，面板和订阅系统都是 HTTPS，
浏览器提示"不安全"点继续即可；以后补了域名，`x-ui → 18 → 1` 随时换成正式证书，不用重装。

**确实需要 CDN 时**才额外加一个橙云域名（用于 TLS 系节点走 Cloudflare 加速/隐藏源站），
带 `cdn` 前缀即可，面板与订阅仍走上面那一个域名。面板【一键配置】会自动识别并生成 CDN 链接。

> 进阶（一般用不到）：如果面板和订阅系统想整体藏到 Cloudflare 后面，可以另做子域 + Origin 证书，
> 命令是 `x-ui → 18 → 8`（Cloudflare 源站证书，15 年免续签），但要求端口在
> `443/2053/2083/2087/2096/8443` 之内，且不能再走灰云直连。用不上就别碰。

'''
s = s[:start] + simple + s[end:]

io.open(p, 'w', encoding='utf-8', newline='').write(s.replace('\n', nl))
print('DEPLOY.md OK')

import io

p = 'x-ui.sh'
raw = io.open(p, encoding='utf-8', newline='').read()
crlf = '\r\n' in raw
s = raw.replace('\r\n', '\n')

# 1) 在 SSL 子菜单里加第 8 项
old_menu = '''    echo -e "${green}\\t7.${plain} 为面板设置证书路径"
    echo -e "${green}\\t0.${plain} 返回主菜单"'''
new_menu = '''    echo -e "${green}\\t7.${plain} 为面板设置证书路径"
    echo -e "${green}\\t8.${plain} Cloudflare 源站证书（15 年，无需续签）"
    echo -e "${green}\\t0.${plain} 返回主菜单"'''
assert old_menu in s
s = s.replace(old_menu, new_menu, 1)

# 2) 加函数 + 分支
anchor = 'ssl_cert_issue_main() {'
func = '''# Cloudflare 源站证书（Origin CA）入口
ssl_cert_cf_origin() {
    echo ""
    echo -e "${yellow}Cloudflare 源站证书（Origin CA）${plain}"
    echo -e "  ${green}特点${plain}：有效期最长 15 年，不需要续签，Cloudflare 自动信任"
    echo -e "  ${yellow}前提${plain}：1) 域名托管在 Cloudflare 且已开橙云"
    echo -e "        2) 面板端口必须是 443 / 2053 / 2083 / 2087 / 2096 / 8443"
    echo -e "        3) 灰云或直接 IP 访问时，浏览器会提示证书不受信任"
    echo ""
    echo -e "  ${green}1.${plain} 粘贴证书（控制台 SSL/TLS → Origin Server 生成，最稳）"
    echo -e "  ${green}2.${plain} 用 Cloudflare 凭据自动签发"
    echo -e "  ${green}0.${plain} 返回"
    echo ""
    read -rp "请选择 [0-2]: " cf_choice
    case "$cf_choice" in
        1|2)
            read -rp "面板域名（如 panel1.example.com）: " cf_domain
            [ -z "$cf_domain" ] && { LOGE "域名不能为空"; return 1; }
            read -rp "当前面板端口 [54321]: " cf_port
            cf_port=${cf_port:-54321}
            case "$cf_port" in
                443|2053|2083|2087|2096|8443) ;;
                *)
                    echo -e "${yellow}⚠ 端口 ${cf_port} 不在 Cloudflare 回源端口白名单内${plain}"
                    echo -e "${yellow}  开橙云后会连不上。建议先用菜单 9 把面板端口改成 8443，再回来申请。${plain}"
                    read -rp "仍要继续吗？[y/N]: " cf_go
                    [[ "$cf_go" != "y" && "$cf_go" != "Y" ]] && return 0
                    ;;
            esac
            if [ "$cf_choice" == "1" ]; then
                run_repo_script cfcert.sh paste "$cf_domain" --target panel
            else
                run_repo_script cfcert.sh api "$cf_domain" --target panel
            fi
            ;;
        *) return 0 ;;
    esac
}

ssl_cert_issue_main() {'''
assert anchor in s
s = s.replace(anchor, func, 1)

# 3) case 分支里加 8)
old_case = '''    7)
        ssl_cert_set_cert_path
        ;;'''
new_case = '''    7)
        ssl_cert_set_cert_path
        ;;
    8)
        ssl_cert_cf_origin
        ssl_cert_issue_main
        ;;'''
if old_case in s:
    s = s.replace(old_case, new_case, 1)
else:
    # 兜底：插到 0) 分支前
    s = s.replace('''    0)
        show_menu
        ;;
}
ssl_cert_issue_main''', '''    8)
        ssl_cert_cf_origin
        ssl_cert_issue_main
        ;;
    0)
        show_menu
        ;;
}
ssl_cert_issue_main''', 1)

if crlf:
    s = s.replace('\n', '\r\n')
io.open(p, 'w', encoding='utf-8', newline='').write(s)
print('x-ui.sh OK')

import io

p = 'x-ui.sh'
s = io.open(p, encoding='utf-8', newline='').read()
crlf = '\r\n' in s
s = s.replace('\r\n', '\n')

# ---------- 1) 菜单项 ----------
old_menu = '''  ${green}30.${plain} Telegram MTProto 代理（FakeTLS，仅 TG 可用）
——————————————————————'''
new_menu = '''  ${green}30.${plain} Telegram MTProto 代理（FakeTLS，仅 TG 可用）
  ${green}31.${plain} 开放端口（输入端口号即可）
——————————————————————'''
assert old_menu in s
s = s.replace(old_menu, new_menu, 1)

old_prompt = '''    echo && read -p "请输入选项 [0-30]: " num'''
new_prompt = '''    echo && read -p "请输入选项 [0-31]: " num'''
assert old_prompt in s
s = s.replace(old_prompt, new_prompt, 1)

# ---------- 2) 新增 case 31 ----------
old_case = '''    30)
        mtg_menu
        ;;'''
assert old_case in s, '未找到 30 的 case'
new_case = '''    30)
        mtg_menu
        ;;
    31)
        quick_open_port
        ;;'''
s = s.replace(old_case, new_case, 1)

# ---------- 3) 函数本体：插在 firewall_menu 之前 ----------
anchor = '''firewall_menu() {'''
func = '''# 【开源增强】开放端口：只要输入端口号，自动判断 ufw / firewalld 并放行
# 支持：443        单个端口（TCP）
#       443,8443   多个端口
#       51820/udp  指定 UDP
#       400-500    端口段
quick_open_port() {
    echo
    echo -e "${yellow}开放端口${plain}：直接输入端口号回车即可。"
    echo -e "  多个用逗号分隔：${green}443,8443${plain}"
    echo -e "  要开 UDP 加后缀：${green}51820/udp${plain}"
    echo -e "  端口段：${green}40000-40100${plain}"
    echo
    read -rp "请输入端口（留空返回）: " input
    input=$(echo "$input" | tr -d '[:space:]')
    [[ -z "$input" ]] && return 0

    # 校验每一段
    local token proto port range
    IFS=',' read -ra tokens <<< "$input"
    for token in "${tokens[@]}"; do
        if ! [[ "$token" =~ ^([0-9]{1,5})(-([0-9]{1,5}))?(/(tcp|udp))?$ ]]; then
            echo -e "${red}格式不对：$token${plain}（示例：443、443,8443、51820/udp、40000-40100）"
            return 1
        fi
        port=$(echo "$token" | sed -E 's#/.*##; s#-.*##')
        if [[ "$port" -lt 1 || "$port" -gt 65535 ]]; then
            echo -e "${red}端口超出范围：$port（1-65535）${plain}"
            return 1
        fi
    done

    # firewalld（CentOS / Rocky 常见）
    if command -v firewall-cmd &>/dev/null && systemctl is-active --quiet firewalld 2>/dev/null; then
        for token in "${tokens[@]}"; do
            proto="tcp"; [[ "$token" == */udp ]] && proto="udp"
            range=${token%/*}
            firewall-cmd --permanent --add-port="${range}/${proto}" >/dev/null 2>&1 \\
                && echo -e "  ${green}已放行${plain} ${range}/${proto}（firewalld）" \\
                || echo -e "  ${red}放行失败${plain} ${range}/${proto}"
        done
        firewall-cmd --reload >/dev/null 2>&1
    else
        # ufw
        if ! command -v ufw &>/dev/null; then
            echo "  ufw 未安装，正在安装..."
            apt-get update -qq && apt-get install -y ufw >/dev/null 2>&1
        fi
        for token in "${tokens[@]}"; do
            proto="tcp"; [[ "$token" == */udp ]] && proto="udp"
            range=${token%/*}
            if ufw allow "${range}/${proto}" >/dev/null 2>&1; then
                echo -e "  ${green}已放行${plain} ${range}/${proto}（ufw）"
            else
                echo -e "  ${red}放行失败${plain} ${range}/${proto}"
            fi
        done

        if ! ufw status 2>/dev/null | grep -q "Status: active"; then
            echo
            echo -e "${yellow}注意：ufw 当前未启用，上面写的规则暂不生效。${plain}"
            local yn
            read -rp "  现在就启用 ufw？[Y/n]: " yn
            if [[ ! "$yn" =~ ^[Nn] ]]; then
                ufw --force enable >/dev/null 2>&1 && echo "  ufw 已启用"
            else
                echo "  已跳过（以后可用 x-ui → 21 → 4 启用）"
            fi
        fi
    fi

    echo
    echo "当前放行情况："
    if command -v firewall-cmd &>/dev/null && systemctl is-active --quiet firewalld 2>/dev/null; then
        firewall-cmd --list-ports 2>/dev/null | tr ' ' '\\n' | sed 's/^/  /' | head -20
    else
        ufw status 2>/dev/null | sed -n '4,30p' | sed 's/^/  /'
    fi
    echo
}

firewall_menu() {'''
assert anchor in s
s = s.replace(anchor, func, 1)

io.open(p, 'w', encoding='utf-8', newline='').write(s.replace('\n', '\r\n') if crlf else s)
print('x-ui.sh OK')

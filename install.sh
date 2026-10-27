#!/bin/bash

# ============================================================
# 本项目仓库：https://github.com/yosituta/x-panel
# 想装上游原版可覆盖：X_PANEL_REPO=xeefei/x-panel bash install.sh
# ============================================================
X_PANEL_REPO="${X_PANEL_REPO:-yosituta/x-panel}"

# ============================================================
# 无人值守安装（--auto）
#
# 用法示例：
#   bash install.sh --auto --domain panel1.example.com
#   （邮箱不用填：内部自动用 admin@主域 向 Let's Encrypt 注册，仅用于账户标识）
#   bash install.sh --auto --panel-port 23333 --path /xp-abc123 \
#                   --user admin --pass 'MyPwd' --domain panel1.example.com --email me@example.com
#   bash install.sh --auto --no-cert --dry-run     # 只看计划不执行
#
# 不带 --auto 时走原来的交互式安装流程，行为不变。
# ============================================================

AUTO_INSTALL=0
A_PORT=54321
A_PATH="/"
A_USER="admin"
A_PASS="123456"
A_DOMAIN=""
A_EMAIL=""
A_CERT=1
A_DRY=0
A_VERSION=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --auto) AUTO_INSTALL=1; shift ;;
        --panel-port) A_PORT="$2"; shift 2 ;;
        --path) A_PATH="$2"; shift 2 ;;
        --user) A_USER="$2"; shift 2 ;;
        --pass) A_PASS="$2"; shift 2 ;;
        --domain) A_DOMAIN="$2"; shift 2 ;;
        --email) A_EMAIL="$2"; shift 2 ;;  # 兼容旧用法，一般不需要指定
        --version) A_VERSION="$2"; shift 2 ;;
        --no-cert) A_CERT=0; shift ;;
        --dry-run) A_DRY=1; shift ;;
        -h|--help)
            sed -n '/^# 无人值守安装/,/^# ====/p' "$0" 2>/dev/null | head -20
            exit 0
            ;;
        *) shift ;;
    esac
done

auto_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo amd64 ;;
        aarch64|arm64) echo arm64 ;;
        *) echo "" ;;
    esac
}

auto_rand() {
    tr -dc 'a-z0-9' </dev/urandom 2>/dev/null | head -c "${1:-8}"
}

auto_install() {
    local green='\033[0;32m' yellow='\033[0;33m' red='\033[0;31m' plain='\033[0m'


    local arch
    arch="$(auto_arch)"
    if [[ -z "$arch" ]]; then
        echo -e "${red}不支持的架构：$(uname -m)${plain}"; return 1
    fi

    # 默认不加密路径；传 --path /xxx 才会设置
    if [[ -z "$A_PATH" || "$A_PATH" == "/" ]]; then
        A_PATH="/"
    else
        A_PATH="/${A_PATH#/}"
    fi
    [[ -z "$A_PASS" ]] && A_PASS="123456"

    if [[ -z "$A_VERSION" ]]; then
        A_VERSION=$(curl -fsSL "https://api.github.com/repos/${X_PANEL_REPO}/releases/latest" 2>/dev/null \
            | grep '"tag_name":' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
    fi
    A_VERSION="${A_VERSION#v}"
    if [[ -z "$A_VERSION" ]]; then
        echo -e "${red}无法获取最新版本号，可用 --version v1.1.1 手动指定${plain}"; return 1
    fi

    local use_cert=0
    if [[ "$A_CERT" == "1" && -n "$A_DOMAIN" ]]; then
        use_cert=1
        if [[ -z "$A_EMAIL" ]]; then
            A_EMAIL="admin@${A_DOMAIN#*.}"
        fi
    fi

    echo -e "${green}==== 安装计划 ====${plain}"
    echo "  仓库      : ${X_PANEL_REPO}"
    echo "  版本      : v${A_VERSION}"
    echo "  架构      : ${arch}"
    echo "  面板端口  : ${A_PORT}"
    echo "  访问路径  : ${A_PATH}"
    echo "  账号      : ${A_USER}"
    echo "  密码      : ${A_PASS}"
    echo "  Xray 内核 : 随包固定版本（勿随意升级，见 CHANGELOG）"
    echo "  管理菜单  : /usr/bin/x-ui（输入 x-ui 进入）"
    if [[ "$use_cert" == "1" ]]; then
        echo "  HTTPS     : 是，域名 ${A_DOMAIN}（注册邮箱 ${A_EMAIL}，剩余 15 天自动续期）"
    else
        echo "  HTTPS     : 否（不申请证书）"
    fi

    if [[ "$A_DRY" == "1" ]]; then
        echo -e "${yellow}--dry-run：未做任何修改${plain}"
        return 0
    fi

    if [[ $EUID -ne 0 ]]; then
        echo -e "${red}请用 root 运行${plain}"; return 1
    fi

    echo -e "${green}---- 1/6 安装依赖 ----${plain}"
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq >/dev/null 2>&1 || true
    apt-get install -y -qq curl wget tar socat cron ca-certificates >/dev/null 2>&1 || true

    echo -e "${green}---- 2/6 准备 acme.sh ----${plain}"
    systemctl enable --now cron >/dev/null 2>&1 || true
    if [[ ! -x /root/.acme.sh/acme.sh ]]; then
        curl -s https://get.acme.sh | sh -s email="${A_EMAIL:-noreply@example.com}" >/dev/null 2>&1 \
            && echo "  acme.sh 已安装" || echo "  acme.sh 安装失败（可用 x-ui 菜单 18 重试）"
    else
        echo "  acme.sh 已存在"
    fi

    echo -e "${green}---- 3/6 下载安装包 ----${plain}"
    local url="https://github.com/${X_PANEL_REPO}/releases/download/v${A_VERSION}/x-ui-linux-${arch}.tar.gz"
    echo "  ${url}"
    rm -rf /tmp/xp-install && mkdir -p /tmp/xp-install && cd /tmp/xp-install || return 1
    if ! curl -fL --retry 3 --connect-timeout 20 -o pkg.tar.gz "$url"; then
        echo -e "${red}下载失败，检查网络或版本号${plain}"; return 1
    fi
    tar -zxf pkg.tar.gz || { echo -e "${red}解压失败${plain}"; return 1; }

    echo -e "${green}---- 4/6 安装到 /usr/local/x-ui ----${plain}"
    systemctl stop x-ui >/dev/null 2>&1 || true
    rm -rf /usr/local/x-ui
    mv x-ui /usr/local/x-ui || return 1
    cd /usr/local/x-ui || return 1
    chmod +x x-ui x-ui.sh
    [[ -f "bin/xray-linux-${arch}" ]] && chmod +x "bin/xray-linux-${arch}"
    cp -f x-ui.service /etc/systemd/system/x-ui.service
    systemctl daemon-reload
    systemctl enable x-ui >/dev/null 2>&1 || true

    echo -e "${green}---- 5/6 写入面板设置 ----${plain}"
    if ! ./x-ui setting -port "$A_PORT" -webBasePath "$A_PATH" \
            -username "$A_USER" -password "$A_PASS" >/tmp/xp-setting.log 2>&1; then
        echo -e "${red}写入设置失败：${plain}"; tail -5 /tmp/xp-setting.log; return 1
    fi
    echo "  端口 ${A_PORT} / 路径 ${A_PATH} 已写入"

    # 管理菜单：与 3x-ui 一致，输入 x-ui 即可进入
    if [[ -f /usr/local/x-ui/x-ui.sh ]]; then
        cp -f /usr/local/x-ui/x-ui.sh /usr/bin/x-ui
        chmod +x /usr/bin/x-ui
        echo "  管理菜单已安装：输入 x-ui 进入"
    fi

    local cert_done=0
    if [[ "$use_cert" == "1" ]]; then
        echo -e "${green}---- 6/6 申请证书（acme.sh standalone，需 80 端口空闲）----${plain}"
        /root/.acme.sh/acme.sh --set-default-ca --server letsencrypt >/dev/null 2>&1 || true
        # --days 15：证书剩余 15 天时自动续期；acme.sh 自带 cron 任务
        if /root/.acme.sh/acme.sh --issue -d "$A_DOMAIN" --standalone -k ec-256 --days 15 --force >/tmp/xp-acme.log 2>&1; then
            mkdir -p "/root/cert/${A_DOMAIN}"
            /root/.acme.sh/acme.sh --install-cert -d "$A_DOMAIN" --ecc \
                --key-file "/root/cert/${A_DOMAIN}/privkey.pem" \
                --fullchain-file "/root/cert/${A_DOMAIN}/fullchain.pem" \
                --reloadcmd "x-ui restart" >/dev/null 2>&1
            if [[ -f "/root/cert/${A_DOMAIN}/fullchain.pem" ]]; then
                ./x-ui setting -webCert "/root/cert/${A_DOMAIN}/fullchain.pem" \
                               -webCertKey "/root/cert/${A_DOMAIN}/privkey.pem" >/dev/null 2>&1
                cert_done=1
                echo "  证书已签发并绑定：/root/cert/${A_DOMAIN}/"
            fi
        else
            echo -e "${yellow}  证书申请失败，日志：/tmp/xp-acme.log${plain}"
            echo -e "${yellow}  常见原因：域名未解析到本机 / 80 端口被占用或被墙${plain}"
            tail -6 /tmp/xp-acme.log 2>/dev/null | sed 's/^/    /'
        fi
    else
        echo -e "${green}---- 6/6 跳过证书 ----${plain}"
    fi

    # 防火墙
    local fw_done=0
    if command -v ufw >/dev/null 2>&1; then
        ufw allow "$A_PORT"/tcp >/dev/null 2>&1 || true
        if [[ "$use_cert" == "1" ]]; then ufw allow 80/tcp >/dev/null 2>&1 || true; fi
        ufw allow 22/tcp >/dev/null 2>&1 || true
        ufw allow 443/tcp >/dev/null 2>&1 || true
        # 节点端口不再整段放行：面板创建/修改入站时会自动 ufw allow 具体端口
        # 未启用则直接启用（用户要求安装时自动打开防火墙）
        if ! ufw status 2>/dev/null | grep -q "Status: active"; then
            echo "y" | ufw enable >/dev/null 2>&1 || true
        fi
        fw_done=1
        echo "  ufw: $(ufw status 2>/dev/null | head -1)（已放行 ${A_PORT}/tcp, 22/tcp$([[ "$use_cert" == "1" ]] && echo ", 80/tcp")）"
    fi
    if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
        firewall-cmd --add-port="$A_PORT"/tcp --permanent >/dev/null 2>&1 || true
        [[ "$use_cert" == "1" ]] && firewall-cmd --add-port=80/tcp --permanent >/dev/null 2>&1 || true
        firewall-cmd --add-port=443/tcp --permanent >/dev/null 2>&1 || true
        # 节点端口由面板按需放行
        firewall-cmd --reload >/dev/null 2>&1 || true
        fw_done=1
        echo "  firewalld: 已放行 ${A_PORT}/tcp"
    fi
    if [[ "$fw_done" == "0" ]]; then
        echo "  未检测到 ufw / firewalld，若使用云厂商安全组请自行放行 ${A_PORT}/tcp"
    fi

    systemctl restart x-ui
    sleep 6
    local active
    active=$(systemctl is-active x-ui)

    local scheme="http"
    [[ "$cert_done" == "1" ]] && scheme="https"
    local host="${A_DOMAIN:-$(curl -s --max-time 5 https://api.ipify.org 2>/dev/null)}"

    echo ""
    echo -e "${green}========================================${plain}"
    echo -e "${green} X-Panel 安装完成${plain}"
    echo -e "${green}========================================${plain}"
    echo "  服务状态 : ${active}"
    echo "  面板版本 : $(./x-ui -v 2>/dev/null)"
    echo "  Xray 内核: $(./bin/xray-linux-${arch} version 2>/dev/null | head -1)"
    echo ""
    echo -e "  登录地址 : ${yellow}${scheme}://${host}:${A_PORT}${A_PATH}/${plain}"
    echo "  用户名   : ${A_USER}"
    echo "  密码     : ${A_PASS}"
    echo ""
    if [[ "$cert_done" != "1" ]]; then
        echo -e "${yellow}未启用 HTTPS。要启用：${plain}"
        echo "  1) 先把域名 A 记录指到本机（建议保持灰云，不要走 CDN）"
        echo "  2) 输入 x-ui → 18 (SSL 证书管理) → 1 (获取证书)"
        echo "  3) 完成后用 https://域名:${A_PORT}${A_PATH}/ 访问"
        echo ""
    fi
    echo -e "${yellow}下一步（多台服务器请按此命名规范）：${plain}"
    echo "  面板域名 : panelN.example.com  （灰云，直接指向本机 IP）"
    echo "  节点域名 : cdnN.example.com    （橙云，Cloudflare 代理）"
    echo "  在面板【一键配置】里选预设，宿主域名与节点域名同号时，CDN 链接会自动带出。"
    echo ""
    if [[ "$A_PASS" == "123456" ]]; then
        echo -e "${red}  注意：当前密码为默认值 123456，登录后请立即修改${plain}"
        echo "        修改：x-ui → 6 (重置用户名、密码)"
        echo ""
    fi
    echo "  管理菜单 : 输入 x-ui（二十多项菜单：证书、防火墙、BBR、订阅、快照等）"
    echo "  日志     : journalctl -u x-ui -f"
    return 0
}

if [[ "$AUTO_INSTALL" == "1" ]]; then
    auto_install
    exit $?
fi



# ==========================================================
# X-Panel 开源增强版 安装脚本
# 基于 xeefei/X-Panel (GPLv3) 修改
# ==========================================================

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

# check root
[[ $EUID -ne 0 ]] && echo -e "${red}致命错误: ${plain} 请使用 root 权限运行此脚本\n" && exit 1

# ----------------------------------------------------------
# 获取机器唯一硬件标识 (HWID)
# ----------------------------------------------------------
get_hwid() {
    local machine_id=""

    # 1. 优先尝试获取 DMI Product UUID (VPS 硬件 ID，重装系统通常不变)
    if [[ -r /sys/class/dmi/id/product_uuid ]]; then
        machine_id=$(cat /sys/class/dmi/id/product_uuid)
    
    # 2. 其次尝试获取 eth0 网卡 MAC 地址 (大部分 VPS 重装后 MAC 不变)
    elif [[ -r /sys/class/net/eth0/address ]]; then
        machine_id=$(cat /sys/class/net/eth0/address)
        
    # 3. 如果都失败，才使用 machine-id (重装会变，作为最后兜底)
    elif [[ -f /etc/machine-id ]]; then
        machine_id=$(cat /etc/machine-id)
    else
        machine_id=$(hostname)
    fi
    
    # 取 MD5 作为唯一指纹，确保格式统一
    echo -n "$machine_id" | md5sum | awk '{print $1}'
}

# ----------------------------------------------------------
# 函数：付费授权版安装入口（本分支已停用）
# ----------------------------------------------------------
# 说明：本分支已移除“远程授权服务器下发脚本”的逻辑，不再依赖任何第三方授权服务
#
install_paid_version() {
    echo ""
    echo -e "${red}本分支不提供“付费授权版”安装流程。${plain}"
    echo -e "${green}本仓库的所有增强功能已内置在开源版中，直接选择 1) 安装即可。${plain}"
    echo ""
    exit 1
}


# ----------------------------------------------------------
# 函数：免费基础版安装逻辑 (install_free_version) 
# ----------------------------------------------------------
install_free_version() {
    echo ""
    echo -e "${green}您选择了安装 【X-Panel 免费基础版】${plain}"
    echo ""
    echo -e "${green}即将开始执行标准安装流程...${plain}"
    sleep 2

    cur_dir=$(pwd)

    # Check OS and set release variable
    if [[ -f /etc/os-release ]]; then
        source /etc/os-release
        release=$ID
    elif [[ -f /usr/lib/os-release ]]; then
        source /usr/lib/os-release
        release=$ID
    else
        echo ""
        echo -e "${red}检查服务器操作系统失败，请联系作者!${plain}" >&2
        exit 1
    fi
    echo ""
    echo -e "${green}---------->>>>>目前服务器的操作系统为: $release${plain}"

    arch() {
        case "$(uname -m)" in
            x86_64 | x64 | amd64 ) echo 'amd64' ;;
            i*86 | x86 ) echo '386' ;;
            armv8* | armv8 | arm64 | aarch64 ) echo 'arm64' ;;
            armv7* | armv7 | arm ) echo 'armv7' ;;
            armv6* | armv6 ) echo 'armv6' ;;
            armv5* | armv5 ) echo 'armv5' ;;
            s390x) echo 's390x' ;;
            *) echo -e "${green}不支持的CPU架构! ${plain}" && rm -f install.sh && exit 1 ;;
        esac
    }

    echo ""
    # check_glibc_version() {
    #    glibc_version=$(ldd --version | head -n1 | awk '{print $NF}')

    #    required_version="2.32"
    #    if [[ "$(printf '%s\n' "$required_version" "$glibc_version" | sort -V | head -n1)" != "$required_version" ]]; then
    #        echo -e "${red}------>>>GLIBC版本 $glibc_version 太旧了！ 要求2.32或以上版本${plain}"
    #        echo -e "${green}-------->>>>请升级到较新版本的操作系统以便获取更高版本的GLIBC${plain}"
    #        exit 1
    #    fi
    #        echo -e "${green}-------->>>>GLIBC版本： $glibc_version（符合高于2.32的要求）${plain}"
    # }
    # check_glibc_version

    # echo ""
    echo -e "${yellow}---------->>>>>当前系统的架构为: $(arch)${plain}"
    echo ""
    last_version=$(curl -Ls "https://api.github.com/repos/${X_PANEL_REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    # 获取 x-ui 版本
    xui_version=$(/usr/local/x-ui/x-ui -v)

    # 检查 xui_version 是否为空
    if [[ -z "$xui_version" ]]; then
        echo ""
        echo -e "${red}------>>>当前服务器没有安装任何 x-ui 系列代理面板${plain}"
        echo ""
        echo -e "${green}-------->>>>片刻之后脚本将会自动引导安装〔X-Panel面板〕${plain}"
    else
        # 检查版本号中是否包含冒号
        if [[ "$xui_version" == *:* ]]; then
            echo -e "${green}---------->>>>>当前代理面板的版本为: ${red}其他 x-ui 分支版本${plain}"
            echo ""
            echo -e "${green}-------->>>>片刻之后脚本将会自动引导安装〔X-Panel面板〕${plain}"
        else
            echo -e "${green}---------->>>>>当前代理面板的版本为: ${red}〔X-Panel面板〕v${xui_version}${plain}"
        fi
    fi
    echo ""
    echo -e "${yellow}---------------------->>>>>〔X-Panel面板〕最新版为：${last_version}${plain}"
    sleep 4

    os_version=$(grep -i version_id /etc/os-release | cut -d \" -f2 | cut -d . -f1)

    if [[ "${release}" == "arch" ]]; then
        echo "您的操作系统是 ArchLinux"
    elif [[ "${release}" == "manjaro" ]]; then
        echo "您的操作系统是 Manjaro"
    elif [[ "${release}" == "armbian" ]]; then
        echo "您的操作系统是 Armbian"
    elif [[ "${release}" == "alpine" ]]; then
        echo "您的操作系统是 Alpine Linux"
    elif [[ "${release}" == "opensuse-tumbleweed" ]]; then
        echo "您的操作系统是 OpenSUSE Tumbleweed"
    elif [[ "${release}" == "centos" ]]; then
        if [[ ${os_version} -lt 8 ]]; then
            echo -e "${red} 请使用 CentOS 8 或更高版本 ${plain}\n" && exit 1
        fi
    elif [[ "${release}" == "ubuntu" ]]; then
        if [[ ${os_version} -lt 20 ]]; then
            echo -e "${red} 请使用 Ubuntu 20 或更高版本!${plain}\n" && exit 1
        fi
    elif [[ "${release}" == "fedora" ]]; then
        if [[ ${os_version} -lt 36 ]]; then
            echo -e "${red} 请使用 Fedora 36 或更高版本!${plain}\n" && exit 1
        fi
    elif [[ "${release}" == "debian" ]]; then
        if [[ ${os_version} -lt 11 ]]; then
            echo -e "${red} 请使用 Debian 11 或更高版本 ${plain}\n" && exit 1
        fi
    elif [[ "${release}" == "almalinux" ]]; then
        if [[ ${os_version} -lt 9 ]]; then
            echo -e "${red} 请使用 AlmaLinux 9 或更高版本 ${plain}\n" && exit 1
        fi
    elif [[ "${release}" == "rocky" ]]; then
        if [[ ${os_version} -lt 9 ]]; then
            echo -e "${red} 请使用 RockyLinux 9 或更高版本 ${plain}\n" && exit 1
        fi
    elif [[ "${release}" == "oracle" ]]; then
        if [[ ${os_version} -lt 8 ]]; then
            echo -e "${red} 请使用 Oracle Linux 8 或更高版本 ${plain}\n" && exit 1
        fi
    else
        echo -e "${red}此脚本不支持您的操作系统。${plain}\n"
        echo "请确保您使用的是以下受支持的操作系统之一："
        echo "- Ubuntu 20.04+"
        echo "- Debian 11+"
        echo "- CentOS 8+"
        echo "- Fedora 36+"
        echo "- Arch Linux"
        echo "- Manjaro"
        echo "- Armbian"
        echo "- Alpine Linux"
        echo "- AlmaLinux 9+"
        echo "- Rocky Linux 9+"
        echo "- Oracle Linux 8+"
        echo "- OpenSUSE Tumbleweed"
        exit 1

    fi

    install_base() {
        case "${release}" in
        ubuntu | debian | armbian)
            apt-get update && apt-get install -y -q wget curl sudo tar tzdata
            ;;
        centos | rhel | almalinux | rocky | ol)
            yum -y --exclude=kernel* update && yum install -y -q wget curl sudo tar tzdata
            ;;
        fedora | amzn | virtuozzo)
            dnf -y --exclude=kernel* update && dnf install -y -q wget curl sudo tar tzdata
            ;;
        arch | manjaro | parch)
            pacman -Sy && pacman -S --noconfirm wget curl sudo tar tzdata
            ;;
        alpine)
            apk update && apk add --no-cache wget curl sudo tar tzdata
            ;;
        opensuse-tumbleweed)
            zypper refresh && zypper -q install -y wget curl sudo tar timezone
            ;;
        *)
            apt-get update && apt-get install -y -q wget curl sudo tar tzdata
            ;;
        esac
    }

    gen_random_string() {
        local length="$1"
        local random_string=$(LC_ALL=C tr -dc 'a-zA-Z0-9' </dev/urandom | fold -w "$length" | head -n 1)
        echo "$random_string"
    }

    # 交互安装同样自动启用防火墙并放行面板端口（用户要求：装完即可访问）
    xp_open_firewall() {
        local port
        port=$(/usr/local/x-ui/x-ui setting -show true 2>/dev/null | grep -Eo 'port（端口号）: [0-9]+' | awk '{print $2}')
        port="${port:-54321}"
        if command -v ufw >/dev/null 2>&1; then
            ufw allow "${port}"/tcp >/dev/null 2>&1 || true
            ufw allow 22/tcp >/dev/null 2>&1 || true
            ufw allow 80/tcp >/dev/null 2>&1 || true
            ufw allow 443/tcp >/dev/null 2>&1 || true
            ufw status 2>/dev/null | grep -q "Status: active" || echo "y" | ufw enable >/dev/null 2>&1 || true
            echo -e "${green}防火墙：已启用并放行 ${port}/tcp、22、80、443（节点端口由面板按需放行）${plain}"
        elif command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
            local p
            for p in "${port}" 22 80 443; do firewall-cmd --add-port="${p}"/tcp --permanent >/dev/null 2>&1 || true; done
            firewall-cmd --reload >/dev/null 2>&1 || true
            echo -e "${green}防火墙：已放行 ${port}/tcp、22、80、443${plain}"
        else
            echo -e "${yellow}未检测到 ufw / firewalld，若用云厂商安全组请自行放行 ${port}/tcp${plain}"
        fi
    }

    # This function will be called when user installed x-ui out of security
    config_after_install() {
        echo -e "${yellow}安装/更新完成！ 为了您的面板安全，建议修改面板设置 ${plain}"
        echo ""
        read -p "$(echo -e "${green}想继续修改吗？${red}选择“n”以保留旧设置${plain} [y/n]？--->>请输入：")" config_confirm
        if [[ "${config_confirm}" == "y" || "${config_confirm}" == "Y" ]]; then
            # 直接回车即用默认值，不必手输
            read -p "请设置您的用户名 [admin]: " config_account
            config_account="${config_account:-admin}"
            echo -e "${yellow}您的用户名将是: ${config_account}${plain}"
            read -p "请设置您的密码 [123456]: " config_password
            config_password="${config_password:-123456}"
            echo -e "${yellow}您的密码将是: ${config_password}${plain}"
            read -p "请设置面板端口 [54321]: " config_port
            config_port="${config_port:-54321}"
            echo -e "${yellow}您的面板端口号为: ${config_port}${plain}"
            read -p "请设置面板登录访问路径 [/]: " config_webBasePath
            config_webBasePath="${config_webBasePath:-/}"
            echo -e "${yellow}您的面板访问路径为: ${config_webBasePath}${plain}"
            echo -e "${yellow}正在初始化，请稍候...${plain}"
            /usr/local/x-ui/x-ui setting -username ${config_account} -password ${config_password}
            echo -e "${yellow}用户名和密码设置成功!${plain}"
            /usr/local/x-ui/x-ui setting -port ${config_port}
            echo -e "${yellow}面板端口号设置成功!${plain}"
            /usr/local/x-ui/x-ui setting -webBasePath ${config_webBasePath}
            echo -e "${yellow}面板登录访问路径设置成功!${plain}"
            echo ""
        else
            echo ""
            sleep 1
            echo -e "${red}--------------->>>>Cancel...--------------->>>>>>>取消修改...${plain}"
            echo ""
            if [[ ! -f "/etc/x-ui/x-ui.db" ]]; then
                local usernameTemp=$(head -c 10 /dev/urandom | base64)
                local passwordTemp=$(head -c 10 /dev/urandom | base64)
                local webBasePathTemp=$(gen_random_string 15)
                /usr/local/x-ui/x-ui setting -username ${usernameTemp} -password ${passwordTemp} -webBasePath ${webBasePathTemp}
                echo ""
                echo -e "${yellow}检测到为全新安装，出于安全考虑将生成随机登录信息:${plain}"
                echo -e "###############################################"
                echo -e "${green}用户名: ${usernameTemp}${plain}"
                echo -e "${green}密  码: ${passwordTemp}${plain}"
                echo -e "${green}访问路径: ${webBasePathTemp}${plain}"
                echo -e "###############################################"
                echo -e "${green}如果您忘记了登录信息，可以在安装后通过 x-ui 命令然后输入${red}数字 10 选项${green}进行查看${plain}"
            else
                echo -e "${green}此次操作属于版本升级，保留之前旧设置项，登录方式保持不变${plain}"
                echo ""
                echo -e "${green}如果您忘记了登录信息，您可以通过 x-ui 命令然后输入${red}数字 10 选项${green}进行查看${plain}"
                echo ""
                echo ""
            fi
        fi
        sleep 1
        echo -e ">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>"
        echo ""
        /usr/local/x-ui/x-ui migrate
    }

    echo ""
    install_x-ui() {
        cd /usr/local/

        # Download resources
        if [ $# == 0 ]; then
            last_version=$(curl -Ls "https://api.github.com/repos/${X_PANEL_REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
            if [[ ! -n "$last_version" ]]; then
                echo -e "${red}获取 X-Panel 版本失败，可能是 Github API 限制，请稍后再试${plain}"
                exit 1
            fi
            echo ""
            echo -e "-----------------------------------------------------"
            echo -e "${green}--------->>获取 X-Panel 最新版本：${yellow}${last_version}${plain}${green}，开始安装...${plain}"
            echo -e "-----------------------------------------------------"
            echo ""
            sleep 2
            echo -e "${green}---------------->>>>>>>>>安装进度50%${plain}"
            sleep 3
            echo ""
            echo -e "${green}---------------->>>>>>>>>>>>>>>>>>>>>安装进度100%${plain}"
            echo ""
            sleep 2
            wget -N --no-check-certificate -O /usr/local/x-ui-linux-$(arch).tar.gz https://github.com/${X_PANEL_REPO}/releases/download/${last_version}/x-ui-linux-$(arch).tar.gz
            if [[ $? -ne 0 ]]; then
                echo -e "${red}下载 X-Panel 失败, 请检查服务器是否可以连接至 GitHub？ ${plain}"
                exit 1
            fi
        else
            last_version=$1
            url="https://github.com/${X_PANEL_REPO}/releases/download/${last_version}/x-ui-linux-$(arch).tar.gz"
            echo ""
            echo -e "--------------------------------------------"
            echo -e "${green}---------------->>>>开始安装 X-Panel 免费基础版$1${plain}"
            echo -e "--------------------------------------------"
            echo ""
            sleep 2
            echo -e "${green}---------------->>>>>>>>>安装进度50%${plain}"
            sleep 3
            echo ""
            echo -e "${green}---------------->>>>>>>>>>>>>>>>>>>>>安装进度100%${plain}"
            echo ""
            sleep 2
            wget -N --no-check-certificate -O /usr/local/x-ui-linux-$(arch).tar.gz ${url}
            if [[ $? -ne 0 ]]; then
                echo -e "${red}下载 X-Panel $1 失败, 请检查此版本是否存在 ${plain}"
                exit 1
            fi
        fi
        wget -O /usr/bin/x-ui-temp https://raw.githubusercontent.com/${X_PANEL_REPO}/main/x-ui.sh

        # Stop x-ui service and remove old resources
        if [[ -e /usr/local/x-ui/ ]]; then
            systemctl stop x-ui
            rm /usr/local/x-ui/ -rf
        fi
        
        sleep 3
        echo -e "${green}------->>>>>>>>>>>检查并保存安装目录${plain}"
        echo ""
        tar zxvf x-ui-linux-$(arch).tar.gz
        rm x-ui-linux-$(arch).tar.gz -f
        
        cd x-ui
        chmod +x x-ui
        chmod +x x-ui.sh

        # Check the system's architecture and rename the file accordingly
        if [[ $(arch) == "armv5" || $(arch) == "armv6" || $(arch) == "armv7" ]]; then
            mv bin/xray-linux-$(arch) bin/xray-linux-arm
            chmod +x bin/xray-linux-arm
        fi
        chmod +x x-ui bin/xray-linux-$(arch)

        # Update x-ui cli and se set permission
        mv -f /usr/bin/x-ui-temp /usr/bin/x-ui
        chmod +x /usr/bin/x-ui
        sleep 2
        echo -e "${green}------->>>>>>>>>>>保存成功${plain}"
        sleep 2
        echo ""
        config_after_install
        xp_open_firewall

    ssh_forwarding() {
        # 获取 IPv4 和 IPv6 地址
        v4=$(curl -s4m8 http://ip.sb -k)
        v6=$(curl -s6m8 http://ip.sb -k)
        local existing_webBasePath=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'webBasePath（访问路径）: .+' | awk '{print $2}') 
        local existing_port=$(/usr/local/x-ui/x-ui setting -show true | grep -Eo 'port（端口号）: .+' | awk '{print $2}') 
        local existing_cert=$(/usr/local/x-ui/x-ui setting -getCert true | grep -Eo 'cert: .+' | awk '{print $2}')
        local existing_key=$(/usr/local/x-ui/x-ui setting -getCert true | grep -Eo 'key: .+' | awk '{print $2}')

        if [[ -n "$existing_cert" && -n "$existing_key" ]]; then
            echo -e "${green}面板已安装证书采用SSL保护${plain}"
            echo ""
            local existing_cert=$(/usr/local/x-ui/x-ui setting -getCert true | grep -Eo 'cert: .+' | awk '{print $2}')
            domain=$(basename "$(dirname "$existing_cert")")
            echo -e "${green}登录访问面板URL: https://${domain}:${existing_port}${green}${existing_webBasePath}${plain}"
        fi
        echo ""
        if [[ -z "$existing_cert" && -z "$existing_key" ]]; then
            echo -e "${red}警告：未找到证书和密钥，面板不安全！${plain}"
            echo ""
            echo -e "${green}------->>>>请按照下述方法设置〔ssh转发〕<<<<-------${plain}"
            echo ""

            # 检查 IP 并输出相应的 SSH 和浏览器访问信息
            if [[ -z $v4 ]]; then
                echo -e "${green}1、本地电脑客户端转发命令：${plain} ${blue}ssh  -L [::]:15208:127.0.0.1:${existing_port}${blue} root@[$v6]${plain}"
                echo ""
                echo -e "${green}2、请通过快捷键【Win + R】调出运行窗口，在里面输入【cmd】打开本地终端服务${plain}"
                echo ""
                echo -e "${green}3、请在终端中成功输入服务器的〔root密码〕，注意区分大小写，用以上命令进行转发${plain}"
                echo ""
                echo -e "${green}4、请在浏览器地址栏复制${plain} ${blue}[::1]:15208${existing_webBasePath}${plain} ${green}进入〔X-Panel面板〕登录界面"
                echo ""
                echo -e "${red}注意：若不使用〔ssh转发〕请为X-Panel面板配置安装证书再行登录管理后台${plain}"
            elif [[ -n $v4 && -n $v6 ]]; then
                echo -e "${green}1、本地电脑客户端转发命令：${plain} ${blue}ssh -L 15208:127.0.0.1:${existing_port}${blue} root@$v4${plain} ${yellow}或者 ${blue}ssh  -L [::]:15208:127.0.0.1:${existing_port}${blue} root@[$v6]${plain}"
                echo ""
                echo -e "${green}2、请通过快捷键【Win + R】调出运行窗口，在里面输入【cmd】打开本地终端服务${plain}"
                echo ""
                echo -e "${green}3、请在终端中成功输入服务器的〔root密码〕，注意区分大小写，用以上命令进行转发${plain}"
                echo ""
                echo -e "${green}4、请在浏览器地址栏复制${plain} ${blue}127.0.0.1:15208${existing_webBasePath}${plain} ${yellow}或者${plain} ${blue}[::1]:15208${existing_webBasePath}${plain} ${green}进入〔X-Panel面板〕登录界面"
                echo ""
                echo -e "${red}注意：若不使用〔ssh转发〕请为X-Panel面板配置安装证书再行登录管理后台${plain}"
            else
                echo -e "${green}1、本地电脑客户端转发命令：${plain} ${blue}ssh -L 15208:127.0.0.1:${existing_port}${blue} root@$v4${plain}"
                echo ""
                echo -e "${green}2、请通过快捷键【Win + R】调出运行窗口，在里面输入【cmd】打开本地终端服务${plain}"
                echo ""
                echo -e "${green}3、请在终端中成功输入服务器的〔root密码〕，注意区分大小写，用以上命令进行转发${plain}"
                echo ""
                echo -e "${green}4、请在浏览器地址栏复制${plain} ${blue}127.0.0.1:15208${existing_webBasePath}${plain} ${green}进入〔X-Panel面板〕登录界面"
                echo ""
                echo -e "${red}注意：若不使用〔ssh转发〕请为X-Panel面板配置安装证书再行登录管理后台${plain}"
                echo ""
            fi
        fi
    }
    # 执行ssh端口转发
    ssh_forwarding

        cp -f x-ui.service /etc/systemd/system/
        systemctl daemon-reload
        systemctl enable x-ui
        systemctl start x-ui
        systemctl stop warp-go >/dev/null 2>&1
        wg-quick down wgcf >/dev/null 2>&1
        ipv4=$(curl -s4m8 ip.p3terx.com -k | sed -n 1p)
        ipv6=$(curl -s6m8 ip.p3terx.com -k | sed -n 1p)
        systemctl start warp-go >/dev/null 2>&1
        wg-quick up wgcf >/dev/null 2A>&1

        echo ""
        echo -e "------->>>>${green}X-Panel 免费基础版 ${last_version}${plain}<<<<安装成功，正在启动..."
        sleep 1
        echo ""
        echo -e "         ---------------------"
        echo -e "         |${green}X-Panel 控制菜单用法 ${plain}|${plain}"
        echo -e "         |  ${yellow}一个更好的面板   ${plain}|${plain}"   
        echo -e "         | ${yellow}基于Xray Core构建 ${plain}|${plain}"  
        echo -e "--------------------------------------------"
        echo -e "x-ui              - 进入管理脚本"
        echo -e "x-ui start        - 启动 X-Panel 面板"
        echo -e "x-ui stop         - 关闭 X-Panel 面板"
        echo -e "x-ui restart      - 重启 X-Panel 面板"
        echo -e "x-ui status       - 查看 X-Panel 状态"
        echo -e "x-ui settings     - 查看当前设置信息"
        echo -e "x-ui enable       - 启用 X-Panel 开机启动"
        echo -e "x-ui disable      - 禁用 X-Panel 开机启动"
        echo -e "x-ui log          - 查看 X-Panel 运行日志"
        echo -e "x-ui banlog       - 检查 Fail2ban 禁止日志"
        echo -e "x-ui update       - 更新 X-Panel 面板"
        echo -e "x-ui custom       - 自定义 X-Panel 版本"
        echo -e "x-ui install      - 安装 X-Panel 面板"
        echo -e "x-ui uninstall    - 卸载 X-Panel 面板"
        echo -e "--------------------------------------------"
        echo ""
        # if [[ -n $ipv4 ]]; then
        #    echo -e "${yellow}面板 IPv4 访问地址为：${green}http://$ipv4:${config_port}/${config_webBasePath}${plain}"
        # fi
        # if [[ -n $ipv6 ]]; then
        #    echo -e "${yellow}面板 IPv6 访问地址为：${green}http://[$ipv6]:${config_port}/${config_webBasePath}${plain}"
        # fi
        #    echo -e "请自行确保此端口没有被其他程序占用，${yellow}并且确保${red} ${config_port} ${yellow}端口已放行${plain}"
        sleep 3
        echo -e ">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>"
        echo ""
        echo -e "${yellow}----->>>X-Panel面板和Xray启动成功<<<-----${plain}"
    }

    # 设置VPS中的时区/时间为【上海时间】
    sudo timedatectl set-timezone Asia/Shanghai

    install_base
    install_x-ui $1
    echo ""
    echo -e "----------------------------------------------"
    sleep 4
    info=$(/usr/local/x-ui/x-ui setting -show true)
    echo -e "${info}${plain}"
    echo ""
    echo -e "若您忘记了上述面板信息，后期可通过x-ui命令进入脚本${red}输入数字〔10〕选项获取${plain}"
    echo ""
    echo -e "----------------------------------------------"
    echo ""
    sleep 2
    echo -e "${green}安装/更新完成${plain}"
    echo ""
    echo -e "${green}项目地址：${yellow}https://github.com/yosituta/x-panel${plain}"
    echo ""
    echo -e "${yellow}输入 x-ui 打开管理菜单：证书、数据快照、深度调优、网页 SSH、线路检测等${plain}"
    echo ""
    echo ""
    echo -e "----------------------------------------------"
    echo ""
}

# 免费版安装逻辑函数 (install_free_version) 结束

# ----------------------------------------------------------
# 脚本主菜单
# ----------------------------------------------------------
main_menu() {
    echo -e "${green}======================================================${plain}"
    echo -e " 欢迎使用 ${yellow}〔X-Panel 面板〕${plain} 一键安装脚本"
    echo -e "${green}======================================================${plain}"
    echo ""
    echo -e "请选择您要安装的版本:"
    echo ""
    echo -e "  ${green}1)${plain} 安装 ${yellow}〔X-Panel 面板〕开源版${plain} (本仓库开源分支，无需授权码)"
    echo ""
    read -p "请输入您的选择 (1): " version_choice
    echo ""
    
    case "$version_choice" in
        1)
            # 如果选择1，调用免费版函数
            install_free_version
            ;;
        *)
            echo -e "${red}输入无效, 退出安装。${plain}"
            exit 1
            ;;
    esac
}

# ----------------------------------------------------------
# 脚本执行入口
# ----------------------------------------------------------
clear
main_menu

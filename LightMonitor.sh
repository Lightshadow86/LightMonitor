#!/bin/bash

#	GPT写的，凑合用把
#	v0.1
#
#			——Light__shadow

# 检查是否以 root 权限运行
if [ "$EUID" -ne 0 ]; then
    echo "请以 root 权限运行脚本"
    exit 1
fi

# 获取系统架构
ARCH=$(uname -m)
OS=$(uname -s)

# 函数：安装 LightMonitor 服务端
function install_lightmonitor_server() {
    install_lightmonitor "server"
}

# 函数：卸载 LightMonitor 服务端
function uninstall_lightmonitor_server() {
    systemctl stop LightMonitor-Server
    systemctl disable LightMonitor-Server
    rm -f /etc/systemd/system/LightMonitor-Server.service
    rm -rf /etc/LightMonitor
    systemctl daemon-reload
    echo "LightMonitor 服务端已卸载"
}

# 函数：安装 LightMonitor 客户端
function install_lightmonitor_client() {
    install_lightmonitor "client"
}

# 函数：卸载 LightMonitor 客户端
function uninstall_lightmonitor_client() {
    systemctl stop LightMonitor
    systemctl disable LightMonitor
    rm -f /etc/systemd/system/LightMonitor.service
    rm -rf /etc/LightMonitor
    systemctl daemon-reload
    echo "LightMonitor 客户端已卸载"
}

# 函数：安装 LightMonitor (服务端或客户端)
function install_lightmonitor() {
    local type=$1
    local monitor_name=""
    local service_name=""
    local exec_start=""
    local exec_start_params=""

    # 根据系统架构和类型选择合适的下载链接和文件名
    case "$OS" in
        Linux)
            case "$ARCH" in
                x86_64)
                    if [[ "$type" == "server" ]]; then
                        MONITOR_FILE="LightMonitor-Server-linux-amd64"
                        monitor_name="LightMonitor-Server"
                        service_name="LightMonitor-Server"
                    else
                        MONITOR_FILE="LightMonitor-linux-amd64"
                        monitor_name="LightMonitor"
                        service_name="LightMonitor"
                    fi
                    ;;
                aarch64)
                    if [[ "$type" == "server" ]]; then
                        MONITOR_FILE="LightMonitor-Server-linux-arm64"
                        monitor_name="LightMonitor-Server"
                        service_name="LightMonitor-Server"
                    else
                        MONITOR_FILE="LightMonitor-linux-arm64"
                        monitor_name="LightMonitor"
                        service_name="LightMonitor"
                    fi
                    ;;
                *) echo "不支持的架构: $ARCH"; exit 1;;
            esac
            ;;
        Darwin)
            case "$ARCH" in
                x86_64)
                    if [[ "$type" == "server" ]]; then
                        MONITOR_FILE="LightMonitor-Server-darwin-amd64"
                        monitor_name="LightMonitor-Server"
                        service_name="LightMonitor-Server"
                    else
                        MONITOR_FILE="LightMonitor-darwin-amd64"
                        monitor_name="LightMonitor"
                        service_name="LightMonitor"
                    fi
                    ;;
                arm64)
                    if [[ "$type" == "server" ]]; then
                        MONITOR_FILE="LightMonitor-Server-darwin-arm64"
                        monitor_name="LightMonitor-Server"
                        service_name="LightMonitor-Server"
                    else
                        MONITOR_FILE="LightMonitor-darwin-arm64"
                        monitor_name="LightMonitor"
                        service_name="LightMonitor"
                    fi
                    ;;
                *) echo "不支持的架构: $ARCH"; exit 1;;
            esac
            ;;
        *) echo "不支持的操作系统: $OS"; exit 1;;
    esac

    # 下载链接
    DOWNLOAD_URL="https://github.com/Lightshadow86/LightMonitor/releases/download/latest/${MONITOR_FILE}"

    # 创建目录并切换到该目录
    mkdir -p /etc/LightMonitor/
    cd /etc/LightMonitor/
	
	
	# 检查是否已经安装 wget
	if command -v wget &> /dev/null; then
		sleep 0
	else
		echo "正在安装 wget..."
		# 检查操作系统类型
		if [ -f /etc/debian_version ]; then
			# Debian/Ubuntu 系统
			apt-get update -qq && apt-get install -y wget > /dev/null 2>&1
		elif [ -f /etc/redhat-release ]; then
			# CentOS/RHEL 系统
			yum install -y wget > /dev/null 2>&1
		else
			echo "不支持的操作系统"
			exit 1
		fi
	fi

	# 检查 wget 是否安装成功
	if command -v wget &> /dev/null; then
		sleep 0
	else
		echo "wget 安装失败！请手动安装"
		exit 1
	fi

    # 下载 LightMonitor
    wget -O "$monitor_name" "$DOWNLOAD_URL" > /dev/null 2>&1 || { echo "下载失败！"; exit 1; }
    chmod +x "$monitor_name"

    # 创建systemd服务文件
    local service_file="/etc/systemd/system/${service_name}.service"
    exec_start="/etc/LightMonitor/$monitor_name"

    # 获取用户输入的参数 (服务端或客户端)
    if [[ "$type" == "server" ]]; then
		read -p "请输入监听端口 (默认为 712): " listen_port
		listen_port="${listen_port:-712}"
		if [[ "$listen_port" =~ ^[0-9]+$ ]]; then
			listen_port=":$listen_port"
		fi
		echo "监听端口: $listen_port"
        echo "Node API 路径: $node_uri"
        read -p "请输入广播 API 路径 (默认为 /Monitor/Status): " broad_uri
        echo "广播 API 路径: $broad_uri"
        broad_uri="${broad_uri:-"/Monitor/Status"}"
        read -p "请输入后台 API 路径 (默认为 /Monitor/Console): " console_uri
        echo "后台 API 路径: $console_uri"
        console_uri="${console_uri:-"/Monitor/Console"}"
        while true; do
            read -p "请输入节点 Token (不能为空): " token
            if [[ -n "$token" ]]; then
				echo "节点 Token: $token"
                break
            fi
            echo "Token 不能为空，请重新输入。"
        done
        exec_start_params=" -listen $listen_port -node_uri $node_uri -broad_uri $broad_uri -console_uri $console_uri -token $token"

    elif [[ "$type" == "client" ]]; then
        while true; do
			read -p "请输入 API URL 路径 (例如 ws://api.example.com/Monitor/Node): " url
			# 检查 URL 是否为空且以 ws:// 或 wss:// 开头
			if [[ -n "$url" && ( "$url" =~ ^ws:// || "$url" =~ ^wss:// ) ]]; then
				echo "API URL 路径: $url"
				break
			fi
			echo "URL 必须以 ws:// 或 wss:// 开始，请重新输入。"
		done
        while true; do
            read -p "请输入本节点的 Token: " token
            if [[ -n "$token" ]]; then
                break
            fi
            echo "Token 不能为空，请重新输入。"
        done
        exec_start_params=" -url $url -token $token"
        echo "节点 Token: $token"
    fi

    # 直接生成整个service文件内容
    cat > "$service_file" <<EOF
[Unit]
Description=LightMonitor ${type} Service
After=network.target
Wants=network.target

[Service]
User=root
Group=root
Type=simple
ExecStart=$exec_start$exec_start_params
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

    chmod 644 "$service_file"
    systemctl daemon-reload
    systemctl enable "$service_name"
    systemctl start "$service_name"
    echo "LightMonitor ${type} 安装完成！服务状态："
    systemctl status "$service_name"
	
	exit 0
}


while true; do
    clear
    echo "=================================================="
    echo "LightMonitor 一键安装脚本 v0.1"
    echo "=================================================="
    echo "1. 安装主控后端"
    echo "2. 卸载主控后端"
    echo "4. 安装被控"
    echo "5. 卸载被控"
    echo "0. 退出"
    echo "============================================="

    read -p "请选择一个选项 (0, 1, 2, 4, 5): " choice

    case $choice in
        0) exit 0 ;;
        1) install_lightmonitor_server ;;
        2) uninstall_lightmonitor_server ;;
        4) install_lightmonitor_client ;;
        5) uninstall_lightmonitor_client ;;
        *) echo "无效选项" ;;
    esac

    echo
    read -p "按 Enter 键继续..."
    clear
done

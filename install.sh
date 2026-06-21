#!/bin/bash
# HS-NAS-R1-Panel — One-click install for Hikvision NAS R1
# Usage: curl -sSL https://raw.githubusercontent.com/fayfoxcat/HS-NAS-R1-Panel/master/install.sh | sudo bash

set -euo pipefail
BIN="/opt/nas-panel/r1-panel"
REPO="https://github.com/fayfoxcat/HS-NAS-R1-Panel"

echo "=== HS-NAS-R1-Panel 一键安装 ==="
echo ""

# 1. 安装系统依赖
echo "[1/3] 安装依赖..."
apt-get update -qq
apt-get install -y -qq cog smartmontools 2>/dev/null

# 2. 下载最新版本
echo "[2/3] 下载二进制..."
mkdir -p /opt/nas-panel
curl -sSL "${REPO}/releases/latest/download/r1-panel" -o "${BIN}" 2>/dev/null || {
    echo "错误：未找到预编译二进制。"
    echo "手动编译并上传："
    echo "  git clone ${REPO}.git"
    echo "  cd HS-NAS-R1-Panel"
    echo "  GOOS=linux GOARCH=amd64 go build -ldflags=\"-s -w\" -o r1-panel ."
    echo "  scp r1-panel root@nas:/opt/nas-panel/"
    exit 1
}
chmod +x "${BIN}"

# 3. 安装 systemd 服务
echo "[3/3] 安装系统服务..."
"${BIN}" install
systemctl enable r1-panel 2>/dev/null || true
systemctl start r1-panel 2>/dev/null || true

echo ""
echo "=== 安装完成 ==="
echo "  服务已安装（随机端口，仅回环地址，屏幕自启）。"
echo "  如需局域网访问：${BIN} install -p 8088"
echo ""
echo "  管理命令："
echo "    systemctl stop/start/restart r1-panel"
echo "    ${BIN} uninstall"

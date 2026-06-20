#!/bin/bash
# HS-NAS-R1-Panel — One-click install for Hikvision NAS R1
# Usage: curl -sSL https://raw.githubusercontent.com/fayfoxcat/HS-NAS-R1-Panel/master/install.sh | sudo bash

set -euo pipefail
BIN="/opt/nas-panel/hs-nas-r1-panel"
REPO="https://github.com/fayfoxcat/HS-NAS-R1-Panel"

echo "=== HS-NAS-R1-Panel Install ==="
echo ""

# 1. Install system dependencies
echo "[1/3] Installing dependencies..."
apt-get update -qq
apt-get install -y -qq cog smartmontools 2>/dev/null

# 2. Download latest release
echo "[2/3] Downloading binary..."
mkdir -p /opt/nas-panel
curl -sSL "${REPO}/releases/latest/download/hs-nas-r1-panel" -o "${BIN}" 2>/dev/null || {
    echo "ERROR: No pre-built binary found."
    echo "Build manually and upload:"
    echo "  git clone ${REPO}.git"
    echo "  cd HS-NAS-R1-Panel"
    echo "  GOOS=linux GOARCH=amd64 go build -ldflags=\"-s -w\" -o hs-nas-r1-panel ."
    echo "  scp hs-nas-r1-panel root@nas:/opt/nas-panel/"
    exit 1
}
chmod +x "${BIN}"

# 3. Install systemd service (headless by default)
echo "[3/3] Installing systemd service..."
"${BIN}" --install
systemctl enable hs-nas-r1-panel 2>/dev/null || true

echo ""
echo "=== Done ==="
echo "  Service installed (headless). To enable web dashboard:"
echo "    ${BIN} --install --web"
echo "    systemctl restart hs-nas-r1-panel"
echo "  Web:  http://$(hostname -I | awk '{print $1}'):8088"
echo ""
echo "  Manage:"
echo "    systemctl stop/start/restart hs-nas-r1-panel"
echo "    ${BIN} --uninstall"
echo ""

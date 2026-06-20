#!/bin/bash
# HS-NAS-R1-Panel — One-click install for Hikvision NAS R1
# Usage: curl -sSL https://raw.githubusercontent.com/fayfoxcat/HS-NAS-R1-Panel/master/install.sh | sudo bash

set -euo pipefail
BIN="/opt/nas-panel/hs-nas-r1-panel"
REPO="https://github.com/fayfoxcat/HS-NAS-R1-Panel"

echo "=== HS-NAS-R1-Panel Install ==="
echo ""

# 1. Install system dependencies
echo "[1/4] Installing dependencies..."
apt-get update -qq
apt-get install -y -qq cog smartmontools 2>/dev/null

# 2. Download latest release (fallback: build from source)
echo "[2/4] Downloading binary..."
mkdir -p /opt/nas-panel
LATEST=$(curl -sSL "${REPO}/releases/latest/download/hs-nas-r1-panel" -o "${BIN}" 2>/dev/null && echo "ok" || echo "fail")
if [ "$LATEST" = "fail" ]; then
    echo "  No pre-built release, building from source..."
    if ! command -v go &>/dev/null; then
        apt-get install -y -qq golang-go 2>/dev/null || {
            echo "ERROR: Go not available and no pre-built binary found."
            exit 1
        }
    fi
    TMP=$(mktemp -d)
    git clone "${REPO}.git" "${TMP}" 2>/dev/null
    cd "${TMP}"
    GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "${BIN}" .
    rm -rf "${TMP}"
fi
chmod +x "${BIN}"

# 3. Install systemd service
echo "[3/4] Installing systemd service..."
"${BIN}" --install
systemctl enable hs-nas-r1-panel 2>/dev/null || true
systemctl start hs-nas-r1-panel 2>/dev/null || true

# 4. Start screen
echo "[4/4] Starting screen..."
pkill cog 2>/dev/null || true
sleep 1
setsid cog -P drm http://localhost:8088 >/dev/null 2>&1 </dev/null &

echo ""
echo "=== Done ==="
echo "  Web:  http://$(hostname -I | awk '{print $1}'):8088"
echo "  Screen: cog running on framebuffer"
echo ""
echo "  Manage:"
echo "    systemctl stop/start/restart hs-nas-r1-panel"
echo "    ${BIN} --uninstall"
echo ""

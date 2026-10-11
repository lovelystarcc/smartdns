#!/bin/bash
set -e

# SmartNet one-line installer (v2.0.0)
echo "==> Installing SmartNet CLI (formerly SmartDNS)..."

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        echo "Error: Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

TARGET_DIR="/usr/local/bin"
if [ "$OS" = "darwin" ] && [ -d "/opt/homebrew/bin" ]; then
    TARGET_DIR="/opt/homebrew/bin"
fi

BIN_NAME="smartnet-${OS}-${ARCH}"
TARGET_BIN="${TARGET_DIR}/smartnet"
SYMLINK_BIN="${TARGET_DIR}/smartdns"

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
    if [ ! -w "$TARGET_DIR" ] || [ -f "$TARGET_BIN" -a ! -w "$TARGET_BIN" ]; then
        SUDO="sudo"
    fi
fi

# Stop legacy daemon before replacing if exists
if command -v smartdns >/dev/null 2>&1; then
    smartdns service stop >/dev/null 2>&1 || true
fi

# If installing from local build or downloading prebuilt from GitHub
if [ -f "dist/${BIN_NAME}" ]; then
    $SUDO cp "dist/${BIN_NAME}" "$TARGET_BIN"
elif command -v go >/dev/null 2>&1 && [ -f "main.go" ]; then
    go build -ldflags="-s -w" -o /tmp/smartnet .
    $SUDO mv -f /tmp/smartnet "$TARGET_BIN"
else
    echo "Downloading smartnet for ${OS}/${ARCH} from GitHub release..."
    DOWNLOAD_URL="https://github.com/lovelystarcc/smartnet/releases/latest/download/${BIN_NAME}"
    TMP_BIN="/tmp/smartnet_download_$$"
    rm -f "$TMP_BIN"
    if command -v curl >/dev/null 2>&1; then
        if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_BIN"; then
            FALLBACK_URL="https://github.com/lovelystarcc/smartnet/releases/latest/download/smartdns-${OS}-${ARCH}"
            curl -fsSL "$FALLBACK_URL" -o "$TMP_BIN"
        fi
    elif command -v wget >/dev/null 2>&1; then
        if ! wget -qO "$TMP_BIN" "$DOWNLOAD_URL"; then
            FALLBACK_URL="https://github.com/lovelystarcc/smartnet/releases/latest/download/smartdns-${OS}-${ARCH}"
            wget -qO "$TMP_BIN" "$FALLBACK_URL"
        fi
    else
        echo "Error: curl or wget is required to download prebuilt binary."
        exit 1
    fi

    chmod +x "$TMP_BIN"

    # Apply ad-hoc signature on macOS (required for arm64 Apple Silicon)
    if [ "$OS" = "darwin" ] && command -v codesign >/dev/null 2>&1; then
        codesign -s - -f "$TMP_BIN" >/dev/null 2>&1 || true
    fi

    # Atomically replace destination binary
    $SUDO mv -f "$TMP_BIN" "$TARGET_BIN"
    $SUDO chmod +x "$TARGET_BIN"
fi

# Create backward-compatible symlink so 'smartdns' command continues to work
$SUDO ln -sf "$TARGET_BIN" "$SYMLINK_BIN"

echo "==> SmartNet successfully installed to $TARGET_BIN"
echo "==> Legacy symlink created: $SYMLINK_BIN -> $TARGET_BIN"
"$TARGET_BIN" version

# Restart background daemon if already running
if "$TARGET_BIN" service status 2>&1 | grep -q "running"; then
    echo "==> Reloading background daemon..."
    $SUDO "$TARGET_BIN" service restart || true
fi

echo ""
echo "Quick start:"
echo "  sudo smartnet setup <your-doh-url> [profile-name]"
echo "  smartnet on"
echo "  smartnet vpn set vless://uuid@server:port?security=reality&sni=apple.com&pbk=...&sid=..."
echo "  smartnet vpn on       # Activate anti-DPI VLESS-Reality tunnel"
echo "  smartnet tun on       # Transparent routing for all system apps"
echo "  smartnet status"
echo "  smartnet off"

#!/bin/bash
set -e

# SmartDNS one-line installer
echo "==> Installing SmartDNS CLI..."

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
if [ "$OS" = "darwin" ] && [ -d "/opt/homebrew/bin" ] && [ -w "/opt/homebrew/bin" ]; then
    TARGET_DIR="/opt/homebrew/bin"
fi

SUDO=""
if [ ! -w "$TARGET_DIR" ]; then
    SUDO="sudo"
fi

BIN_NAME="smartdns-${OS}-${ARCH}"
TARGET_BIN="${TARGET_DIR}/smartdns"

# If installing from local build or downloading prebuilt from GitHub
if [ -f "dist/${BIN_NAME}" ]; then
    $SUDO cp "dist/${BIN_NAME}" "$TARGET_BIN"
elif command -v go >/dev/null 2>&1 && [ -f "main.go" ]; then
    go build -ldflags="-s -w" -o /tmp/smartdns .
    $SUDO mv -f /tmp/smartdns "$TARGET_BIN"
else
    echo "Downloading smartdns for ${OS}/${ARCH} from GitHub release..."
    DOWNLOAD_URL="https://github.com/lovelystarcc/smartdns/releases/latest/download/${BIN_NAME}"
    if command -v curl >/dev/null 2>&1; then
        $SUDO curl -fsSL "$DOWNLOAD_URL" -o "$TARGET_BIN"
    elif command -v wget >/dev/null 2>&1; then
        $SUDO wget -qO "$TARGET_BIN" "$DOWNLOAD_URL"
    else
        echo "Error: curl or wget is required to download prebuilt binary."
        exit 1
    fi
fi

$SUDO chmod +x "$TARGET_BIN"

# Apply ad-hoc signature on macOS (required for arm64 Apple Silicon)
if [ "$OS" = "darwin" ] && command -v codesign >/dev/null 2>&1; then
    $SUDO codesign -s - -f "$TARGET_BIN" >/dev/null 2>&1 || true
fi

echo "==> SmartDNS successfully installed to $TARGET_BIN"
"$TARGET_BIN" version

# Restart background daemon if already running
if "$TARGET_BIN" service status 2>&1 | grep -q "running"; then
    echo "==> Restarting background service with the updated version..."
    $SUDO "$TARGET_BIN" service restart || true
fi

echo ""
echo "Quick start:"
echo "  sudo smartdns setup <your-doh-url> [profile-name]"
echo "  smartdns on"
echo "  smartdns test"
echo "  smartdns off"

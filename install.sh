#!/bin/bash
set -e

echo "==> Installing SmartNet CLI..."

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

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
    if [ ! -w "$TARGET_DIR" ] || [ -f "$TARGET_BIN" -a ! -w "$TARGET_BIN" ]; then
        SUDO="sudo"
    fi
fi

if command -v smartnet >/dev/null 2>&1; then
    smartnet service stop >/dev/null 2>&1 || true
fi

if [ -f "dist/${BIN_NAME}" ]; then
    $SUDO cp "dist/${BIN_NAME}" "$TARGET_BIN"
elif command -v go >/dev/null 2>&1 && [ -f "main.go" ]; then
    go build -ldflags="-s -w" -o /tmp/smartnet .
    $SUDO mv -f /tmp/smartnet "$TARGET_BIN"
else
    echo "Downloading ${BIN_NAME} from GitHub release..."
    DOWNLOAD_URL="https://github.com/lovelystarcc/smartnet/releases/latest/download/${BIN_NAME}"
    TMP_BIN="/tmp/smartnet_download_$$"
    rm -f "$TMP_BIN"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$DOWNLOAD_URL" -o "$TMP_BIN"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "$TMP_BIN" "$DOWNLOAD_URL"
    else
        echo "Error: curl or wget is required."
        exit 1
    fi

    chmod +x "$TMP_BIN"

    if [ "$OS" = "darwin" ] && command -v codesign >/dev/null 2>&1; then
        codesign -s - -f "$TMP_BIN" >/dev/null 2>&1 || true
    fi

    $SUDO mv -f "$TMP_BIN" "$TARGET_BIN"
    $SUDO chmod +x "$TARGET_BIN"
fi

if [ "$OS" = "darwin" ] && command -v codesign >/dev/null 2>&1; then
    codesign -s - -f "$TARGET_BIN" >/dev/null 2>&1 || true
fi

echo "==> Installed to $TARGET_BIN"
"$TARGET_BIN" version

if "$TARGET_BIN" service status 2>&1 | grep -q "running"; then
    echo "==> Restarting service..."
    $SUDO "$TARGET_BIN" service restart || true
fi

echo ""
echo "Quick start:"
echo "  sudo smartnet setup <url> [name]"
echo "  smartnet on"
echo "  smartnet test"
echo "  smartnet off"

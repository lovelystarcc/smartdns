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
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

TARGET_DIR="/usr/local/bin"
if [ "$OS" = "darwin" ] && [ -d "/opt/homebrew/bin" ] && [ -w "/opt/homebrew/bin" ]; then
    TARGET_DIR="/opt/homebrew/bin"
fi

BIN_NAME="smartdns-${OS}-${ARCH}"

# If building from source or installing prebuilt
if [ -f "dist/${BIN_NAME}" ]; then
    cp "dist/${BIN_NAME}" "${TARGET_DIR}/smartdns"
elif command -v go >/dev/null 2>&1 && [ -f "main.go" ]; then
    go build -ldflags="-s -w" -o "${TARGET_DIR}/smartdns" .
else
    echo "Installing smartdns to ${TARGET_DIR}/smartdns..."
    # Placeholder for GitHub release download URL when uploaded to a repo
fi

chmod +x "${TARGET_DIR}/smartdns"
echo "==> SmartDNS successfully installed to ${TARGET_DIR}/smartdns"
smartdns version

echo ""
echo "Quick start:"
echo "  sudo smartdns setup <your-doh-url> [profile-name]"
echo "  smartdns on"
echo "  smartdns test"
echo "  smartdns off"

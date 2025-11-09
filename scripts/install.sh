#!/bin/bash
# crosh installer script
# Usage: curl -fsSL https://crosh.boomyao.com/scripts/install.sh | bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
REPO="boomyao/crosh"
BINARY_NAME="crosh"
INSTALL_DIR="/usr/local/bin"
VERSION="${CROSH_VERSION:-latest}"

echo -e "${GREEN}crosh installer${NC}"
echo "================"
echo ""

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        linux)
            OS="linux"
            ;;
        darwin)
            OS="darwin"
            ;;
        *)
            echo -e "${RED}Error: Unsupported OS: $OS${NC}"
            exit 1
            ;;
    esac

    case "$ARCH" in
        x86_64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            echo -e "${RED}Error: Unsupported architecture: $ARCH${NC}"
            exit 1
            ;;
    esac

    echo -e "Detected platform: ${GREEN}${OS}/${ARCH}${NC}"
}

# Get latest version from Cloudflare CDN
get_latest_version() {
    if [ "$VERSION" != "latest" ]; then
        echo -e "Using specified version: ${GREEN}${VERSION}${NC}"
        return
    fi

    echo "Fetching latest version..."
    
    # Get version from Cloudflare Worker API
    VERSION=$(curl -s "https://crosh.boomyao.com/api/version" 2>/dev/null | grep -o '"version":"[^"]*"' | head -1 | sed 's/"version":"//;s/"$//')
    
    if [ -z "$VERSION" ]; then
        echo -e "${RED}Error: Failed to get latest version${NC}"
        echo "Please specify version manually: CROSH_VERSION=v1.0.0 bash install.sh"
        exit 1
    fi

    echo -e "Latest version: ${GREEN}${VERSION}${NC}"
}

# Download binary from Cloudflare CDN
download_binary() {
    BINARY_FILE="${BINARY_NAME}-${OS}-${ARCH}"
    
    if [ "$VERSION" = "latest" ]; then
        get_latest_version
    fi

    TMP_FILE="/tmp/${BINARY_NAME}.tmp"
    
    # Download from Cloudflare Worker CDN
    CDN_URL="https://crosh.boomyao.com/dist/${BINARY_FILE}"
    echo "Downloading from Cloudflare CDN..."
    echo "URL: $CDN_URL"
    
    if curl -fsSL -o "$TMP_FILE" "$CDN_URL" 2>/dev/null; then
        echo -e "${GREEN}✓${NC} Downloaded from Cloudflare CDN"
    else
        echo -e "${RED}Error: Failed to download binary${NC}"
        echo ""
        echo "Please try manual installation:"
        echo "1. Download from: https://github.com/${REPO}/releases"
        echo "2. Or use CDN: https://crosh.boomyao.com/dist/${BINARY_FILE}"
        exit 1
    fi

    # Verify the downloaded file is not empty
    if [ ! -s "$TMP_FILE" ]; then
        echo -e "${RED}Error: Downloaded file is empty${NC}"
        exit 1
    fi
}

# Install binary
install_binary() {
    echo "Installing to $INSTALL_DIR..."

    # Check if we need sudo
    if [ -w "$INSTALL_DIR" ]; then
        SUDO=""
    else
        SUDO="sudo"
        echo "Need sudo permission to install to $INSTALL_DIR"
    fi

    $SUDO mv "/tmp/${BINARY_NAME}.tmp" "$INSTALL_DIR/$BINARY_NAME"
    $SUDO chmod +x "$INSTALL_DIR/$BINARY_NAME"

    echo -e "${GREEN}✓${NC} Installed to $INSTALL_DIR/$BINARY_NAME"
}

# Verify installation
verify_installation() {
    if command -v "$BINARY_NAME" >/dev/null 2>&1; then
        VERSION_OUTPUT=$($BINARY_NAME version 2>&1 || echo "crosh installed")
        echo -e "${GREEN}✓${NC} Installation verified"
        echo "  $VERSION_OUTPUT"
        return 0
    else
        echo -e "${YELLOW}Warning: $BINARY_NAME not found in PATH${NC}"
        echo "You may need to add $INSTALL_DIR to your PATH"
        return 1
    fi
}

# Post-install instructions
print_instructions() {
    echo ""
    echo -e "${GREEN}Installation complete!${NC}"
    echo ""
    echo "Quick start:"
    echo "  1. Test connectivity:    crosh test"
    echo "  2. Auto-configure:       crosh auto"
    echo "  3. Check status:         crosh status"
    echo ""
    echo "For proxy support:"
    echo "  1. Add subscription:     crosh proxy add <subscription-url>"
    echo "  2. Enable proxy:         crosh proxy enable"
    echo ""
    echo "For more information:"
    echo "  crosh help"
    echo "  https://github.com/$REPO"
    echo ""
}

# Main installation flow
main() {
    detect_platform
    download_binary
    install_binary
    
    if verify_installation; then
        print_instructions
    fi
}

# Run main
main

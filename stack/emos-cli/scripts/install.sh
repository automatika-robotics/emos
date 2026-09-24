#!/bin/bash
set -e

# --- Configuration ---
GITHUB_ORG="automatika-robotics"
REPO="emos"
BINARY_NAME="emos"
INSTALL_DIR="/usr/local/bin"

# --- Functions ---
info()    { echo "[INFO] $1"; }
success() { printf "\033[0;32m[SUCCESS] %s\033[0m\n" "$1"; }
error()   { printf "\033[0;31m[ERROR] %s\033[0m\n" "$1" >&2; exit 1; }

check_root() {
    if [ "$EUID" -ne 0 ]; then
        error "This script must be run with sudo or as root."
    fi
}

detect_arch() {
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64)  echo "amd64" ;;
        aarch64) echo "arm64" ;;
        *)       error "Unsupported architecture: $ARCH" ;;
    esac
}

install_binary() {
    local arch
    arch=$(detect_arch)

    info "Detected architecture: $arch"

    # Stable installs take the latest release. The dev channel takes the
    # newest nightly, which is published as a pre-release.
    local api="https://api.github.com/repos/$GITHUB_ORG/$REPO/releases"
    local release_url="$api/latest"
    if [ "$CHANNEL" = "dev" ]; then
        local tag
        tag=$(curl -sSL "$api" | grep -o '"tag_name": *"v[^"]*-dev\.[^"]*"' | head -1 | cut -d '"' -f 4)
        [ -n "$tag" ] || error "No dev build has been published yet."
        info "Installing the dev build $tag"
        release_url="$api/tags/$tag"
    fi
    local download_url
    download_url=$(curl -sSL "$release_url" | \
        grep "browser_download_url.*emos-linux-${arch}" | \
        head -1 | cut -d '"' -f 4)

    if [ -z "$download_url" ]; then
        error "Could not find a release binary for linux-$arch. Check https://github.com/$GITHUB_ORG/$REPO/releases"
    fi

    info "Downloading emos CLI from $download_url..."
    curl -sSLf "$download_url" -o "/tmp/$BINARY_NAME" || error "Download failed."

    chmod +x "/tmp/$BINARY_NAME"
    mv "/tmp/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"

    success "emos CLI installed to $INSTALL_DIR/$BINARY_NAME"
}

# --- Main ---
# The channel comes as --dev (`curl ... | sudo bash -s -- --dev`) or as
# EMOS_CHANNEL=dev; sudo drops the environment, so the flag is the safe form.
main() {
    CHANNEL="${EMOS_CHANNEL:-stable}"
    [ "${1:-}" = "--dev" ] && CHANNEL=dev
    check_root
    install_binary
    info "Run 'emos --help' to get started."
}

main "$@"

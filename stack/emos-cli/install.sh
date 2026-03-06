#!/bin/bash
set -e # Exit immediately if a command exits with a non-zero status.

# --- Configuration ---
GITHUB_ORG="automatika-robotics"
REPO="emos-cli"
BRANCH="main"
BASE_URL="https://raw.githubusercontent.com/$GITHUB_ORG/$REPO/$BRANCH"

EMOS_CLI_URL="$BASE_URL/emos-cli.sh"
RECIPE_RUN_URL="$BASE_URL/recipe_run.sh"
MAPPING_RUN_URL="$BASE_URL/mapping_run.sh"
EMOS_LIB_URL="$BASE_URL/emos-lib.sh"

# Install Locations
BIN_PATH="/usr/local/bin"
LIB_DIR="/usr/local/lib/emos"
CLI_NAME="emos"
CLI_SYMLINK="$BIN_PATH/$CLI_NAME"
CLI_TARGET_SCRIPT="$LIB_DIR/emos-cli.sh"

# --- UI Functions ---
info() {
    echo "[INFO] $1"
}

success() {
    printf "\033[0;32m[SUCCESS] %s\033[0m\n" "$1"
}

error() {
    printf "\033[0;31m[ERROR] %s\033[0m\n" "$1" >&2
    exit 1
}

# --- Core Logic Functions ---
check_root() {
    if [ "$EUID" -ne 0 ]; then
        error "This script must be run with sudo or as the root user."
    fi
}

check_docker() {
    if ! command -v docker &> /dev/null; then
        error "Docker is not installed. Please install Docker before running this script. See: https://docs.docker.com/engine/install/"
    fi
    info "Docker installation found."
}

detect_package_manager() {
    if command -v apt-get &>/dev/null; then
        echo "apt"
    elif command -v dnf &>/dev/null; then
        echo "dnf"
    elif command -v yum &>/dev/null; then
        echo "yum"
    elif command -v pacman &>/dev/null; then
        echo "pacman"
    elif command -v brew &>/dev/null; then
        echo "brew"
    else
        echo "unknown"
    fi
}

install_dependencies() {
    info "Detecting package manager..."
    PKG_MANAGER=$(detect_package_manager)
    info "Installing dependencies..."
    case "$PKG_MANAGER" in
        apt)
            info "Installing: curl, jq, gpg..."
            apt-get update
            apt-get install -y curl jq gpg
            info "Setting up Charm repository to install gum..."
            mkdir -p /etc/apt/keyrings
            curl -fsSL https://repo.charm.sh/apt/gpg.key | gpg --dearmor > /etc/apt/keyrings/charm.gpg
            echo "deb [signed-by=/etc/apt/keyrings/charm.gpg] https://repo.charm.sh/apt/ * *" | tee /etc/apt/sources.list.d/charm.list
            apt-get update
            apt-get install -y gum
            ;;
        dnf|yum)
            info "Installing: gum, curl, jq..."
            dnf install -y gum curl jq
            ;;
        pacman)
            info "Installing: gum, curl, jq..."
            pacman -Syu --noconfirm gum curl jq
            ;;
        brew)
            info "Installing: gum, curl, jq..."
            brew install gum curl jq
            ;;
        *)
            error "Unsupported package manager. Please install dependencies manually: gum, curl, and jq."
            ;;
    esac
    success "Dependencies installed."
}

# Helper function for downloading
download_file() {
    local url="$1"
    local dest="$2"
    if ! curl -sSLf "$url" -o "$dest"; then
        error "Failed to download: $url"
    fi
}

install_cli() {
    info "Creating directories..."
    mkdir -p "$LIB_DIR"
    mkdir -p "$BIN_PATH"

    info "Downloading the latest emos CLI tools to $LIB_DIR..."

    download_file "$EMOS_CLI_URL" "$LIB_DIR/emos-cli.sh"
    download_file "$RECIPE_RUN_URL" "$LIB_DIR/recipe_run.sh"
    download_file "$MAPPING_RUN_URL" "$LIB_DIR/mapping_run.sh"
    download_file "$EMOS_LIB_URL" "$LIB_DIR/emos-lib.sh"

    info "Setting permissions..."
    chmod +x "$LIB_DIR/emos-cli.sh"
    chmod +x "$LIB_DIR/recipe_run.sh"
    chmod +x "$LIB_DIR/mapping_run.sh"

    info "Creating symlink at $CLI_SYMLINK..."
    # Use -f to force overwrite any existing symlink
    ln -s -f "$CLI_TARGET_SCRIPT" "$CLI_SYMLINK"

    success "emos CLI tools installed successfully."
}

update_cli() {
    info "Checking for emos CLI updates..."

    # Check if the symlink exists
    if [ ! -L "$CLI_SYMLINK" ]; then
        error "emos is not installed. Cannot update. Run the installer without arguments."
    fi

    # Grep the target script, not the symlink
    local local_version
    local_version=$(grep '^EMOS_VERSION=' "$CLI_TARGET_SCRIPT" | cut -d'=' -f2 | tr -d '"')

    if [ -z "$local_version" ]; then
         info "Could not determine local version. Forcing update..."
         install_cli
         exit 10 # Signal update
    fi

    local remote_version
    remote_version=$(curl -sSLf "$EMOS_CLI_URL" | grep '^EMOS_VERSION=' | cut -d'=' -f2 | tr -d '"')
    if [ -z "$remote_version" ]; then
        error "Could not determine the latest version available for download."
    fi

    info "Local version: $local_version, Remote version: $remote_version"

    if [ "$remote_version" != "$local_version" ]; then
        info "A new version ($remote_version) is available. Updating all tools..."
        install_cli
        exit 10 # Signal that an update was performed
    else
        info "You already have the latest version of the emos CLI."
        exit 0 # Signal "up to date"
    fi
}

main() {
    check_root
    case "$1" in
        update)
            update_cli
            ;;
        "")
            info "Starting first-time installation of emos..."
            check_docker
            install_dependencies
            install_cli
            info "Run 'emos --help' to get started."
            ;;
        *)
            error "Unknown command for installer: '$1'"
            ;;
    esac
}

main "$@"

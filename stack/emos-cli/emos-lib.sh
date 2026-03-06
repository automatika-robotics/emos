#!/bin/bash

# ==============================================================================
# emos-lib.sh - Common library functions for EmbodiedOS scripts
# ==============================================================================

# --- Theme ---
THEME_RED="#d54e53"
THEME_BLUE="#81a2be"
THEME_NEUTRAL="#EEF2F3"

# --- Core Logging Functions ---

# Usage: log "Your message here"
log() {
    gum style --foreground 240 "│"
    gum style --foreground 250 "├─ $1"
}

# Usage: success "Your message here"
success() {
    gum style --foreground 2 "│"
    gum style --foreground 2 "├─ ✔ Success: $1"
}

# Usage: warn "Your message here"
warn() {
    gum style --foreground 3 "│"
    gum style --foreground 3 "├─ ! Warning: $1"
}

# Usage: error "Your message here" (exits the script)
error() {
    gum style --foreground 1 "│"
    gum style --foreground 1 "├─ ✖ Error: $1"
}

# Usage: print_header "YOUR TITLE"
print_header() {
    gum style --bold --padding "1 2" --border thick --border-foreground "$THEME_BLUE" --foreground "$THEME_BLUE" "$1"
}

# --- Spinner Utility ---

# A wrapper for gum spin to show a loader for long-running commands
# This function will call success() or error() (and exit) on its own.
# Usage: run_with_spinner "Doing a thing..." "my_command --with --args"
run_with_spinner() {
  local title="$1"
  local cmd="$2"
  local tmpfile
  tmpfile=$(mktemp)

  gum spin --spinner dot --title "$title" -- bash -c "$cmd >$tmpfile 2>&1"
  local EXIT_CODE=$?

  if [ $EXIT_CODE -eq 0 ]; then
    success "$title"
    rm -f "$tmpfile"
    return 0
  else
    error "$title"
    gum style --faint "  Command failed with output:"
    gum format -- "$(cat "$tmpfile")"
    rm -f "$tmpfile"
    return 1
  fi
}

# --- Dependency Check ---

# Usage: check_dependencies
check_dependencies() {
    local missing_deps=()
    # Check for all common dependencies across all scripts
    for dep in gum docker curl jq; do
        if ! command -v "$dep" &> /dev/null; then
            missing_deps+=("$dep")
        fi
    done
    if [ ${#missing_deps[@]} -ne 0 ]; then
        # Use gum style, but don't use the error() function
        # in case gum itself is the missing dependency.
        echo "Error: Missing required dependencies: ${missing_deps[*]}. Please install them to continue."
        exit 1
    fi
}

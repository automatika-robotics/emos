#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "=== EMOS Documentation Build ==="

# Ensure submodules are initialized (needed for autodoc2 API reference generation)
echo "[1/2] Checking submodules..."
git submodule update --init --recursive

# Build docs
echo "[2/2] Building Sphinx documentation..."
cd docs
sphinx-build -b html . _build/html

echo ""
echo "=== Build complete ==="
echo "Open docs/_build/html/index.html to view the site"
echo "Or run: python -m http.server -d docs/_build/html 8000"

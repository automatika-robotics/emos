#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "=== EMOS Documentation Build ==="

# Ensure submodules are initialized
echo "[1/3] Checking submodules..."
git submodule update --init --recursive

# Copy static assets from submodules
echo "[2/3] Copying static assets..."
mkdir -p docs/_static/images/diagrams

# Diagrams from Kompass
cp stack/kompass/docs/_static/images/diagrams/* docs/_static/images/diagrams/ 2>/dev/null || true

# Diagrams from Sugarcoat
cp stack/sugarcoat/docs/_static/images/diagrams/* docs/_static/images/diagrams/ 2>/dev/null || true

# Images from Kompass (simulation screenshots, algorithm visualizations)
cp -r stack/kompass/docs/_static/images/*.png docs/_static/images/ 2>/dev/null || true
cp -r stack/kompass/docs/_static/images/*.gif docs/_static/images/ 2>/dev/null || true

# Images from Sugarcoat
cp -r stack/sugarcoat/docs/_static/images/*.png docs/_static/images/ 2>/dev/null || true
cp -r stack/sugarcoat/docs/_static/images/*.gif docs/_static/images/ 2>/dev/null || true

# GIFs and diagrams from EmbodiedAgents
cp stack/embodied-agents/docs/_static/agents_ui.gif docs/_static/ 2>/dev/null || true
cp stack/embodied-agents/docs/_static/complete_*.png docs/_static/ 2>/dev/null || true

# Build docs
echo "[3/3] Building Sphinx documentation..."
cd docs
sphinx-build -b html . _build/html

echo ""
echo "=== Build complete ==="
echo "Open docs/_build/html/index.html to view the site"
echo "Or run: python -m http.server -d docs/_build/html 8000"

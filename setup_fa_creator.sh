#!/bin/bash
# =============================================================================
# FA Creator Setup Script
# =============================================================================
# One-time setup to build the FA Creator application.
# After running this, use ./run_fa_creator.sh to launch the app.
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GO_MODULE_DIR="$SCRIPT_DIR/go_module"

echo ""
echo "========================================"
echo "   FA Creator - Setup"
echo "   MBII Full Authentic Content Editor"
echo "========================================"
echo ""

# Check for Go
if ! command -v go &> /dev/null; then
    echo "ERROR: Go is not installed."
    echo ""
    echo "Install Go from: https://go.dev/dl/"
    echo ""
    echo "  macOS:   brew install go"
    echo "  Ubuntu:  sudo apt install golang"
    echo "  Windows: Download from go.dev"
    echo ""
    exit 1
fi

echo "Go version: $(go version)"
echo ""

# Build the application
echo "Building FA Creator..."
cd "$GO_MODULE_DIR"

# Detect platform and set output name
if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" || "$OSTYPE" == "cygwin" ]]; then
    OUTPUT="fa_creator.exe"
else
    OUTPUT="fa_creator"
fi

go build -o "$OUTPUT"

if [ $? -eq 0 ]; then
    echo ""
    echo "========================================"
    echo "   Setup Complete!"
    echo "========================================"
    echo ""
    echo "To launch FA Creator:"
    echo ""
    echo "  ./run_fa_creator.sh"
    echo ""
    echo "Or directly:"
    echo ""
    echo "  cd go_module && ./$OUTPUT"
    echo ""
else
    echo ""
    echo "ERROR: Build failed. Check the error messages above."
    exit 1
fi

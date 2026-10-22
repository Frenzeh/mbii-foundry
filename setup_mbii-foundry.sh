#!/bin/bash
# =============================================================================
# MBII Foundry — Setup Script
# =============================================================================
# One-time setup to build MBII Foundry.
# After running this, use ./run_mbii-foundry.sh to launch the app.
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GO_MODULE_DIR="$SCRIPT_DIR/go_module"

echo ""
echo "========================================"
echo "   MBII Foundry — Setup"
echo "   Visual Content Editor for MBII"
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
echo "Building MBII Foundry..."
cd "$GO_MODULE_DIR"

# Detect platform and set output name
if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" || "$OSTYPE" == "cygwin" ]]; then
    OUTPUT="mbii-foundry.exe"
else
    OUTPUT="mbii-foundry"
fi

if [[ "$OSTYPE" == "darwin"* ]]; then
    export MACOSX_DEPLOYMENT_TARGET=13.0
    if [ "$(/usr/libexec/PlistBuddy -c 'Print :LSMinimumSystemVersion' "$SCRIPT_DIR/macos/Info.plist")" != "$MACOSX_DEPLOYMENT_TARGET" ]; then
        echo "ERROR: Info.plist minimum must match macOS $MACOSX_DEPLOYMENT_TARGET" >&2
        exit 1
    fi
    export CGO_CFLAGS="${CGO_CFLAGS:-} -mmacosx-version-min=$MACOSX_DEPLOYMENT_TARGET"
    export CGO_CXXFLAGS="${CGO_CXXFLAGS:-} -mmacosx-version-min=$MACOSX_DEPLOYMENT_TARGET"
    export CGO_LDFLAGS="${CGO_LDFLAGS:-} -mmacosx-version-min=$MACOSX_DEPLOYMENT_TARGET"
    BUILD_STAGE="$(mktemp -d "${TMPDIR:-/tmp}/foundry-setup-XXXXXX")"
    trap 'rm -rf "$BUILD_STAGE"' EXIT
    export GOCACHE="$BUILD_STAGE/go-cache"
    CGO_ENABLED=1 go build -o "$OUTPUT" -ldflags="-linkmode=external -extldflags=-mmacosx-version-min=$MACOSX_DEPLOYMENT_TARGET"
    buildInfo="$(xcrun vtool -show-build "$OUTPUT")"
    printf '%s\n' "$buildInfo"
    minos="$(printf '%s\n' "$buildInfo" | awk '$1 == "minos" {print $2}')"
    if [ "$minos" != "$MACOSX_DEPLOYMENT_TARGET" ] && [ "$minos" != "$MACOSX_DEPLOYMENT_TARGET.0" ]; then
        echo "ERROR: Binary minos is '$minos', expected $MACOSX_DEPLOYMENT_TARGET" >&2
        exit 1
    fi
else
    go build -o "$OUTPUT"
fi

if [ $? -eq 0 ]; then
    echo ""
    echo "========================================"
    echo "   Setup Complete!"
    echo "========================================"
    echo ""
    echo "To launch MBII Foundry:"
    echo ""
    echo "  ./run_mbii-foundry.sh"
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

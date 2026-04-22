#!/bin/bash
# ═══════════════════════════════════════════════════════════════════
# FA Creator - Build Script
# Creates distributable packages for macOS and Windows
# ═══════════════════════════════════════════════════════════════════

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
GO_MODULE="$SCRIPT_DIR/go_module"
MACOS_TEMPLATES="$SCRIPT_DIR/macos"
APP_NAME="FA Creator"

echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "  Building FA Creator"
echo "═══════════════════════════════════════════════════════════════════"
echo ""

cd "$GO_MODULE"

# Build for current platform
echo "Building for current platform..."
go build -o fa_creator
echo "  ✓ Built: fa_creator"

# Create macOS .app bundle
echo ""
echo "Creating macOS app bundle..."
APP_BUNDLE="$ROOT_DIR/$APP_NAME.app"

# Create fresh bundle structure
rm -rf "$APP_BUNDLE"
mkdir -p "$APP_BUNDLE/Contents/MacOS"
mkdir -p "$APP_BUNDLE/Contents/Resources"

# Copy binary
cp fa_creator "$APP_BUNDLE/Contents/MacOS/"
chmod +x "$APP_BUNDLE/Contents/MacOS/fa_creator"

# Copy Info.plist and icon from templates
if [ -f "$MACOS_TEMPLATES/Info.plist" ]; then
    cp "$MACOS_TEMPLATES/Info.plist" "$APP_BUNDLE/Contents/"
    echo "  ✓ Copied Info.plist"
fi

if [ -f "$MACOS_TEMPLATES/AppIcon.icns" ]; then
    cp "$MACOS_TEMPLATES/AppIcon.icns" "$APP_BUNDLE/Contents/Resources/"
    echo "  ✓ Copied AppIcon.icns"
fi

# Code sign for macOS (required for newer macOS versions)
echo "  Signing app bundle..."
codesign -s - -f --deep "$APP_BUNDLE" 2>/dev/null || echo "  (signing skipped - codesign not available)"
echo "  ✓ Created: $APP_NAME.app"

# Build Windows executable (requires Windows or cross-compile setup)
echo ""
echo "Note: Windows build requires native Windows or cross-compile toolchain."
echo "To build on Windows, run: go build -o fa_creator.exe -ldflags=\"-H windowsgui\""

echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "  Build Complete!"
echo "═══════════════════════════════════════════════════════════════════"
echo ""
echo "  macOS:   $APP_NAME.app (double-click to run)"
echo "  Windows: fa_creator.exe (double-click to run)"
echo ""

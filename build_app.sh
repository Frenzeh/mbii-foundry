#!/bin/bash
# ═══════════════════════════════════════════════════════════════════
# MBII Foundry — Build Script
# Produces a distributable binary + macOS .app bundle.
# ═══════════════════════════════════════════════════════════════════

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
GO_MODULE="$SCRIPT_DIR/go_module"
MACOS_TEMPLATES="$SCRIPT_DIR/macos"
BIN_NAME="mbii-foundry"
APP_NAME="MBII Foundry"

echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "  Building $APP_NAME"
echo "═══════════════════════════════════════════════════════════════════"
echo ""

cd "$GO_MODULE"

# Build for current platform
echo "Building for current platform..."
go build -o "$BIN_NAME"
echo "  ✓ Built: $BIN_NAME"

# Create macOS .app bundle
echo ""
echo "Creating macOS app bundle..."
APP_BUNDLE="$ROOT_DIR/$APP_NAME.app"

# Create fresh bundle structure
rm -rf "$APP_BUNDLE"
mkdir -p "$APP_BUNDLE/Contents/MacOS"
mkdir -p "$APP_BUNDLE/Contents/Resources"

# Copy binary
cp "$BIN_NAME" "$APP_BUNDLE/Contents/MacOS/"
chmod +x "$APP_BUNDLE/Contents/MacOS/$BIN_NAME"

# Copy Info.plist and icon from templates
if [ -f "$MACOS_TEMPLATES/Info.plist" ]; then
    cp "$MACOS_TEMPLATES/Info.plist" "$APP_BUNDLE/Contents/"
    echo "  ✓ Copied Info.plist"
fi

if [ -f "$MACOS_TEMPLATES/AppIcon.icns" ]; then
    cp "$MACOS_TEMPLATES/AppIcon.icns" "$APP_BUNDLE/Contents/Resources/"
    echo "  ✓ Copied AppIcon.icns"
fi

# Copy the runtime data the app expects next to it: enum metadata,
# per-enum markdown definitions, JSON schemas, starter templates.
# Without these the info panel falls back to placeholder descriptions
# ("Beskar attribute.") because LoadExternalData can't find the files.
for rsrc in data definitions schemas templates; do
    if [ -d "$SCRIPT_DIR/$rsrc" ]; then
        cp -r "$SCRIPT_DIR/$rsrc" "$APP_BUNDLE/Contents/Resources/"
        echo "  ✓ Copied $rsrc/"
    fi
done

# Code sign for macOS (required for newer macOS versions)
echo "  Signing app bundle..."
codesign -s - -f --deep "$APP_BUNDLE" 2>/dev/null || echo "  (signing skipped - codesign not available)"
echo "  ✓ Created: $APP_NAME.app"

# Build Windows executable (requires Windows or cross-compile setup)
echo ""
echo "Note: Windows build requires native Windows or cross-compile toolchain."
echo "To build on Windows, run: go build -o $BIN_NAME.exe -ldflags=\"-H windowsgui\""

echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "  Build Complete!"
echo "═══════════════════════════════════════════════════════════════════"
echo ""
echo "  macOS:   $APP_NAME.app (double-click to run)"
echo "  Windows: $BIN_NAME.exe (double-click to run)"
echo ""

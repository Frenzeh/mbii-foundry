#!/bin/bash
# ═══════════════════════════════════════════════════════════════════
# MBII Foundry — Build Script
# Produces a distributable binary + macOS .app bundle.
#
# Staging policy: the bundle is assembled in an exclusive mktemp -d
# directory on the DESTINATION filesystem (same device → the final
# publish is an atomic rename). There is no fixed staging path, so
# nothing is ever rm -rf'd ahead of time and concurrent builds cannot
# collide. An EXIT trap removes the staging dir, and a failed publish
# (old bundle already moved aside) is rolled back from the backup by
# the same trap.
#
# Signing policy (fails closed): ad-hoc signature by default, which
# proves structural integrity only — no Apple Developer ID trust, no
# notarization. Set FOUNDRY_CODESIGN_IDENTITY to a real identity to
# sign with Developer ID. The script never claims either.
# ═══════════════════════════════════════════════════════════════════

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
GO_MODULE="$SCRIPT_DIR/go_module"
MACOS_TEMPLATES="$SCRIPT_DIR/macos"
BIN_NAME="mbii-foundry"
APP_NAME="MBII Foundry"
DEST_DIR="${DEST_DIR:-$SCRIPT_DIR/dist}"

echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "  Building $APP_NAME"
echo "═══════════════════════════════════════════════════════════════════"
echo ""

cd "$GO_MODULE"

BUILD_STAGE="$(mktemp -d "${TMPDIR:-/tmp}/foundry-build-binaries-XXXXXX")"
BUILD_BINARY="$BUILD_STAGE/$BIN_NAME"
trap 'rm -rf "$BUILD_STAGE"' EXIT

# Build for current platform (default fallback)
echo "Building..."
if [[ "$OSTYPE" == "darwin"* ]]; then
    echo "  Building universal macOS binary..."
    CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 CC="clang -arch x86_64" CXX="clang++ -arch x86_64" go build -o "$BUILD_STAGE/${BIN_NAME}_amd64"
    CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 CC="clang -arch arm64" CXX="clang++ -arch arm64" go build -o "$BUILD_STAGE/${BIN_NAME}_arm64"
    if [ "$(lipo -archs "$BUILD_STAGE/${BIN_NAME}_amd64")" != "x86_64" ]; then
        echo "  ! amd64 build does not contain exactly x86_64" >&2
        exit 1
    fi
    if [ "$(lipo -archs "$BUILD_STAGE/${BIN_NAME}_arm64")" != "arm64" ]; then
        echo "  ! arm64 build does not contain exactly arm64" >&2
        exit 1
    fi
    lipo -create -output "$BUILD_BINARY" "$BUILD_STAGE/${BIN_NAME}_amd64" "$BUILD_STAGE/${BIN_NAME}_arm64"
    lipo "$BUILD_BINARY" -verify_arch x86_64 arm64
    buildArchs="$(lipo -archs "$BUILD_BINARY")"
    if [ "$(wc -w <<<"$buildArchs" | tr -d ' ')" != "2" ]; then
        echo "  ! Universal binary has unexpected architectures: $buildArchs" >&2
        exit 1
    fi
    echo "  ✓ Built: $BIN_NAME (universal)"
else
    go build -o "$BUILD_BINARY"
    echo "  ✓ Built: $BIN_NAME"
fi

# ── Staging + publish setup ─────────────────────────────────────────
mkdir -p "$DEST_DIR"

# Exclusive staging directory on the DESTINATION filesystem: mktemp
# guarantees a fresh, non-guessable name (no rm -rf of a fixed path,
# no PID-suffixed collisions), and same-device means the final
# publish mv is atomic.
STAGE_DIR="$(mktemp -d "$DEST_DIR/.foundry-build-XXXXXX")"
APP_BUNDLE="$DEST_DIR/$APP_NAME.app"
STAGING_BUNDLE="$STAGE_DIR/$APP_NAME.app"
BACKUP_BUNDLE=""

# EXIT trap: restore the previous bundle if the publish failed, then
# remove the staging dir. Failures are never silent: if the restore
# itself fails, the staging dir is KEPT — it holds the only copy of
# the previous bundle — and its path is printed before exiting non-zero.
cleanup_and_restore() {
    local status=$?
    trap - EXIT
    if [ -n "$BACKUP_BUNDLE" ] && [ ! -d "$APP_BUNDLE" ]; then
        echo "  ! Publishing failed — restoring previous bundle"
        if mv "$BACKUP_BUNDLE" "$APP_BUNDLE"; then
            BACKUP_BUNDLE=""
            rm -rf "$STAGE_DIR"
        else
            echo "  !! RESTORE FAILED — previous bundle preserved at: $BACKUP_BUNDLE" >&2
            echo "  !! (staging dir kept for recovery: $STAGE_DIR)" >&2
            rm -rf "$BUILD_STAGE"
            exit 1
        fi
    else
        rm -rf "$STAGE_DIR"
    fi
    rm -rf "$BUILD_STAGE"
    exit "$status"
}
trap cleanup_and_restore EXIT
echo ""
echo "Creating macOS app bundle (staging: $STAGE_DIR)..."

# Create fresh bundle structure
mkdir -p "$STAGING_BUNDLE/Contents/MacOS"
mkdir -p "$STAGING_BUNDLE/Contents/Resources"

# Copy binary
cp "$BUILD_BINARY" "$STAGING_BUNDLE/Contents/MacOS/$BIN_NAME"
chmod +x "$STAGING_BUNDLE/Contents/MacOS/$BIN_NAME"

# Copy Info.plist and icon from templates
if [ -f "$MACOS_TEMPLATES/Info.plist" ]; then
    cp "$MACOS_TEMPLATES/Info.plist" "$STAGING_BUNDLE/Contents/"
    echo "  ✓ Copied Info.plist"
else
    echo "  ! Missing Info.plist"
    exit 1
fi

if [ -f "$MACOS_TEMPLATES/AppIcon.icns" ]; then
    cp "$MACOS_TEMPLATES/AppIcon.icns" "$STAGING_BUNDLE/Contents/Resources/"
    echo "  ✓ Copied AppIcon.icns"
fi

for rsrc in data definitions schemas templates; do
    if [ -d "$SCRIPT_DIR/$rsrc" ]; then
        cp -r "$SCRIPT_DIR/$rsrc" "$STAGING_BUNDLE/Contents/Resources/"
        echo "  ✓ Copied $rsrc/"
    else
        echo "  ! Missing resource dir: $rsrc"
        exit 1
    fi
done

if [ -d "$SCRIPT_DIR/private" ]; then
    cp -r "$SCRIPT_DIR/private" "$STAGING_BUNDLE/Contents/Resources/"
    echo "  ✓ Copied private/ (dev overlay — local-only)"
fi

# Validate architecture
echo "  Validating architecture..."
lipo "$STAGING_BUNDLE/Contents/MacOS/$BIN_NAME" -verify_arch x86_64 arm64
lipoOut="$(lipo -archs "$STAGING_BUNDLE/Contents/MacOS/$BIN_NAME")"
if [ "$(wc -w <<<"$lipoOut" | tr -d ' ')" != "2" ]; then
    echo "  ! Binary is not an exact x86_64+arm64 universal binary: $lipoOut"
    exit 1
fi

# Code sign for macOS (required for newer macOS versions).
# Default is the ad-hoc identity ("-"): structural integrity only,
# no Apple Developer ID trust and no notarization. Set
# FOUNDRY_CODESIGN_IDENTITY to a Developer ID for real trust.
echo "  Signing app bundle..."
if ! command -v codesign >/dev/null 2>&1; then
    echo "  ! codesign not available, failing closed"
    exit 1
fi
SIGN_IDENTITY="${FOUNDRY_CODESIGN_IDENTITY:--}"
if [ "$SIGN_IDENTITY" = "-" ]; then
    echo "  ! Signing AD-HOC (FOUNDRY_CODESIGN_IDENTITY unset) — no Apple Developer ID trust, not notarized"
fi
codesign -s "$SIGN_IDENTITY" -f --deep --strict "$STAGING_BUNDLE"
echo "  ✓ Signed: $APP_NAME.app"

# Validate signature
echo "  Validating signature..."
codesign --verify --deep --strict "$STAGING_BUNDLE"

# Atomically publish: move the old bundle into staging (same device,
# atomic), then rename the new bundle into place. If the second mv
# fails, the EXIT trap restores the backup; on success the trap drops
# staging (including the superseded bundle).
echo "  Publishing..."
if [ -d "$APP_BUNDLE" ]; then
    BACKUP_BUNDLE="$STAGE_DIR/previous-$APP_NAME.app"
    mv "$APP_BUNDLE" "$BACKUP_BUNDLE"
fi
mv "$STAGING_BUNDLE" "$APP_BUNDLE"
BACKUP_BUNDLE=""   # published OK — trap must not "restore" over it

echo "  ✓ Created: $APP_NAME.app in $DEST_DIR"

# Build Windows executable (requires Windows or cross-compile setup)
echo ""
echo "Note: Windows build requires native Windows or cross-compile toolchain."
echo "To build on Windows, run: go build -o $BIN_NAME.exe -ldflags=\"-H windowsgui\""

echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "  Build Complete!"
echo "═══════════════════════════════════════════════════════════════════"
echo ""
echo "  macOS:   $APP_BUNDLE (double-click to run)"
echo "  Windows: $BIN_NAME.exe (double-click to run)"
echo ""

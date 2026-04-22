#!/bin/bash
# =============================================================================
# MBII Foundry — Launcher
# =============================================================================
# Launch the MBII Foundry GUI application.
# Run ./setup_mbii-foundry.sh first if you haven't built it yet.
# =============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GO_MODULE_DIR="$SCRIPT_DIR/go_module"

# Detect platform and set executable name
if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" || "$OSTYPE" == "cygwin" ]]; then
    EXECUTABLE="mbii-foundry.exe"
else
    EXECUTABLE="mbii-foundry"
fi

FULL_PATH="$GO_MODULE_DIR/$EXECUTABLE"

# Check if built
if [ ! -f "$FULL_PATH" ]; then
    echo ""
    echo "MBII Foundry has not been built yet."
    echo ""
    echo "Run setup first:"
    echo "  ./setup_mbii-foundry.sh"
    echo ""
    exit 1
fi

# Launch the application
echo "Launching MBII Foundry..."
cd "$GO_MODULE_DIR"
./"$EXECUTABLE"

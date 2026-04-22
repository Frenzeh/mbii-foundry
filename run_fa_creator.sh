#!/bin/bash
# =============================================================================
# FA Creator Launcher
# =============================================================================
# Launch the FA Creator GUI application.
# Run ./setup_fa_creator.sh first if you haven't built it yet.
# =============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GO_MODULE_DIR="$SCRIPT_DIR/go_module"

# Detect platform and set executable name
if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" || "$OSTYPE" == "cygwin" ]]; then
    EXECUTABLE="fa_creator.exe"
else
    EXECUTABLE="fa_creator"
fi

FULL_PATH="$GO_MODULE_DIR/$EXECUTABLE"

# Check if built
if [ ! -f "$FULL_PATH" ]; then
    echo ""
    echo "FA Creator has not been built yet."
    echo ""
    echo "Run setup first:"
    echo "  ./setup_fa_creator.sh"
    echo ""
    exit 1
fi

# Launch the application
echo "Launching FA Creator..."
cd "$GO_MODULE_DIR"
./"$EXECUTABLE"

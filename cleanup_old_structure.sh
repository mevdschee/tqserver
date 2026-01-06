#!/bin/bash
set -e

echo "Cleaning up old directory structure..."

# Remove old binaries directory (old structure)
if [ -d "bin" ]; then
    echo "Removing old bin/ directory..."
    rm -rf bin/
fi

# Remove old cmd directory (moved to server/src)
if [ -d "cmd" ]; then
    echo "Removing old cmd/ directory..."
    rm -rf cmd/
fi

# Remove old internal directory (moved to server/src)
if [ -d "internal" ]; then
    echo "Removing old internal/ directory..."
    rm -rf internal/
fi

# Remove old pages directory (moved to workers/)
if [ -d "pages" ]; then
    echo "Removing old pages/ directory..."
    rm -rf pages/
fi

# Remove old templates directory if not needed
if [ -d "templates" ]; then
    echo "Checking templates/ directory..."
    if [ -z "$(ls -A templates/)" ]; then
        echo "Removing empty templates/ directory..."
        rm -rf templates/
    else
        echo "Note: templates/ is not empty, keeping it for now"
    fi
fi

echo "✅ Cleanup complete!"
echo ""
echo "New structure:"
echo "  - server/src/      (server source code)"
echo "  - server/bin/      (compiled server binary)"
echo "  - workers/*/src/   (worker source code)"
echo "  - workers/*/bin/   (compiled worker binaries)"
echo "  - pkg/             (shared packages)"

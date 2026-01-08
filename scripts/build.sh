#!/bin/bash
set -e

WORKERS_DIR="workers"
SERVER_DIR="server"

# Default to development mode
MODE="${1:-dev}"

# Validate mode
if [ "$MODE" != "dev" ] && [ "$MODE" != "prod" ]; then
    echo "Usage: $0 [dev|prod]"
    echo "  dev  - Development build (default, with timestamp checking)"
    echo "  prod - Production build (optimized, no timestamp checking)"
    exit 1
fi

# Set build flags based on mode
if [ "$MODE" = "prod" ]; then
    BUILD_FLAGS="-ldflags=-s -w"
    CHECK_TIMESTAMPS=false
    echo "Building for production..."
else
    BUILD_FLAGS=""
    CHECK_TIMESTAMPS=true
    echo "Building for development..."
fi

# Build server
echo "Building server..."
mkdir -p "$SERVER_DIR/bin"

server_output="$SERVER_DIR/bin/tqserver"
needs_server_rebuild=false

if [ "$CHECK_TIMESTAMPS" = true ]; then
    if [ ! -f "$server_output" ]; then
        needs_server_rebuild=true
    else
        # Check if any source files in src or pkg are newer than binary use find recursively
        for src_file in $(find "$SERVER_DIR/src" "$SERVER_DIR/../pkg" -name "*.go"); do
            if [ "$src_file" -nt "$server_output" ]; then
                needs_server_rebuild=true
                break
            fi
        done
    fi
    
    if [ "$needs_server_rebuild" = false ]; then
        echo "✓ Server up to date"
    else
        go build $BUILD_FLAGS -o "$server_output" $SERVER_DIR/src/*.go
        echo "✓ Built server"
    fi
else
    go build $BUILD_FLAGS -o "$server_output" $SERVER_DIR/src/*.go
    echo "✓ Built server"
fi

echo ""
echo "Building workers..."

for worker_dir in $WORKERS_DIR/*/; do
    worker_name=$(basename "$worker_dir")
    
    src_dir="$worker_dir/src"
    if [ ! -f "$src_dir/main.go" ]; then
        echo "Skipping $worker_name (no main.go)"
        continue
    fi
    
    output_dir="$worker_dir/bin"
    output_file="$output_dir/$worker_name"
    
    # Check if rebuild needed (only in dev mode)
    needs_rebuild=false
    if [ "$CHECK_TIMESTAMPS" = true ]; then
        if [ ! -f "$output_file" ]; then
            needs_rebuild=true
        else
            # Check if any source files are newer than binary
            for src_file in "$src_dir"/*.go; do
                if [ "$src_file" -nt "$output_file" ]; then
                    needs_rebuild=true
                    break
                fi
            done
        fi
        
        if [ "$needs_rebuild" = false ]; then
            echo "✓ $worker_name up to date"
            continue
        fi
    fi
    
    echo "Building $worker_name..."
    
    mkdir -p "$output_dir"
    
    # Build with mode-specific flags
    go build $BUILD_FLAGS -o "$output_file" "$src_dir"/*.go
    
    echo "✓ Built $worker_name"
done

echo ""
if [ "$MODE" = "prod" ]; then
    echo "✅ Production build complete"
else
    echo "✅ Development build complete"
fi
echo "Server built to: server/bin/tqserver"
echo "Workers built to: workers/*/bin/"

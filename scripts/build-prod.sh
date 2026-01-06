#!/bin/bash
set -e

WORKERS_DIR="workers"
SERVER_DIR="server"

echo "Building for production..."

# Build server
echo "Building server..."
mkdir -p "$SERVER_DIR/bin"
go build -ldflags="-s -w" -o "$SERVER_DIR/bin/tqserver" $SERVER_DIR/src/*.go
echo "✓ Built server"

# Build workers
for worker_dir in $WORKERS_DIR/*/; do
    worker_name=$(basename "$worker_dir")
    
    src_dir="$worker_dir/src"
    if [ ! -f "$src_dir/main.go" ]; then
        continue
    fi
    
    echo "Building worker: $worker_name..."
    
    mkdir -p "$worker_dir/bin"
    
    # Build binary with optimizations
    go build -ldflags="-s -w" -o "$worker_dir/bin/$worker_name" "$src_dir"/*.go
    
    echo "✓ Built $worker_name"
done

echo ""
echo "✅ Production build complete"
echo "Workers built to: workers/*/bin/"
echo "Server built to: server/bin/tqserver"

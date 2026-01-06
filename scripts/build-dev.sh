#!/bin/bash
set -e

WORKERS_DIR="workers"

echo "Building workers for development..."

for worker_dir in $WORKERS_DIR/*/; do
    worker_name=$(basename "$worker_dir")
    
    src_dir="$worker_dir/src"
    if [ ! -f "$src_dir/main.go" ]; then
        echo "Skipping $worker_name (no main.go)"
        continue
    fi
    
    output_dir="$worker_dir/bin"
    output_file="$output_dir/$worker_name"
    
    # Check if rebuild needed (compare timestamps)
    needs_rebuild=false
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
    
    echo "Building $worker_name..."
    
    mkdir -p "$output_dir"
    
    # Build (no embed needed)
    go build -o "$output_file" "$src_dir"/*.go
    
    echo "✓ Built $worker_name"
done

echo "✅ Development build complete"

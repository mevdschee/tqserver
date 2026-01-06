#!/bin/bash
# Pre-deployment hook script
# This script runs before deployment begins

set -e

TARGET="${1:-unknown}"
SERVER_BIN="${2}"
WORKERS="${3}"

echo "Running pre-deployment checks for target: $TARGET"

# Check if binaries exist locally
if [ -z "$SERVER_BIN" ]; then
    echo "Error: Server binary path not provided"
    exit 1
fi

if [ ! -f "$SERVER_BIN" ]; then
    echo "Error: Server binary not found at $SERVER_BIN"
    exit 1
fi

# Check worker binaries
if [ -n "$WORKERS" ]; then
    for worker in $WORKERS; do
        WORKER_BIN="workers/${worker}/bin/tqworker_${worker}"
        if [ ! -f "$WORKER_BIN" ]; then
            echo "Error: Worker binary not found at $WORKER_BIN"
            exit 1
        fi
    done
fi

# Verify configuration file exists
if [ ! -f "config/server.yaml" ]; then
    echo "Warning: config/server.yaml not found"
fi

# Run tests (optional)
# echo "Running tests..."
# go test ./... || exit 1

# Create backup timestamp
date +%Y%m%d_%H%M%S > /tmp/tqserver_deploy_timestamp

echo "✓ Pre-deployment checks passed"
exit 0

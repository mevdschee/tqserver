#!/bin/bash
# Build and run TQServer
# Use --mode dev for development with file watching
# Use --mode prod for production with SIGHUP reload

MODE="${1:-dev}"

# Build server
echo "Building server..."
./scripts/build.sh

# Run server
echo "Starting server..."
./server/bin/tqserver

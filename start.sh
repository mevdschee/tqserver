#!/bin/bash
# Build and run TQServer
# Use --mode dev for development with file watching
# Use --mode prod for production with SIGHUP reload

MODE="${1:-dev}"

# Build server
echo "Building server..."
if [ "$MODE" = "prod" ]; then
    ./scripts/build-prod.sh
else
    ./scripts/build-dev.sh
fi

# Run server
echo "Starting server..."
./server/bin/tqserver

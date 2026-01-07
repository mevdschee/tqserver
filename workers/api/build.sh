#!/bin/bash
# Build script for Kotlin API worker

set -e

echo "Building Kotlin API Worker..."
echo "================================"

cd "$(dirname "$0")"

# Check for Java
if ! command -v java &> /dev/null; then
    echo "Error: Java is not installed or not in PATH"
    echo "Please install Java 17 or higher"
    exit 1
fi

echo "Java version:"
java -version

# Check for Gradle wrapper
if [ ! -f "./gradlew" ]; then
    echo "Error: Gradle wrapper not found"
    echo "Please run: gradle wrapper"
    exit 1
fi

# Make gradlew executable
chmod +x gradlew

# Build the project
echo ""
echo "Running Gradle build..."
./gradlew clean build

# Make the wrapper script executable
chmod +x bin/api

echo ""
echo "================================"
echo "Build complete!"
echo "JAR location: build/libs/api.jar"
echo "Wrapper script: bin/api"
echo ""
echo "To run manually:"
echo "  export WORKER_PORT=9000"
echo "  export WORKER_ROUTE=/api"
echo "  export WORKER_MODE=dev"
echo "  ./bin/api"
echo ""
echo "Or let TQServer manage it automatically."

# Building Workers

- [Introduction](#introduction)
- [Build System](#build-system)
- [Build Configuration](#build-configuration)
- [Build Process](#build-process)
- [Build Optimization](#build-optimization)
- [Dependencies](#dependencies)
- [Cross-Compilation](#cross-compilation)
- [Build Scripts](#build-scripts)
- [Troubleshooting](#troubleshooting)

## Introduction

TQServer provides a flexible build system for compiling workers. The build system supports Go compilation, dependency management, and optimization for both development and production environments.

## Build System

### Build Workflow

```
Source Code → Dependencies → Compilation → Binary → Deployment
     ↓             ↓              ↓           ↓          ↓
  *.go files   go.mod/sum    go build    executable   Running
```

### Automatic Building

TQServer automatically builds workers when:

1. **Server Starts**: Builds all workers on startup
2. **File Changes**: Hot reload triggers rebuild (dev mode)
3. **Manual Request**: Explicit build command

```bash
# Build all workers
./bin/tqserver build

# Build specific worker
./bin/tqserver build api

# Build with verbose output
./bin/tqserver build --verbose api
```

## Build Configuration

### Worker Build Config

```yaml
# workers/api/config.yaml

build:
  # Build command (optional, defaults to go build)
  command: "go build -o bin/api src/*.go"
  
  # Build timeout
  timeout: 60s
  
  # Working directory (defaults to worker directory)
  dir: "."
  
  # Environment variables for build
  env:
    CGO_ENABLED: "0"
    GOOS: "linux"
    GOARCH: "amd64"
  
  # Build flags
  flags:
    - "-ldflags=-s -w"        # Strip debug info
    - "-trimpath"             # Remove file paths
    - "-buildmode=exe"
  
  # Source patterns to watch
  sources:
    - "src/**/*.go"
    - "go.mod"
    - "go.sum"
  
  # Files to exclude
  exclude:
    - "**/*_test.go"
    - "**/testdata/**"
```

### Global Build Configuration

```yaml
# config/server.yaml

build:
  # Default build settings for all workers
  defaults:
    timeout: 60s
    parallel: true            # Build workers in parallel
    max_parallel: 4           # Max concurrent builds
  
  # Cache settings
  cache:
    enabled: true
    dir: ".cache/build"
  
  # Development mode settings
  development:
    optimize: false           # Fast builds
    include_debug: true       # Include debug symbols
  
  # Production mode settings
  production:
    optimize: true            # Optimized builds
    include_debug: false      # Strip debug symbols
    static: true              # Static linking
```

## Build Process

### Basic Build

```bash
#!/bin/bash
# Simple build script

WORKER_NAME="api"
SRC_DIR="workers/$WORKER_NAME/src"
OUT_DIR="workers/$WORKER_NAME/bin"
BINARY="$OUT_DIR/$WORKER_NAME"

# Create output directory
mkdir -p "$OUT_DIR"

# Build
go build -o "$BINARY" "$SRC_DIR"/*.go

# Check result
if [ $? -eq 0 ]; then
    echo "✓ Build successful: $BINARY"
    exit 0
else
    echo "✗ Build failed"
    exit 1
fi
```

### Development Build

Fast compilation for development:

```bash
#!/bin/bash
# Development build (fast)

go build \
    -o workers/api/bin/api \
    workers/api/src/*.go

# ~200-500ms for typical worker
```

### Production Build

Optimized compilation for production:

```bash
#!/bin/bash
# Production build (optimized)

go build \
    -ldflags="-s -w" \
    -trimpath \
    -tags production \
    -o workers/api/bin/api \
    workers/api/src/*.go

# ~500-1000ms, smaller binary
```

### Build Implementation

```go
package builder

import (
    "context"
    "fmt"
    "os/exec"
    "path/filepath"
    "time"
)

type Builder struct {
    config *BuildConfig
}

func (b *Builder) Build(ctx context.Context, workerName string) error {
    workerPath := filepath.Join("workers", workerName)
    srcPath := filepath.Join(workerPath, "src")
    binPath := filepath.Join(workerPath, "bin", workerName)
    
    // Create bin directory
    if err := os.MkdirAll(filepath.Dir(binPath), 0755); err != nil {
        return fmt.Errorf("failed to create bin dir: %w", err)
    }
    
    // Build command
    cmd := exec.CommandContext(ctx, "go", "build",
        "-o", binPath,
        srcPath,
    )
    
    // Set environment
    cmd.Env = append(os.Environ(),
        "CGO_ENABLED=0",
        "GOOS="+runtime.GOOS,
        "GOARCH="+runtime.GOARCH,
    )
    cmd.Dir = workerPath
    
    // Capture output
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("build failed: %w\n%s", err, output)
    }
    
    log.Printf("Built %s successfully", workerName)
    return nil
}
```

### Parallel Building

```go
func (b *Builder) BuildAll(workers []string) error {
    var wg sync.WaitGroup
    errChan := make(chan error, len(workers))
    
    // Limit concurrency
    sem := make(chan struct{}, b.config.MaxParallel)
    
    for _, worker := range workers {
        wg.Add(1)
        go func(name string) {
            defer wg.Done()
            
            // Acquire semaphore
            sem <- struct{}{}
            defer func() { <-sem }()
            
            // Build with timeout
            ctx, cancel := context.WithTimeout(
                context.Background(),
                b.config.Timeout,
            )
            defer cancel()
            
            if err := b.Build(ctx, name); err != nil {
                errChan <- fmt.Errorf("%s: %w", name, err)
            }
        }(worker)
    }
    
    wg.Wait()
    close(errChan)
    
    // Collect errors
    var errors []error
    for err := range errChan {
        errors = append(errors, err)
    }
    
    if len(errors) > 0 {
        return fmt.Errorf("build failures: %v", errors)
    }
    
    return nil
}
```

## Build Optimization

### Build Cache

Enable Go build cache:

```bash
# Set cache directory
export GOCACHE="$HOME/.cache/go-build"
export GOMODCACHE="$HOME/go/pkg/mod"

# Build with cache
go build -o bin/api src/*.go

# First build: ~500ms
# Subsequent builds with no changes: ~50ms
```

### Incremental Builds

Only rebuild changed files:

```go
func (b *Builder) needsRebuild(workerName string) (bool, error) {
    binaryPath := filepath.Join("workers", workerName, "bin", workerName)
    
    // Check if binary exists
    binaryInfo, err := os.Stat(binaryPath)
    if os.IsNotExist(err) {
        return true, nil // Binary doesn't exist
    }
    
    // Check source files
    srcPath := filepath.Join("workers", workerName, "src")
    sources, _ := filepath.Glob(filepath.Join(srcPath, "*.go"))
    
    for _, src := range sources {
        srcInfo, _ := os.Stat(src)
        if srcInfo.ModTime().After(binaryInfo.ModTime()) {
            return true, nil // Source newer than binary
        }
    }
    
    return false, nil // No rebuild needed
}
```

### Parallel Compilation

Utilize multiple CPU cores:

```bash
# Set number of parallel compilation jobs
export GOMAXPROCS=8

# Build with parallelization
go build -p 8 -o bin/api src/*.go
```

### Link-Time Optimization

Strip unnecessary symbols:

```bash
go build \
    -ldflags="-s -w" \
    -o bin/api src/*.go

# -s: Omit symbol table
# -w: Omit DWARF debug info
# Result: 30-50% smaller binary
```

## Dependencies

### Go Modules

```bash
# Initialize module
cd workers/api
go mod init github.com/yourorg/tqserver/workers/api

# Add dependencies
go get github.com/gorilla/mux@latest
go get github.com/lib/pq@v1.10.0

# Tidy dependencies
go mod tidy

# Vendor dependencies (optional)
go mod vendor
```

### go.mod Example

```go
// workers/api/go.mod

module github.com/yourorg/tqserver/workers/api

go 1.24

require (
    github.com/gorilla/mux v1.8.1
    github.com/lib/pq v1.10.9
    github.com/redis/go-redis/v9 v9.4.0
)

require (
    github.com/cespare/xxhash/v2 v2.2.0 // indirect
    github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
)
```

### Dependency Management

```bash
# Update all dependencies
go get -u ./...

# Update specific dependency
go get github.com/gorilla/mux@v1.8.1

# View dependency graph
go mod graph

# Verify dependencies
go mod verify

# Clean module cache
go clean -modcache
```

### Vendoring Dependencies

```bash
# Create vendor directory
go mod vendor

# Build with vendor
go build -mod=vendor -o bin/api src/*.go

# Benefits:
# - Faster builds (no network)
# - Reproducible builds
# - Offline development
```

## Cross-Compilation

### Build for Different Platforms

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o bin/api-linux-amd64 src/*.go

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o bin/api-linux-arm64 src/*.go

# macOS AMD64
GOOS=darwin GOARCH=amd64 go build -o bin/api-darwin-amd64 src/*.go

# Windows AMD64
GOOS=windows GOARCH=amd64 go build -o bin/api-windows-amd64.exe src/*.go
```

### Cross-Compilation Script

```bash
#!/bin/bash
# scripts/build-all-platforms.sh

WORKER=$1
PLATFORMS="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64"

for platform in $PLATFORMS; do
    GOOS=${platform%/*}
    GOARCH=${platform#*/}
    
    output="workers/$WORKER/bin/${WORKER}-${GOOS}-${GOARCH}"
    if [ "$GOOS" = "windows" ]; then
        output="${output}.exe"
    fi
    
    echo "Building for $GOOS/$GOARCH..."
    GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags="-s -w" \
        -o "$output" \
        "workers/$WORKER/src"/*.go
    
    if [ $? -eq 0 ]; then
        echo "✓ Built: $output"
    else
        echo "✗ Failed: $GOOS/$GOARCH"
    fi
done
```

## Build Scripts

### Development Build Script

```bash
#!/bin/bash
# scripts/build-dev.sh

set -e

WORKER=$1

if [ -z "$WORKER" ]; then
    echo "Usage: $0 <worker-name>"
    exit 1
fi

WORKER_DIR="workers/$WORKER"
SRC_DIR="$WORKER_DIR/src"
BIN_DIR="$WORKER_DIR/bin"
BINARY="$BIN_DIR/$WORKER"

echo "Building $WORKER (development)..."

# Create bin directory
mkdir -p "$BIN_DIR"

# Fast build (no optimization)
go build \
    -o "$BINARY" \
    "$SRC_DIR"/*.go

if [ $? -eq 0 ]; then
    echo "✓ Build successful: $BINARY"
    
    # Show binary info
    ls -lh "$BINARY"
    
    exit 0
else
    echo "✗ Build failed"
    exit 1
fi
```

### Production Build Script

```bash
#!/bin/bash
# scripts/build-prod.sh

set -e

WORKER=$1
VERSION=${2:-"1.0.0"}
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

if [ -z "$WORKER" ]; then
    echo "Usage: $0 <worker-name> [version]"
    exit 1
fi

WORKER_DIR="workers/$WORKER"
SRC_DIR="$WORKER_DIR/src"
BIN_DIR="$WORKER_DIR/bin"
BINARY="$BIN_DIR/$WORKER"

echo "Building $WORKER v$VERSION (production)..."

# Create bin directory
mkdir -p "$BIN_DIR"

# Build with optimization and version info
go build \
    -ldflags="-s -w -X main.Version=$VERSION -X main.BuildTime=$BUILD_TIME -X main.GitCommit=$GIT_COMMIT" \
    -trimpath \
    -tags production \
    -o "$BINARY" \
    "$SRC_DIR"/*.go

if [ $? -eq 0 ]; then
    echo "✓ Build successful: $BINARY"
    
    # Show binary info
    ls -lh "$BINARY"
    
    # Strip additional symbols (if available)
    if command -v strip &> /dev/null; then
        strip "$BINARY"
        echo "✓ Stripped binary"
    fi
    
    # Show version
    "$BINARY" --version
    
    exit 0
else
    echo "✗ Build failed"
    exit 1
fi
```

### Version Information

```go
// src/main.go

package main

import (
    "flag"
    "fmt"
)

var (
    Version   = "dev"
    BuildTime = "unknown"
    GitCommit = "unknown"
)

func main() {
    version := flag.Bool("version", false, "Show version")
    flag.Parse()
    
    if *version {
        fmt.Printf("Version: %s\n", Version)
        fmt.Printf("Build Time: %s\n", BuildTime)
        fmt.Printf("Git Commit: %s\n", GitCommit)
        return
    }
    
    // Start server...
}
```

## Troubleshooting

### Build Failures

**Problem**: Build fails with "package not found"

```bash
# Solution: Tidy dependencies
cd workers/api
go mod tidy
go build -o bin/api src/*.go
```

**Problem**: Build timeout

```yaml
# Solution: Increase timeout
build:
  timeout: 120s  # Increase from default 60s
```

**Problem**: Out of memory during build

```bash
# Solution: Limit parallel builds
export GOMAXPROCS=2
go build -p 2 -o bin/api src/*.go
```

### Dependency Issues

**Problem**: Dependency version conflict

```bash
# View dependency graph
go mod graph | grep conflicting-package

# Update specific dependency
go get github.com/package@v1.2.3

# Re-tidy
go mod tidy
```

**Problem**: Cannot download dependencies

```bash
# Set proxy
export GOPROXY=https://proxy.golang.org,direct

# Or use direct
export GOPROXY=direct

# Retry build
go build -o bin/api src/*.go
```

### Performance Issues

**Problem**: Slow builds

```bash
# Enable build cache
export GOCACHE="$HOME/.cache/go-build"

# Use vendoring
go mod vendor
go build -mod=vendor -o bin/api src/*.go

# Increase parallelization
go build -p 8 -o bin/api src/*.go
```

## Best Practices

### Separate Build Stages

```dockerfile
# Dockerfile with multi-stage build

# Stage 1: Build
FROM golang:1.24 AS builder
WORKDIR /build
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/api workers/api/src/*.go

# Stage 2: Runtime
FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/api /usr/local/bin/api
ENTRYPOINT ["api"]
```

### Reproducible Builds

```bash
# Pin dependency versions
go get github.com/package@v1.2.3

# Commit go.sum
git add go.sum
git commit -m "Pin dependencies"

# Use specific Go version
go mod edit -go=1.24
```

### Build Validation

```bash
#!/bin/bash
# Validate build

# Build
go build -o bin/api src/*.go

# Run basic tests
./bin/api --version
./bin/api --help

# Check binary
file bin/api
ldd bin/api  # Check dependencies
```

## Next Steps

- [Testing Workers](testing.md) - Test your builds
- [Worker Configuration](configuration.md) - Configure build settings
- [Deployment](../getting-started/deployment.md) - Deploy built workers
- [Hot Reload](../architecture/hot-reload.md) - Understand rebuild process

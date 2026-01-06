# Deployment Organization Specification

## Overview

This specification defines how to organize binaries, web root files (static assets), and view files (HTML templates) in a way that supports:
- **Production deployments** with MD5-based change detection and embedded resources
- **Development mode** with file watchers for hot reload and external files
- **Zero-downtime upgrades** by starting workers on new ports when binaries change
- **Atomic deployments** using manifests in production mode
- **Embedded resources** in production binaries for single-file deployment

## Key Principles

1. **Dev vs Prod Separation**: Development uses file watchers; production uses manifest-based detection
2. **No Symlinks**: Use direct file references and manifest-based tracking
3. **Structured Folders**: Each worker and server has `src/`, `bin/`, `public/`, `private/` organization
4. **Git Commit in Folder Names**: Deployment folders named with git commit hash for atomic, versioned, traceable deployments
5. **Assets Alongside Binaries**: Production deployments include binaries + assets in versioned folders (no embedding)

## Current State

### Current Structure
```
tqserver/
├── bin/                     # Compiled worker binaries (flat structure)
│   ├── tqserver
│   ├── tqworker_index
│   └── tqworker_api_users
├── pages/                   # Worker source code
│   ├── index/
│   │   ├── main.go         # Worker entry point
│   │   ├── index.html      # View files (per-worker)
│   │   └── hello.html
│   └── api/
│       └── users/
│           └── main.go
├── templates/               # Shared templates
│   └── base.html
└── config/
    └── server.yaml
```

### Current Behavior
- **Build naming**: Flat structure with route-based names
- **Change detection**: File watcher on `.go` files triggers rebuild
- **Deployment**: All changes happen in-place with file watching
- **No distinction** between dev and prod modes

## Proposed Structure

### Directory Layout

```
tqserver/
├── server/                        # Main server
│   ├── src/                       # Server source code
│   │   ├── main.go
│   │   ├── config.go
│   │   ├── proxy.go
│   │   ├── router.go
│   │   └── supervisor.go
│   ├── bin/
│   │   └── tqserver               # Server binary
│   ├── public/                    # Server public assets
│   │   └── admin/                 # Admin UI files
│   │       ├── index.html
│   │       └── dashboard.html
│   └── private/                   # Server private resources
│       └── config.yaml.example
│
├── workers/                       # All workers organized consistently
│   ├── index/                     # Root route worker
│   │   ├── src/                   # Worker source code
│   │   │   ├── main.go           # Entry point
│   │   │   └── handlers.go       # Request handlers
│   │   ├── bin/
│   │   │   └── index              # Worker binary
│   │   ├── public/                # Public assets (served to browsers)
│   │   │   ├── css/
│   │   │   │   └── styles.css
│   │   │   ├── js/
│   │   │   │   └── app.js
│   │   │   └── images/
│   │   │       └── logo.png
│   │   └── private/               # Private resources (templates, configs)
│   │       ├── views/
│   │       │   ├── index.html
│   │       │   └── hello.html
│   │       └── templates/
│   │           └── base.html
│   │
│   ├── api_users/                 # API route worker
│   │   ├── src/
│   │   │   └── main.go
│   │   ├── bin/
│   │   │   └── api_users          # Worker binary
│   │   ├── public/
│   │   │   └── swagger/          # API docs
│   │   │       └── index.html
│   │   └── private/
│   │       └── config/
│   │           └── validation.yaml
│   │
│   └── shared/                    # Shared resources across workers
│       ├── public/                # Shared public assets
│       │   ├── css/
│       │   │   └── common.css
│       │   └── js/
│       │       └── utils.js
│       └── private/               # Shared templates
│           └── templates/
│               ├── base.html
│               └── error.html
│
├── deploy/                        # Production deployment directory
│   ├── manifest.json              # Points to current deployment
│   ├── a1b2c3d4e5f67890.../       # Deployment folder (git commit hash)
│   │   ├── server/
│   │   │   ├── bin/
│   │   │   │   └── tqserver       # Server binary
│   │   │   ├── public/            # Server public assets
│   │   │   │   └── admin/
│   │   │   └── private/           # Server private resources
│   │   │       └── config.yaml
│   │   └── workers/
│   │       ├── index/             # Worker deployment
│   │       │   ├── bin/
│   │       │   │   └── index      # Worker binary
│   │       │   ├── public/        # Worker public assets
│   │       │   │   ├── css/
│   │       │   │   └── js/
│   │       │   └── private/       # Worker private resources
│   │       │       └── views/
│   │       └── api_users/
│   │           ├── bin/
│   │           │   └── api_users
│   │           ├── public/
│   │           └── private/
│   └── b2c3d4e5f67890.../         # Previous deployment (kept for rollback)
│       └── (...same structure...)
│
├── config/
│   ├── server.yaml                # Server config (dev and prod)
│   └── deployment.yaml            # Deployment settings
│
└── scripts/                       # Build and deployment scripts
    ├── build-dev.sh
    ├── build-prod.sh
    └── deploy.sh
```

### Folder Structure Convention

Each worker and the server follow this structure:

#### `src/` - Source Code
- Go source files
- Entry point: `main.go`
- No embed directives needed (resources always external)

#### `bin/` - Compiled Binary
- Single binary built from `src/`
- Same binary used in dev and prod modes
- Mode determined by runtime environment variables

#### `public/` - Public Assets
- Static files served directly to clients (CSS, JS, images)
- **Dev mode**: Served from `workers/{name}/public/`
- **Prod mode**: Served from `deploy/{commit_hash}/workers/{name}/public/`
- URL mapping: `/static/{worker}/path/to/file.css`
- **Always** located at `{WORKER_BASE}/public/` (hardcoded)

#### `private/` - Private Resources
- Templates, config files, not directly accessible via HTTP
- **Dev mode**: Loaded from `workers/{name}/private/`
- **Prod mode**: Loaded from `deploy/{commit_hash}/workers/{name}/private/`
- **Always** located at `{WORKER_BASE}/private/` (hardcoded)
- Includes:
  - `views/`: HTML templates
  - `templates/`: Shared template components
  - `config/`: Worker-specific configuration

## MD5-Based Change Detection

### Git Commit Hash-Based Deployment

Each deployment folder is named after the git commit hash:
```bash
# Get current git commit hash (full 40-character SHA-1)
COMMIT_HASH=$(git rev-parse HEAD)
# Example: a1b2c3d4e5f67890123456789abcdef012345678

# Or short version (first 7-12 characters)
COMMIT_SHORT=$(git rev-parse --short HEAD)
# Example: a1b2c3d
```

This creates deployment folders like: `deploy/a1b2c3d4e5f67890.../`

**Benefits:**
- Traceable: Direct link to source code version
- Git integration: Easy to see what's deployed
- Reproducible: Can rebuild exact same deployment from git
- Audit trail: Git history shows all changes

### Manifest Structure

The deployment manifest (`deploy/manifest.json`) points to the current deployment folder:

```json
{
  "version": "2026-01-06T15:30:00Z",
  "deployment_id": "deploy-1736180400",
  "current_deployment": "a1b2c3d4e5f67890123456789abcdef012345678",
  "deployment_path": "deploy/a1b2c3d4e5f67890123456789abcdef012345678",
  
  "deployment": {
    "git_commit": "a1b2c3d4e5f67890123456789abcdef012345678",
    "git_commit_short": "a1b2c3d",
    "git_branch": "main",
    "git_tag": "v1.2.3",
    "built_at": "2026-01-06T15:25:00Z",
    "deployed_at": "2026-01-06T15:30:00Z",
    
    "server": {
      "path": "server",
      "binary": "bin/tqserver",
      "binary_md5": "server123456789",
      "size_bytes": 12582912,
      "has_public": true,
      "has_private": true
    },
    
    "workers": {
      "index": {
        "path": "workers/index",
        "binary": "bin/index",
        "binary_md5": "index123456789",
        "route": "/",
        "size_bytes": 8388608,
        "files": {
          "src": ["main.go", "handlers.go"],
          "public": ["css/styles.css", "js/app.js"],
          "private": ["views/index.html", "views/hello.html"]
        }
      },
      "api_users": {
        "path": "workers/api_users",
        "binary": "bin/api_users",
        "binary_md5": "apiusers123456",
        "route": "/api/users",
        "size_bytes": 7340032,
        "files": {
          "src": ["main.go"],
          "public": ["swagger/index.html"],
          "private": ["config/validation.yaml"]
        }
      }
    }
  },
  
  "previous_deployments": [
    {
      "git_commit": "b2c3d4e5f67890123456789abcdef0123456789",
      "git_commit_short": "b2c3d4e",
      "deployed_at": "2026-01-05T10:00:00Z",
      "status": "rollback_available"
    }
  ]
}
```

### Atomic Deployment Process

1. **Build Phase**:
   ```bash
   # Build all workers and copy assets
   ./scripts/build-prod.sh
   # - Gets git commit hash
   # - Creates deploy/{commit_hash}/ directory
   # - Builds binaries (no embed)
   # - Copies public/ and private/ assets
   # - Creates manifest.json pointing to {commit_hash} folder
   ```

2. **Deploy Phase**:
   ```bash
   # Upload deployment folder atomically
   ./scripts/deploy.sh production
   # - Uploads entire deploy/{commit_hash}/ folder
   # - Creates deploy/manifest.json.new
   # - Atomic rename: manifest.json.new -> manifest.json
   ```

3. **Server Detection**:
   - Server watches `deploy/manifest.json` (mtime check every 60s)
   - On change:
     - Parse new manifest
     - Compare git commit hash against running deployment
     - If commit changed:
       - Start new workers on new ports
       - Health check
       - Switch traffic
       - Stop old workers
   - On SIGHUP: Force manifest reload

### Development Mode

#### File Watcher
Monitors changes to source and resource files:
```yaml
file_watcher:
  enabled: true  # Only in dev mode
  watch_patterns:
    - "workers/*/src/**/*.go"
    - "workers/*/public/**/*"
    - "workers/*/private/**/*"
    - "server/src/**/*.go"
    - "config/*.yaml"
  
  debounce_ms: 100
  
  on_change:
    compute_folder_md5: true
    rebuild_if_changed: true
    restart_worker: true
```

#### Development Build
```bash
# Build without embed (faster, smaller)
go build -tags=dev -o workers/index/bin/dev/index workers/index/src/*.go

# Binary loads resources from filesystem:
# - public/ → served via HTTP
# - private/ → loaded via os.ReadFile
```

#### Hot Reload Flow
1. Developer edits `workers/index/src/main.go`
2. File watcher detects change
3. Check if binary needs rebuild (source files changed)
4. If rebuild needed:
   - Rebuild `workers/index/bin/index`
   - Restart worker on new port
   - Update internal tracking
5. If only resource file changed (HTML/CSS/JS):
   - No rebuild needed in dev mode (served from disk)
   - Worker detects change and reloads template cache

### Production Mode

#### No File Watching
```yaml
file_watcher:
  enabled: false  # Disabled in prod

deployment:
  mode: "prod"
  manifest_file: "deploy/manifest.json"
  check_interval_seconds: 60
  
  on_manifest_change:
    - validate_manifest
    - compare_md5s
    - restart_changed_workers
```

#### Production Binary and Assets
```bash
# Get git commit hash
COMMIT_HASH=$(git rev-parse HEAD)

# Ensure working directory is clean
if [ -n "$(git status --porcelain)" ]; then
    echo "Error: Working directory has uncommitted changes"
    exit 1
fi

# Create deployment directory
mkdir -p deploy/$COMMIT_HASH/workers/index

# Build binary (no embed, include git info in ldflags)
go build -ldflags="-s -w -X main.GitCommit=$COMMIT_HASH -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o deploy/$COMMIT_HASH/workers/index/bin/index \
  workers/index/src/*.go

# Copy assets
cp -r workers/index/public deploy/$COMMIT_HASH/workers/index/
cp -r workers/index/private deploy/$COMMIT_HASH/workers/index/
```

#### Resource Access in Code
```go
package main

import (
    "net/http"
    "os"
    "path/filepath"
)

func loadTemplate(name string) ([]byte, error) {
    basePath := os.Getenv("TQ_WORKER_BASE") // e.g., "deploy/a1b2c3d4e5f6.../workers/index"
    templatePath := filepath.Join(basePath, "private/views", name)
    return os.ReadFile(templatePath)
}

func serveStatic(w http.ResponseWriter, r *http.Request) {
    basePath := os.Getenv("TQ_WORKER_BASE")
    publicPath := filepath.Join(basePath, "public")
    http.FileServer(http.Dir(publicPath)).ServeHTTP(w, r)
}
```

## Configuration Changes

### New `config/deployment.yaml`

```yaml
deployment:
  mode: "dev"  # "dev" or "prod", can use env: ${DEPLOYMENT_MODE:-dev}
  
  # Development mode settings
  dev:
    file_watcher:
      enabled: true
      debounce_ms: 100
      watch_patterns:
        - "workers/*/src/**/*.go"
        - "workers/*/public/**/*"
        - "workers/*/private/**/*"
        - "workers/shared/**/*"
        - "server/src/**/*.go"
        - "config/*.yaml"
    
    build:
      output_dir: "bin"  # workers/{name}/bin/{name}
      compute_md5: true  # For change detection
      parallel_builds: 4
    
    resources:
      base_path: "workers"  # Load from workers/{name}/
      cache_templates: false  # Reload on every request
      watch_resource_changes: true
  
  # Production mode settings
  prod:
    file_watcher:
      enabled: false  # No file watching
    
    build:
      output_dir: "deploy"
      compute_md5: true
      parallel_builds: 8
      ldflags: "-s -w"  # Strip debug info
    
    deployment:
      folder_naming: "{git_commit}"  # Deploy folder named with git commit hash
      copy_assets: true              # Copy public/ and private/ to deploy folder
      require_clean_working_dir: true  # Fail if uncommitted changes
      
    manifest:
      file: "deploy/manifest.json"
      check_interval_seconds: 60
      require_signature: false  # Future: GPG signature
    
    reload_triggers:
      - "manifest_change"  # Detect via mtime
      - "sighup"           # Manual reload signal
    
    resources:
      base_path: "deploy/{git_commit}"  # Load from deployment folder
      cache_templates: true
      watch_resource_changes: false

# Git configuration
git:
  require_clean_working_dir: true  # Fail build if uncommitted changes
  include_commit_in_binary: true   # Embed git info in binary via ldflags
  use_short_hash: false            # Use full hash for folder names
  
  # Git information embedded in binaries
  ldflags:
    - "-X main.GitCommit={{.Commit}}"
    - "-X main.GitBranch={{.Branch}}"
    - "-X main.GitTag={{.Tag}}"
    - "-X main.BuildTime={{.Time}}"

# Binary and folder naming
binaries:
  format: "{worker_name}"  # Same for dev and prod
  
  dev:
    path: "workers/{worker_name}/bin/{worker_name}"
  
  prod:
    folder: "deploy/{git_commit}/workers/{worker_name}"
    path: "bin/{worker_name}"
```

### Updates to `config/server.yaml`

```yaml
server:
  port: 8080
  read_timeout_seconds: 30
  write_timeout_seconds: 30
  idle_timeout_seconds: 120
  
  # Deployment mode (overrides deployment.yaml if set)
  deployment_mode: "${DEPLOYMENT_MODE}"  # Reads from env, or use explicit "dev"/"prod"
  
  # Static file serving
  static:
    enabled: true
    url_prefix: "/static"
    cache_max_age_seconds: 3600  # Browser cache
    
    # Dev mode: serve from filesystem
    # Prod mode: serve from embedded resources
    dev_root: "workers"  # Serves from workers/{name}/public/
    
workers:
  # Binary location depends on mode
  bin_dir: "${WORKERS_BIN_DIR}"  # Set by deployment config
  
  # In prod: base path is deployment folder
  deployment_base: "${DEPLOYMENT_BASE}"  # e.g., "deploy/a1b2c3d4e5f6.../"
  
  # Port management
  port_range_start: 9000
  port_range_end: 9999
  
  # Worker lifecycle
  startup_delay_ms: 100
  restart_delay_ms: 100
  shutdown_grace_period_ms: 500
  
  # Manifest tracking (prod only)
  manifest_file: "deploy/manifest.json"
  check_manifest_interval_seconds: 60
  
  # Worker discovery
  discovery:
    mode: "filesystem"  # or "manifest" in prod
    workers_dir: "workers"
    include_patterns:
      - "*/src/main.go"  # Any dir with src/main.go is a worker
  
  default:
    go_max_procs: 1
    max_requests: 0
    read_timeout_seconds: 30
    write_timeout_seconds: 30
    idle_timeout_seconds: 120
    go_mem_limit: ""
    
    # Environment variables passed to workers
    env:
      TQ_MODE: "${DEPLOYMENT_MODE}"
      TQ_WORKER_BASE: "${WORKER_BASE}"  # Base path for this worker (public/ and private/ are always under this)
  
  paths:
    "/api":
      go_max_procs: 2
      max_requests: 5000
```

## Build Process

### Development Build Script: `scripts/build-dev.sh`

```bash
#!/bin/bash
set -e

WORKERS_DIR="workers"

echo "Building workers for development..."

for worker_dir in $WORKERS_DIR/*/; do
    worker_name=$(basename "$worker_dir")
    
    # Skip shared directory
    if [ "$worker_name" = "shared" ]; then
        continue
    fi
    
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
```

### Production Build Script: `scripts/build-prod.sh`

```bash
#!/bin/bash
set -e

WORKERS_DIR="workers"
SERVER_DIR="server"
DEPLOY_DIR="deploy"
MANIFEST_FILE="$DEPLOY_DIR/manifest.json"

echo "Building for production deployment..."

# Check for uncommitted changes
if [ -n "$(git status --porcelain)" ]; then
    echo "Error: Working directory has uncommitted changes"
    echo "Please commit or stash changes before building for production"
    exit 1
fi

# Get git information
COMMIT_HASH=$(git rev-parse HEAD)
COMMIT_SHORT=$(git rev-parse --short HEAD)
GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
GIT_TAG=$(git describe --tags --exact-match 2>/dev/null || echo "")
BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)

echo "Git Commit: $COMMIT_HASH"
echo "Branch: $GIT_BRANCH"
if [ -n "$GIT_TAG" ]; then
    echo "Tag: $GIT_TAG"
fi

# Create deployment directory
DEPLOYMENT_DIR="$DEPLOY_DIR/$COMMIT_HASH"
if [ -d "$DEPLOYMENT_DIR" ]; then
    echo "Deployment for commit $COMMIT_HASH already exists"
    echo "Using existing deployment: $DEPLOYMENT_DIR"
    # Update manifest and exit
    exit 0
fi

mkdir -p "$DEPLOYMENT_DIR/server"
mkdir -p "$DEPLOYMENT_DIR/workers"

echo "Building to: $DEPLOYMENT_DIR"

# Build server
echo "Building server..."
mkdir -p "$DEPLOYMENT_DIR/server/bin"
go build -ldflags="-s -w -X main.GitCommit=$COMMIT_HASH -X main.GitBranch=$GIT_BRANCH -X main.GitTag=$GIT_TAG -X main.BuildTime=$BUILD_TIME" \
    -o "$DEPLOYMENT_DIR/server/bin/tqserver" \
    $SERVER_DIR/src/*.go

server_md5=$(md5sum "$DEPLOYMENT_DIR/server/bin/tqserver" | cut -d' ' -f1)
echo "✓ Built server (commit: $COMMIT_SHORT, binary MD5: $server_md5)"

# Copy server assets
if [ -d "$SERVER_DIR/public" ]; then
    cp -r "$SERVER_DIR/public" "$DEPLOYMENT_DIR/server/"
    echo "  Copied server public assets"
fi
if [ -d "$SERVER_DIR/private" ]; then
    cp -r "$SERVER_DIR/private" "$DEPLOYMENT_DIR/server/"
    echo "  Copied server private resources"
fi

# Initialize manifest
cat > "$MANIFEST_FILE.tmp" <<EOF
{
  "version": "$BUILD_TIME",
  "deployment_id": "deploy-$(date +%s)",
  "current_deployment": "$COMMIT_HASH",
  "deployment_path": "$DEPLOYMENT_DIR",
  "deployment": {
    "git_commit": "$COMMIT_HASH",
    "git_commit_short": "$COMMIT_SHORT",
    "git_branch": "$GIT_BRANCH",
    "git_tag": "$GIT_TAG",
    "built_at": "$BUILD_TIME",
    "server": {
      "path": "server",
      "binary": "bin/tqserver",
      "binary_md5": "$server_md5"
    },
    "workers": {}
  }
}
EOF

# Build workers
for worker_dir in $WORKERS_DIR/*/; do
    worker_name=$(basename "$worker_dir")
    
    if [ "$worker_name" = "shared" ]; then
        continue
    fi
    
    src_dir="$worker_dir/src"
    if [ ! -f "$src_dir/main.go" ]; then
        continue
    fi
    
    echo "Building worker: $worker_name..."
    
    # Create worker directory in deployment
    worker_deploy="$DEPLOYMENT_DIR/workers/$worker_name"
    mkdir -p "$worker_deploy/bin"
    
    # Build binary with git information
    go build -ldflags="-s -w -X main.GitCommit=$COMMIT_HASH -X main.GitBranch=$GIT_BRANCH -X main.GitTag=$GIT_TAG -X main.BuildTime=$BUILD_TIME" \
        -o "$worker_deploy/bin/$worker_name" \
        "$src_dir"/*.go
    
    worker_md5=$(md5sum "$worker_deploy/bin/$worker_name" | cut -d' ' -f1)
    worker_size=$(stat -c%s "$worker_deploy/bin/$worker_name" 2>/dev/null || stat -f%z "$worker_deploy/bin/$worker_name")
    
    # Copy assets
    if [ -d "$worker_dir/public" ]; then
        cp -r "$worker_dir/public" "$worker_deploy/"
        echo "  Copied public assets"
    fi
    
    if [ -d "$worker_dir/private" ]; then
        cp -r "$worker_dir/private" "$worker_deploy/"
        echo "  Copied private resources"
    fi
    
    # Detect route
    route="/$worker_name"
    if [ "$worker_name" = "index" ]; then
        route="/"
    fi
    
    # Add to manifest
    jq --arg name "$worker_name" \
       --arg path "workers/$worker_name" \
       --arg binary "bin/$worker_name" \
       --arg md5 "$worker_md5" \
       --arg route "$route" \
       --arg size "$worker_size" \
       '.deployment.workers[$name] = {
         "path": $path,
         "binary": $binary,
         "binary_md5": $md5,
         "route": $route,
         "size_bytes": ($size | tonumber)
       }' "$MANIFEST_FILE.tmp" > "$MANIFEST_FILE.tmp2"
    
    mv "$MANIFEST_FILE.tmp2" "$MANIFEST_FILE.tmp"
    
    echo "✓ Built $worker_name"
    echo "  Binary: $worker_deploy/bin/$worker_name"
    echo "  Binary MD5: $worker_md5"
    echo "  Size: $worker_size bytes"
    echo "  Git Commit: $COMMIT_SHORT"
done

# Finalize manifest
mv "$MANIFEST_FILE.tmp" "$MANIFEST_FILE"

echo ""
echo "✅ Production build complete"
echo "Deployment folder: $DEPLOYMENT_DIR"
echo "Git Commit: $COMMIT_HASH"
echo "Git Branch: $GIT_BRANCH"
if [ -n "$GIT_TAG" ]; then
    echo "Git Tag: $GIT_TAG"
fi
echo "Manifest: $MANIFEST_FILE"
echo ""
echo "Structure:"
echo "  deploy/$COMMIT_HASH/"
echo "    ├── server/"
echo "    │   ├── bin/tqserver"
echo "    │   ├── public/"
echo "    │   └── private/"
echo "    └── workers/"
for worker_dir in $DEPLOYMENT_DIR/workers/*/; do
    if [ -d "$worker_dir" ]; then
        worker_name=$(basename "$worker_dir")
        echo "        ├── $worker_name/"
        echo "        │   ├── bin/$worker_name"
        [ -d "$worker_dir/public" ] && echo "        │   ├── public/"
        [ -d "$worker_dir/private" ] && echo "        │   └── private/"
    fi
done
```

### Deployment Script: `scripts/deploy.sh`

```bash
#!/bin/bash
set -e

if [ $# -lt 1 ]; then
    echo "Usage: $0 <target> [--dry-run]"
    echo "Targets: staging, production"
    exit 1
fi

TARGET=$1
DRY_RUN=""
if [ "$2" = "--dry-run" ]; then
    DRY_RUN="--dry-run"
fi

# Load deployment config
case $TARGET in
    staging)
        SERVER="deploy@staging.example.com"
        DEPLOY_PATH="/opt/tqserver"
        ;;
    production)
        SERVER="deploy@prod.example.com"
        DEPLOY_PATH="/opt/tqserver"
        ;;
    *)
        echo "Unknown target: $TARGET"
        exit 1
        ;;
esac

echo "Deploying to $TARGET..."
echo "Server: $SERVER"
echo "Path: $DEPLOY_PATH"
echo ""

# Build production binaries and assets
echo "Step 1: Building production deployment..."
./scripts/build-prod.sh

# Get deployment commit hash from manifest
COMMIT_HASH=$(jq -r '.current_deployment' deploy/manifest.json)
COMMIT_SHORT=$(jq -r '.deployment.git_commit_short' deploy/manifest.json)
GIT_BRANCH=$(jq -r '.deployment.git_branch' deploy/manifest.json)

echo ""
echo "Deploying commit: $COMMIT_SHORT ($COMMIT_HASH)"
echo "Branch: $GIT_BRANCH"

# Create deployment package
echo ""
echo "Step 2: Creating deployment package..."
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
PACKAGE="tqserver-deploy-$TIMESTAMP.tar.gz"

# Package the deployment folder and manifest
tar -czf "$PACKAGE" \
    deploy/manifest.json \
    deploy/$COMMIT_HASH/

echo "Created: $PACKAGE"
echo "Size: $(du -h $PACKAGE | cut -f1)"

# Upload to server
echo ""
echo "Step 3: Uploading to server..."
if [ -n "$DRY_RUN" ]; then
    echo "[DRY RUN] Would upload $PACKAGE to $SERVER:/tmp/"
else
    scp "$PACKAGE" "$SERVER:/tmp/"
fi

# Extract and activate on server (atomic)
echo ""
echo "Step 4: Extracting on server..."
if [ -n "$DRY_RUN" ]; then
    echo "[DRY RUN] Would extract and activate deployment"
else
    ssh "$SERVER" <<EOF
set -e
cd $DEPLOY_PATH

# Extract deployment
tar -xzf /tmp/$PACKAGE

# Validate manifest
if [ ! -f deploy/manifest.json ]; then
    echo "ERROR: No manifest.json in deployment package"
    exit 1
fi

# Validate deployment folder exists
COMMIT_HASH=\$(jq -r '.current_deployment' deploy/manifest.json)
COMMIT_SHORT=\$(jq -r '.deployment.git_commit_short' deploy/manifest.json)
if [ ! -d "deploy/\$COMMIT_HASH" ]; then
    echo "ERROR: Deployment folder deploy/\$COMMIT_HASH not found"
    exit 1
fi

echo "Deploying commit: \$COMMIT_SHORT (\$COMMIT_HASH)"
echo "Deployment folder: deploy/\$COMMIT_HASH"
echo "Contents:"
ls -la "deploy/\$COMMIT_HASH/"

# Trigger reload (server will detect manifest change)
echo "Triggering server reload..."
sudo systemctl reload tqserver || killall -HUP tqserver

# Cleanup old deployment after 60 seconds (safety window)
(sleep 60 && rm -rf deploy.old /tmp/$PACKAGE) &

echo "✅ Deployment complete"
EOF
fi

echo ""
echo "✅ Deployed to $TARGET successfully"
echo ""
echo "Monitor deployment:"
echo "  ssh $SERVER 'tail -f $DEPLOY_PATH/logs/server_*.log'"
```

## Implementation Plan

### Phase 1: Restructure Directories (Week 1)
1. **Reorganize worker directories**
   - Create `workers/` top-level directory
   - Move `pages/index/` → `workers/index/`
   - Create `src/`, `bin/`, `public/`, `private/` in each worker
   - Move `.go` files to `src/`
   - Move `.html` to `private/views/`
   - Move static assets to `public/`

2. **Reorganize server**
   - Create `server/src/`, `server/bin/`, `server/public/`, `server/private/`
   - Move `cmd/tqserver/*.go` and `internal/**/*.go` to `server/src/`
   - Create `server/public/admin/` for admin UI

3. **Update build paths**
   - Update `go.mod` with new paths
   - Update import statements
   - Test builds still work

### Phase 2: MD5 and Manifest System (Week 1-2)
1. **Implement MD5 utilities**
   - `pkg/deployment/md5.go`: Folder MD5 calculation
   - `pkg/deployment/manifest.go`: Manifest read/write
   - `pkg/deployment/validator.go`: Manifest validation

2. **Build scripts**
   - `scripts/build-dev.sh`: Dev builds with MD5 tracking
   - `scripts/build-prod.sh`: Prod builds with embed
   - Both compute folder MD5 for change detection

3. **Manifest structure**
   - Define JSON schema
   - Implement atomic write (write + rename)
   - Add validation logic

### Phase 3: Folder-Based Deployment (Week 2)
1. **Deployment folder structure**
   - Create `deploy/{md5}/` for each deployment
   - Copy binaries to `deploy/{md5}/workers/{name}/bin/`
   - Copy assets to `deploy/{md5}/workers/{name}/public/` and `private/`

2. **Resource loading abstraction**
   - `pkg/worker/resources.go`: Unified resource loading
   - Base path from environment variable
   - Template cache management

3. **Static file serving**
   - Serve from deployment folder in prod mode
   - Serve from workers/ in dev mode
   - URL routing: `/static/{worker}/path`

### Phase 4: Development Mode (Week 2-3)
1. **File watcher updates**
   - Watch new directory structure
   - Compute folder MD5 on changes
   - Rebuild only if MD5 changed
   - No symlinks - direct binary reference

2. **Dev build process**
   - Build without `-tags=prod` (no embed)
   - Smaller, faster builds
   - Resources loaded from filesystem

3. **Hot reload**
   - Detect MD5 changes
   - Rebuild and restart worker
   - No symlink management needed

### Phase 5: Production Mode (Week 3)
1. **Manifest detection**
   - Watch `deploy/manifest.json` via mtime
   - Parse on changes
   - Compare MD5s against running workers

2. **SIGHUP handler**
   - Reload config on SIGHUP
   - Re-read manifest
   - Trigger rolling restart of changed workers

3. **Disable file watcher in prod**
   - Check deployment mode from config
   - Skip watcher if mode=prod

4. **Worker restart logic**
   - Start new worker on new port
   - Health check
   - Switch traffic
   - Stop old worker
   - Update internal tracking

### Phase 6: Deployment Tools (Week 3-4)
1. **Packaging scripts**
   - `scripts/build-prod.sh`: Build all + copy assets to versioned folder
   - `scripts/deploy.sh`: Upload deployment folder and activate

2. **Atomic deployment**
   - Create `deploy/{md5}/` with complete structure
   - Upload entire folder
   - Update manifest to point to new folder

3. **Rollback support**
   - Keep previous deployment folders
   - Quick rollback: update manifest to previous MD5
   - Cleanup old folders (configurable retention)

### Phase 7: Testing and Documentation (Week 4)
1. **Unit tests**
   - MD5 calculation
   - Manifest operations
   - Resource loading (embedded vs filesystem)

2. **Integration tests**
   - Dev mode workflow
   - Prod deployment workflow
   - Rolling restart
   - SIGHUP handling

3. **Documentation**
   - Updated README with new structure
   - Deployment guide
   - Development guide
   - Migration guide from old structure

## Migration Path

### For Existing Deployments

1. **Create new structure alongside old**:
   ```bash
   mkdir -p workers/index
   cp -r pages/index workers/index/src
   # ... reorganize files
   ```

2. **Update build to produce both**:
   - Old format: `bin/tqworker_index`
   - New format: `workers/index/bin/dev/index`

3. **Gradual cutover**:
   - Test new structure in dev
   - Deploy to staging
   - Deploy to production with rollback plan

4. **Remove old structure** once validated

## Benefits

### Development
✅ **Clear organization**: Each worker is self-contained with src/bin/public/private
✅ **Fast builds**: Single binary, no separate dev/prod builds
✅ **Hot reload**: MD5-based change detection (no unnecessary rebuilds)
✅ **No symlinks**: Simpler filesystem operations
✅ **Isolated resources**: Each worker has its own assets
✅ **Single binary**: Same binary runs in dev and prod (mode via env vars)

### Production
✅ **Versioned deployments**: Each deployment in its own git commit-named folder
✅ **Git traceability**: Direct link between deployment and source code
✅ **Atomic deployments**: Manifest-based with validation
✅ **Zero-downtime**: Workers restart on new ports
✅ **Change detection**: Only restart what changed (folder MD5-based)
✅ **No file watching**: Reduced overhead and attack surface
✅ **Easy rollback**: Keep old deployment folders, update manifest
✅ **Clear structure**: Binary + assets together in deployment folder
✅ **Smaller binaries**: No embedded resources bloat

### Operations
✅ **Audit trail**: Manifest tracks all deployments
✅ **Easy rollback**: Keep old deployment, swap manifest
✅ **Resource efficiency**: Only changed workers restart
✅ **Simpler deployment**: Single tar.gz with manifest
✅ **No coordination needed**: SIGHUP triggers reload

## Security Considerations

### Resource Access
- Resources in deployment folder (read-only filesystem)
- Workers access via environment-provided base path
- No dynamic path construction (prevents directory traversal)

### Manifest Integrity
- MD5 validation of manifest file
- Future: Add GPG signatures
- Atomic operations prevent partial updates

### File Permissions
- `deploy/` directory read-only for server process
- Separate deployment user from runtime user
- Binaries executable only, not writable

## Performance Considerations

### Deployment Size
- **Dev mode**: Binaries only (~2-5MB per worker)
- **Prod mode**: Binary + assets per folder (~5-20MB per worker)
- Deployment folders can be cleaned up (keep last N)

### MD5 Calculation
- **Dev mode**: Computed on file change (cached)
- **Prod mode**: Pre-computed during build (entire deployment)
- Folder MD5 cached between builds

### Manifest Checking
- Check interval: 60 seconds (configurable)
- Cost: Single file stat() + JSON parse if changed
- Negligible overhead

### Resource Access
- **Dev mode**: Filesystem I/O (but cached)
- **Prod mode**: Filesystem I/O from deployment folder
- Both modes use OS filesystem cache

## Future Enhancements

### V2: Advanced Features
- **Binary signing**: GPG signatures in manifest
- **Deployment compression**: Compressed deployment archives
- **Version pinning**: Pin routes to specific deployment versions
- **Canary deployments**: Deploy to subset of workers
- **Incremental updates**: Rsync-based deployment for large assets
- **Optional embedding**: Hybrid mode with critical assets embedded

### V3: Cluster Support
- **Distributed manifest**: Share via Redis/etcd
- **Coordinated restarts**: Cluster-wide orchestration
- **Binary CDN**: Central storage for binaries
- **Health aggregation**: Cluster health monitoring

## Open Questions

1. **Deployment cleanup**: How long to keep old deployment folders?
   - **Proposed**: Keep last 5 deployments or folders from last 7 days
   - Keep deployments matching git tags indefinitely

2. **Manifest conflicts**: What if manifest.json changes while reading?
   - **Proposed**: Use atomic file operations, validate JSON before applying

3. **Partial deployments**: Support deploying single route?
   - **Proposed**: No, always deploy entire folder (atomicity)

4. **Hot config reload**: Which config changes require restart?
   - **Proposed**: Port changes require restart, others can reload

5. **Large assets**: What if deployment folders are huge?
   - **Proposed**: Use rsync for incremental updates, or CDN for static assets

6. **Shared resources**: How to handle `workers/shared/`?
   - **Proposed**: Copy into each worker's deployment folder during build

## Conclusion

This specification provides a comprehensive plan for organizing workers and resources with a clear separation between development and production modes. The use of git commit hash-named deployment folders in production provides versioned, atomic, and traceable deployments with easy rollback capabilities.

Key advantages of this approach:
- **Git-based versioning**: Each deployment tied to exact source code version
- **Traceability**: Can see exactly what code is deployed
- **Simple rollback**: Update manifest to point to previous commit
- **Reproducible**: Can rebuild identical deployment from git
- **Clear structure**: Binary + assets together, easy to understand
- **No embedding complexity**: Simpler build process, standard file I/O
- **Flexible**: Easy to add CDN or other optimizations later

The phased implementation approach allows for incremental development and testing, while maintaining backwards compatibility during migration. The folder-based approach provides a solid foundation for future enhancements like cluster deployments and asset CDNs.
  


  
                                                        
    
            
        

            
                  
        
            
                  
    
  
          
            











 
 
 


  
 
 

 
 
 

















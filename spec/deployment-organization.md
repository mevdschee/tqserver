# Deployment Organization Specification

## Overview

This specification defines how to organize binaries, web root files (static assets), and view files (HTML templates) in a way that supports:
- **Incremental deployments** with file timestamp-based change detection
- **Development mode** with file watchers for hot reload and external files
- **Zero-downtime upgrades** by starting workers on new ports when files change
- **Flexible updates** allowing any asset or binary to be updated independently

## Key Principles

1. **Dev vs Prod Separation**: Development uses file watchers; production uses timestamp-based detection
2. **Structured Folders**: Each worker and server has `src/`, `bin/`, `public/`, `private/` organization
3. **Assets Alongside Binaries**: Production deployments include binaries + assets (no embedding)

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
- Served from `workers/{name}/public/` (dev and prod)
- URL mapping: `/static/{worker}/path/to/file.css`
- **Always** located at `{WORKER_BASE}/public/` (hardcoded)

#### `private/` - Private Resources
- Templates, config files, not directly accessible via HTTP
- Loaded from `workers/{name}/private/` (dev and prod)
- **Always** located at `{WORKER_BASE}/private/` (hardcoded)
- Includes:
  - `views/`: HTML templates
  - `templates/`: Shared template components
  - `config/`: Worker-specific configuration

## File Timestamp-Based Change Detection

### How It Works

The server continuously monitors worker binaries and assets by checking file modification timestamps (mtime):

1. **Track Running Workers**: Server maintains a registry of each running worker with:
   - Worker name and route
   - PID and port
   - Binary path and mtime at startup
   - Asset directories (public/private) and latest mtime

2. **SIGHUP Signal**: On receiving SIGHUP, server checks:
   - Server binary file mtime: `server/bin/tqserver`
   - Worker binary file mtime: `workers/{name}/bin/{name}`

3. **Restart or Reload on Changes**: If any file has newer mtime than recorded:
   - If binary changed: Full worker restart (new port, health check, traffic switch)
   - If only config changed: Reload server using sighup
   - Update registry with new mtimes on startup/reload

### Worker Registry Structure

The server tracks running workers in memory:

```go
type WorkerInstance struct {
    Name      string
    Route     string
    PID       int
    Port      int
    StartedAt time.Time
    
    // File tracking
    BinaryPath    string
    BinaryMtime   time.Time
    PublicPath    string
    PublicMtime   time.Time
    PrivatePath   string
    PrivateMtime  time.Time
    
    // Health
    Status        string  // "starting", "healthy", "stopping"
    LastHealthCheck time.Time
}

// Example runtime state
var runningWorkers = map[string]*WorkerInstance{
    "index": {
        Name:      "index",
        Route:     "/",
        PID:       12345,
        Port:      9001,
        StartedAt: time.Parse("2026-01-06T10:30:00Z"),
        
        BinaryPath:  "workers/index/bin/index",
        BinaryMtime: time.Parse("2026-01-06T10:29:45Z"),
        PublicPath:  "workers/index/public",
        PublicMtime: time.Parse("2026-01-06T09:15:20Z"),
        PrivatePath: "workers/index/private",
        PrivateMtime: time.Parse("2026-01-06T09:15:20Z"),
          "public": ["swagger/index.html"],
        "private": ["config/validation.yaml"]
    }
}
```

### Development Mode

#### File Watcher
Monitors changes to source and config files:
```yaml
file_watcher:
  enabled: true  # Only in dev mode
  debounce_ms: 100
  watch_patterns:
    - "workers/*/src/**/*.go"
    - "server/src/**/*.go"
    - "config/*.yaml" 

```

#### Development Build
```bash
# Build to workers/{name}/bin/{name}
go build -o workers/index/bin/index workers/index/src/*.go

# Binary loads resources from filesystem:
# - public/ → served via HTTP
# - private/ → loaded via os.ReadFile
```

#### Hot Reload Flow
1. Developer edits `workers/index/src/main.go`
2. File watcher detects change
3. Rebuild `workers/index/bin/index`
4. File mtime updates
5. A sighup is fired at the server
6. Server detects newer binary mtime
7. Server restarts worker on new port
8. Traffic switched to new worker

### Production Mode

#### Timestamp Checking
```yaml
file_watcher:
  enabled: false  # Disabled in prod (use SIGHUP instead)

deployment:
  mode: "prod"
  check_on_sighup: true  # Check file timestamps on SIGHUP signal
  
  on_file_change:
    - restart_server  # If binary changed
    - restart_affected_worker  # If binary changed
```

#### Production Build and Deploy
```bash
# Build binary directly to worker bin directory
go build -ldflags="-s -w" -o workers/index/bin/index workers/index/src/*.go

# Or deploy via rsync (preserves mtimes)
rsync -avz --checksum \
  workers/ user@server:/opt/tqserver/workers/

# Or via scp
scp workers/index/bin/index user@server:/opt/tqserver/workers/index/bin/

# Server will detect mtime change and restart worker
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
        - "server/src/**/*.go"
        - "config/*.yaml"
    
    build:
      output_dir: "bin"  # workers/{name}/bin/{name}
      parallel_builds: 4
    
    resources:
      base_path: "workers"  # Load from workers/{name}/
      watch_resource_changes: true
  
  # Production mode settings
  prod:
    file_watcher:
      enabled: false  # Use SIGHUP instead
    
    timestamp_check:
      enabled: true
      trigger: "sighup"  # Only check on SIGHUP signal
    
    build:
      output_dir: "workers"  # Build directly to workers/{name}/bin/
      parallel_builds: 8
      ldflags: "-s -w"  # Strip debug info
    
    resources:
      base_path: "workers"  # Load from workers/{name}/
      watch_resource_changes: false

# Binary naming
binaries:
  format: "{worker_name}"
  path: "workers/{worker_name}/bin/{worker_name}"
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
  # Worker binary directory
  workers_dir: "workers"
  
  # Port management
  port_range_start: 9000
  port_range_end: 9999
  
  # Worker lifecycle
  startup_delay_ms: 100
  restart_delay_ms: 100
  shutdown_grace_period_ms: 500
  
  # Timestamp checking (prod only)
  timestamp_check:
    enabled: true  # In prod mode
    trigger: "sighup"  # Only on SIGHUP signal
  
  # Worker discovery
  discovery:
    mode: "filesystem"
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
```

### Deployment Script: `scripts/deploy.sh`

```bash
#!/bin/bash
set -e

if [ $# -lt 1 ]; then
    echo "Usage: $0 <target> [workers...]"
    echo "Targets: staging, production"
    echo "Examples:"
    echo "  $0 production              # Deploy all"
    echo "  $0 production index        # Deploy only index worker"
    echo "  $0 production index api    # Deploy index and api workers"
    exit 1
fi

TARGET=$1
shift
DEPLOY_WORKERS=("$@")

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

# If no specific workers specified, deploy all
if [ ${#DEPLOY_WORKERS[@]} -eq 0 ]; then
    echo "Deploying: all workers and server"
    
    # Deploy server
    echo "Deploying server..."
    rsync -avz --checksum server/bin/tqserver "$SERVER:$DEPLOY_PATH/server/bin/"
    rsync -avz --checksum server/public/ "$SERVER:$DEPLOY_PATH/server/public/" 2>/dev/null || true
    rsync -avz --checksum server/private/ "$SERVER:$DEPLOY_PATH/server/private/" 2>/dev/null || true
    
    # Deploy all workers
    echo "Deploying all workers..."
    rsync -avz --checksum --exclude='src' workers/ "$SERVER:$DEPLOY_PATH/workers/"
else
    # Deploy specific workers
    for worker in "${DEPLOY_WORKERS[@]}"; do
        echo "Deploying worker: $worker..."
        rsync -avz --checksum --exclude='src' "workers/$worker/" "$SERVER:$DEPLOY_PATH/workers/$worker/"
    done
fi

echo ""
echo "✅ Deployed to $TARGET successfully"
echo ""
echo "Trigger reload: ssh $SERVER 'killall -HUP tqserver'"
echo ""
echo "Monitor deployment:"
echo "  ssh $SERVER 'tail -f $DEPLOY_PATH/logs/server_*.log'"
```

## Implementation Plan

### Phase 1: Restructure Directories (Week 1)
1. **Reorganize worker directories**
   - Create `workers/` top-level directory
   - Move `pages/index/` → `workers/index/`
   - Create `src/`, `bin/`, `public/`, `views/`, `config/`, `data/` in each worker
   - Move `.go` files to `src/`
   - Move `.html` to `views/`
   - Move static assets to `public/`
   - Worker configs to `config/`
   - Worker data to `data/`

2. **Reorganize server**
   - Create `server/src/`, `server/bin/`, `server/public/`
   - Move `cmd/tqserver/*.go` and `internal/**/*.go` to `server/src/`
   - Create `server/public/admin/` for admin UI

3. **Update build paths**
   - Update `go.mod` with new paths
   - Update import statements
   - Test builds still work

### Phase 2: Timestamp-Based Change Detection (Week 1-2)
1. **Implement timestamp tracking utilities**
   - `pkg/supervisor/timestamps.go`: File mtime tracking
   - `pkg/supervisor/registry.go`: Worker registry with file mtimes
   - `pkg/supervisor/checker.go`: SIGHUP-triggered timestamp checking

2. **Build scripts**
   - `scripts/build-dev.sh`: Dev builds with timestamp-based rebuilds
   - `scripts/build-prod.sh`: Prod builds with optimizations
   - `scripts/deploy.sh`: rsync-based deployment

3. **Worker registry**
   - In-memory registry of running workers
   - Track binary and asset mtimes
   - Implement comparison logic on SIGHUP

### Phase 3: Resource Loading (Week 2)
1. **Resource loading abstraction**
   - `pkg/worker/resources.go`: Unified resource loading
   - Base path from environment variable (`TQ_WORKER_BASE`)
   - Template cache management

2. **Static file serving**
   - Serve from `workers/{name}/public/` in all modes
   - URL routing: `/static/{worker}/path`
   - Browser caching headers

### Phase 4: Development Mode (Week 2-3)
1. **File watcher updates**
   - Watch new directory structure
   - Trigger rebuilds on source changes
   - Restart workers on binary/asset changes

2. **Dev build process**
   - Build to `workers/{name}/bin/{name}`
   - Fast incremental builds
   - No optimization flags
   - Smaller, faster builds
   - Resources loaded from filesystem

3. **Hot reload**
   - Detect file changes via watcher
   - Rebuild and restart worker immediately

### Phase 5: Production Mode (Week 3)
1. **SIGHUP-triggered checking**
   - Register SIGHUP signal handler
   - Check mtimes of binaries and assets on signal
   - Compare against recorded mtimes in registry

2. **Smart restart logic**
   - Binary change: Full restart (new port, health check, traffic switch)
   - Asset-only change: Reload without restart
   - Update registry with new mtimes

3. **Disable file watcher in prod**
   - Check deployment mode from config
   - Skip watcher if mode=prod
   - Use SIGHUP signal instead

4. **Worker restart logic**
   - Start new worker on new port (if binary changed)
   - Health check
   - Switch traffic
   - Stop old worker
   - Update registry with new mtimes

### Phase 6: Deployment Tools (Week 3-4)
1. **Deployment scripts**
   - `scripts/build-prod.sh`: Build with optimizations
   - `scripts/deploy.sh`: rsync-based incremental deployment
   - Support for deploying specific workers or all

2. **Incremental deployment**
   - rsync with --checksum flag
   - Upload only changed files
   - Server detects via mtime changes

3. **Rollback support**
   - Use git to revert source changes
   - Rebuild and redeploy
   - Or restore from backup directory

### Phase 7: Testing and Documentation (Week 4)
1. **Unit tests**
   - Timestamp tracking and comparison
   - Worker registry operations
   - Resource loading from filesystem

2. **Integration tests**
   - Dev mode workflow with file watcher
   - Prod mode workflow with SIGHUP-triggered checking
   - Rolling restart on binary changes
   - Asset reload without restart
   - SIGHUP handling
   - Incremental deployment

3. **Documentation**
   - Updated README with new structure
   - Deployment guide
   - Development guide
   - Migration guide from old structure

## Migration Path

### For Existing Deployments

1. **Create new structure alongside old**:
   ```bash
   mkdir -p workers/index/src
   cp pages/index/*.go workers/index/src/
   mkdir -p workers/index/views
   cp pages/index/*.html workers/index/views/
   mkdir -p workers/index/{config,data,public}
   # ... reorganize other files
   ```

2. **Update build to use new paths**:
   - Build to: `workers/index/bin/index`
   - Keep old binary during transition: `bin/tqworker_index`

3. **Gradual cutover**:
   - Test new structure in dev
   - Deploy to staging
   - Deploy to production
   - Monitor for issues

4. **Remove old structure** once validated

## Benefits

### Development
✅ **Clear organization**: Each worker is self-contained with src/bin/public/private
✅ **Fast builds**: Incremental builds based on source timestamps
✅ **Hot reload**: Immediate restart on file changes
✅ **No complexity**: Direct filesystem operations
✅ **Isolated resources**: Each worker has its own assets
✅ **Simple workflow**: Edit, build, restart automatically

### Production
✅ **Incremental deployments**: Update any file independently
✅ **Simple process**: Build and rsync, server handles rest
✅ **Zero-downtime**: Workers restart on new ports
✅ **Efficient**: Only changed workers restart
✅ **Flexible**: No coordination needed
✅ **On-demand checking**: SIGHUP triggers reload
✅ **Transparent**: File mtimes are standard and reliable

### Operations
✅ **No special tools**: Standard rsync/scp for deployment
✅ **Easy rollback**: Git revert + rebuild + deploy
✅ **Resource efficient**: Only changed workers restart
✅ **Simple monitoring**: Check file mtimes and worker status
✅ **Manual control**: SIGHUP triggers reload when ready

## Security Considerations

### Resource Access
- Workers access resources from `workers/{name}/` directories
- Base path provided via environment variable
- No dynamic path construction (prevents directory traversal)
- Read-only access patterns

### File Integrity
- File mtimes provide tamper detection
- Unexpected mtime changes trigger investigation
- Server logs all worker restarts with reasons

### File Permissions
- `workers/` directory structure with proper permissions
- Separate deployment user from runtime user
- Binaries executable only, not writable by server process

## Performance Considerations

### Deployment Size
- Binary sizes: ~2-10MB per worker
- Assets: Variable (KB to MB)
- Incremental transfers via rsync minimize bandwidth

### Timestamp Checking
- **Dev mode**: Immediate via file watcher
- **Prod mode**: On SIGHUP signal only
- Cost per check: `stat()` calls on binary and asset directories
- Very low overhead (only when triggered)

### Resource Access
- All modes: Filesystem I/O (OS cache optimized)
- No assumptions about template caching (application decides)
- Public assets served with browser cache headers
- Private resources reloaded on change detection

## Future Enhancements

### V2: Advanced Features
- **Checksum validation**: Optional SHA256 checksums for deployments
- **Compression**: Compressed deployment transfers
- **Smart caching**: Worker-level asset caching
- **Canary deployments**: Gradual rollout to subset of workers
- **Blue-green deployment**: Parallel deployment with instant switchover

### V3: Cluster Support
- **Distributed checking**: Share worker states via Redis/etcd
- **Coordinated restarts**: Cluster-wide orchestration
- **Central storage**: Shared filesystem or object storage for binaries
- **Health aggregation**: Cluster health monitoring

## Conclusion

This specification provides a comprehensive plan for organizing workers and resources with timestamp-based change detection. The approach prioritizes simplicity and flexibility over complex coordination.

Key advantages:
- **Simple**: No coordination files or versioned folders
- **Flexible**: Update any file independently
- **Incremental**: Only changed files need deployment
- **Fast**: Minimal overhead for checking
- **Standard**: Uses filesystem mtimes (reliable and universal)
- **Transparent**: Easy to debug and understand
- **Scalable**: Can add more sophisticated features later

The phased implementation allows for incremental development and testing. The timestamp-based approach provides a solid foundation that can be enhanced with checksums, distributed coordination, or other features as needed.

## Future Enhancements

### V2: Advanced Features
- **Checksum validation**: Optional SHA256 checksums for deployments
- **Deployment compression**: Compressed deployment archives
- **Smart caching**: Worker-level asset caching
- **Canary deployments**: Deploy to subset of workers
- **Blue-green deployment**: Parallel deployment with instant switchover

### V3: Cluster Support
- **Distributed state**: Share worker states via Redis/etcd
- **Coordinated restarts**: Cluster-wide orchestration
- **Binary CDN**: Central storage for binaries
- **Health aggregation**: Cluster health monitoring

## Conclusion

This specification provides a comprehensive plan for organizing workers and resources with timestamp-based change detection. The approach prioritizes simplicity and flexibility over complex coordination.

Key advantages:
- **Simple**: No coordination files or versioned folders
- **Flexible**: Update any file independently
- **Incremental**: Only changed files need deployment
- **Fast**: Minimal overhead for checking
- **Standard**: Uses filesystem mtimes (reliable and universal)
- **Transparent**: Easy to debug and understand
- **Scalable**: Can add more sophisticated features later

The phased implementation approach allows for incremental development and testing, while maintaining backwards compatibility during migration. The timestamp-based approach provides a solid foundation for future enhancements like cluster deployments and asset CDNs.











 
 
 


  
 
 

 
 
 

















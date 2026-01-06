# Hot Reload System

- [Introduction](#introduction)
- [How Hot Reload Works](#how-hot-reload-works)
- [File Watching](#file-watching)
- [Build Pipeline](#build-pipeline)
- [Graceful Swap](#graceful-swap)
- [Zero-Downtime Strategy](#zero-downtime-strategy)
- [Performance](#performance)
- [Limitations](#limitations)

## Introduction

TQServer's hot reload system enables **sub-second deployment of code changes** without dropping requests or requiring manual intervention. When you save a file, TQServer automatically detects the change, rebuilds the worker, and gracefully swaps to the new version in ~0.3-1.0 seconds.

```
Save File → Detect (100ms) → Build (200-500ms) → Swap (100-200ms) → Running
Total: 400-800ms with zero dropped requests
```

## How Hot Reload Works

### Overview

```
┌──────────────────────────────────────────────────────────┐
│                   Hot Reload Cycle                        │
│                                                           │
│  1. File Change   →  2. Detection  →  3. Debounce       │
│         ↓                                                 │
│  6. Swap Traffic  ←  5. Health Check  ←  4. Build       │
│         ↓                                                 │
│  7. Old Worker Shutdown  →  8. Cleanup  →  9. Complete   │
└──────────────────────────────────────────────────────────┘
```

### Step-by-Step Process

**1. File Change Detection**
```go
// File watcher detects change
event := fsnotify.Event{
    Name: "workers/api/src/main.go",
    Op:   fsnotify.Write,
}
```

**2. Debouncing**
```go
// Wait for rapid changes to settle
debounceTimer := time.NewTimer(500 * time.Millisecond)
<-debounceTimer.C
```

**3. Build Worker**
```bash
# Compile new version
go build -o workers/api/bin/api.new workers/api/src
```

**4. Start New Instance**
```go
// Start on different port
newPort := portPool.Acquire()  // e.g., 9005
cmd := exec.Command(workerBinary, "--port", newPort)
cmd.Start()
```

**5. Health Check**
```go
// Wait for new worker to be healthy
for i := 0; i < 30; i++ {
    resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", newPort))
    if err == nil && resp.StatusCode == 200 {
        break  // Healthy!
    }
    time.Sleep(100 * time.Millisecond)
}
```

**6. Traffic Swap**
```go
// Atomically swap routing
router.SwapWorker(workerName, oldPort, newPort)
// From this point, new requests go to new worker
```

**7. Graceful Shutdown**
```go
// Signal old worker to shut down
oldWorker.Signal(syscall.SIGTERM)

// Wait for in-flight requests to complete
shutdownTimeout := time.After(30 * time.Second)
select {
case <-oldWorker.Done():
    // Clean shutdown
case <-shutdownTimeout:
    // Force kill if needed
    oldWorker.Signal(syscall.SIGKILL)
}
```

**8. Port Release**
```go
// Return port to pool
portPool.Release(oldPort)
```

**9. Binary Cleanup**
```go
// Rename old binary with timestamp
os.Rename("workers/api/bin/api", "workers/api/bin/api.1704556800")

// Rename new binary to primary
os.Rename("workers/api/bin/api.new", "workers/api/bin/api")
```

## File Watching

### Watched Directories

TQServer monitors:

```
workers/*/src/**/*.go       # Source code
workers/*/private/**/*      # Templates, views
workers/*/public/**/*       # Static assets
config/*.yaml               # Configuration
```

### File Watcher Implementation

```go
package watcher

import (
    "github.com/fsnotify/fsnotify"
    "path/filepath"
    "time"
)

type FileWatcher struct {
    watcher        *fsnotify.Watcher
    debounceMs     int
    changeHandler  func(event ChangeEvent)
    debounceTimers map[string]*time.Timer
}

func (fw *FileWatcher) watchLoop() {
    for {
        select {
        case event := <-fw.watcher.Events:
            if fw.shouldIgnore(event.Name) {
                continue
            }
            
            fw.debounceChange(event)
            
        case err := <-fw.watcher.Errors:
            log.Printf("Watcher error: %v", err)
        }
    }
}

func (fw *FileWatcher) debounceChange(event fsnotify.Event) {
    workerName := fw.extractWorkerName(event.Name)
    
    // Cancel existing timer
    if timer, exists := fw.debounceTimers[workerName]; exists {
        timer.Stop()
    }
    
    // Create new debounce timer
    fw.debounceTimers[workerName] = time.AfterFunc(
        time.Duration(fw.debounceMs)*time.Millisecond,
        func() {
            fw.changeHandler(ChangeEvent{
                Path:       event.Name,
                WorkerName: workerName,
                ChangeType: fw.determineChangeType(event.Name),
            })
        },
    )
}
```

### Ignore Patterns

Files/directories ignored:

```yaml
file_watcher:
  ignore_patterns:
    - "*.log"
    - "*.tmp"
    - ".git/**"
    - "**/bin/**"
    - "**/node_modules/**"
    - "**/.DS_Store"
    - "**/*~"
```

### Debouncing Strategy

**Problem**: Rapid file changes (e.g., save multiple files)

**Solution**: Debounce with configurable delay

```
File1 saved   →  Timer 500ms
File2 saved   →  Reset timer 500ms
File3 saved   →  Reset timer 500ms
(500ms passes) →  Trigger build
```

**Configuration**:
```yaml
workers:
  file_watcher:
    debounce: 500ms  # Development: fast
    # debounce: 2s   # Production: disabled or slow
```

## Build Pipeline

### Build Process

```bash
#!/bin/bash
# Simplified build script

WORKER=$1
SRC="workers/$WORKER/src"
OUT="workers/$WORKER/bin/$WORKER.new"

# Build with all source files
go build -o "$OUT" "$SRC"/*.go

# Check for errors
if [ $? -eq 0 ]; then
    echo "Build successful: $WORKER"
    exit 0
else
    echo "Build failed: $WORKER"
    exit 1
fi
```

### Incremental Builds

Go's compiler is fast, but we can optimize further:

```go
// Build with cache
cmd := exec.Command("go", "build",
    "-o", outputPath,
    "-buildmode=exe",
    sourcePath,
)

// Set build cache
cmd.Env = append(os.Environ(),
    "GOCACHE="+cacheDir,
    "GOMODCACHE="+modCacheDir,
)
```

### Build Optimization

**Development Build** (fast):
```bash
go build -o bin/worker src/*.go
# ~200-500ms for typical worker
```

**Production Build** (optimized):
```bash
go build \
    -ldflags="-s -w" \
    -trimpath \
    -o bin/worker \
    src/*.go
# ~500-800ms, but smaller binary
```

### Build Failure Handling

```go
func buildWorker(workerName string) error {
    cmd := exec.Command("go", "build", ...)
    output, err := cmd.CombinedOutput()
    
    if err != nil {
        log.Printf("Build failed for %s:\n%s", workerName, output)
        
        // Keep old worker running!
        // Don't swap to broken version
        
        return fmt.Errorf("build failed: %w", err)
    }
    
    return nil
}
```

## Graceful Swap

### Overlap Period

During swap, both workers run simultaneously:

```
Time:     0s    0.5s    1s    1.5s    2s
          │      │      │      │      │
Old:      ████████████████░░░░░(done)
New:            ░░░░████████████████████
          │      │      │      │      │
          Start  Ready  Swap   Drain  Complete
                 ↑      ↑
                 Health Swap
                 Check  Traffic
```

### Port Allocation Strategy

**Port Pool Management**:
```go
type PortPool struct {
    available chan int
    allocated map[int]bool
}

// During swap:
oldPort := 9000  // Still in use
newPort := portPool.Acquire()  // Gets 9001

// After old worker shutdown:
portPool.Release(oldPort)  // 9000 back to pool
```

### Traffic Switching

**Atomic Swap**:
```go
type Router struct {
    mu      sync.RWMutex
    workers map[string]*Worker
}

func (r *Router) SwapWorker(name string, newWorker *Worker) {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    // Atomic pointer swap
    // All new requests use newWorker immediately
    r.workers[name] = newWorker
}
```

### Connection Draining

```go
func gracefulShutdown(worker *Worker, timeout time.Duration) {
    // Signal to stop accepting new connections
    worker.Process.Signal(syscall.SIGTERM)
    
    // Wait for existing connections to complete
    done := make(chan struct{})
    go func() {
        worker.Process.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        log.Printf("Worker %s shutdown cleanly", worker.Name)
    case <-time.After(timeout):
        log.Printf("Worker %s force killed after timeout", worker.Name)
        worker.Process.Signal(syscall.SIGKILL)
        worker.Process.Wait()
    }
}
```

## Zero-Downtime Strategy

### Request Flow During Swap

**Before Swap**:
```
Client → TQServer → Router → Old Worker (port 9000)
```

**During Swap** (New worker starting):
```
Client → TQServer → Router → Old Worker (port 9000)
New Worker (port 9001): Starting, health checking...
```

**During Swap** (Both running):
```
New Requests:     Client → TQServer → Router → New Worker (9001)
In-flight:        Client → TQServer → Router → Old Worker (9000)
```

**After Swap**:
```
Client → TQServer → Router → New Worker (port 9001)
Old Worker: Shutting down...
```

### Health Check Validation

```go
func waitForHealthy(worker *Worker, maxWait time.Duration) bool {
    client := &http.Client{Timeout: 2 * time.Second}
    deadline := time.Now().Add(maxWait)
    
    for time.Now().Before(deadline) {
        resp, err := client.Get(
            fmt.Sprintf("http://localhost:%d/health", worker.Port),
        )
        
        if err == nil && resp.StatusCode == 200 {
            resp.Body.Close()
            return true  // Healthy!
        }
        
        time.Sleep(100 * time.Millisecond)
    }
    
    return false  // Failed to become healthy
}
```

### Rollback on Failure

```go
func swapWorker(name string, newWorker *Worker) error {
    // Validate new worker is healthy
    if !waitForHealthy(newWorker, 10*time.Second) {
        newWorker.Kill()
        return errors.New("new worker failed health check")
    }
    
    // Keep reference to old worker
    oldWorker := router.GetWorker(name)
    
    // Swap traffic
    router.SwapWorker(name, newWorker)
    
    // Monitor new worker
    go func() {
        time.Sleep(5 * time.Second)
        if !newWorker.IsHealthy() {
            log.Printf("New worker unhealthy, rolling back!")
            
            // Rollback to old worker
            if oldWorker.Process.ProcessState == nil {
                router.SwapWorker(name, oldWorker)
                newWorker.Kill()
            }
        }
    }()
    
    // Gracefully shutdown old worker
    gracefulShutdown(oldWorker, 30*time.Second)
    return nil
}
```

## Performance

### Reload Timing Breakdown

Typical hot reload timeline:

```
Event                Time      Cumulative
─────────────────────────────────────────
File change          0ms       0ms
Detection            50ms      50ms
Debounce wait        500ms     550ms
Build start          10ms      560ms
Compilation          300ms     860ms
Binary ready         10ms      870ms
Worker start         50ms      920ms
Health checks (3x)   300ms     1220ms
Traffic swap         5ms       1225ms
Old worker drain     100ms     1325ms
Port release         5ms       1330ms
Total:                         ~1.3s
```

**Optimization Opportunities**:
- Reduce debounce (development): 500ms → 100ms
- Incremental builds: 300ms → 150ms
- Parallel health checks: 300ms → 100ms
- **Optimized total: ~400-600ms**

### Build Performance

**Factors Affecting Build Time**:
- Source code size
- Dependencies
- CPU performance
- Disk I/O speed
- Build cache warmth

**Benchmarks**:
```
Small worker (100 LOC):     ~100-200ms
Medium worker (1000 LOC):   ~200-400ms
Large worker (10000 LOC):   ~500-1000ms
With many dependencies:     +200-500ms
```

### Memory Usage During Reload

```
Before:  Old Worker (50MB)
During:  Old Worker (50MB) + New Worker (50MB) = 100MB
After:   New Worker (50MB)

Peak memory usage = 2x normal
```

**Mitigation**:
- Ensure sufficient memory headroom
- Typical overhead: 10-100MB per worker
- Monitor memory during swaps

## Limitations

### When Hot Reload Doesn't Help

**Database Schema Changes**:
```
✗ Cannot hot-reload database structure
→ Require migrations and coordinated deployment
```

**Breaking API Changes**:
```
✗ Cannot hot-reload client contracts
→ Require versioning and gradual rollout
```

**External Service Changes**:
```
✗ Cannot hot-reload third-party integrations
→ Require configuration updates
```

### What Can Be Hot Reloaded

✅ **Source Code**:
- Business logic changes
- Bug fixes
- New features
- Route handlers

✅ **Templates**:
- HTML templates
- Email templates
- View rendering

✅ **Static Assets** (if served by worker):
- CSS files
- JavaScript files
- Images

✅ **Worker Configuration**:
- Timeouts
- Resource limits
- Environment variables

### Production Considerations

**Disable in Production?**
```yaml
# Production config
workers:
  file_watcher:
    enabled: false  # Disable hot reload
```

**Why disable**:
- Intentional deployments only
- Avoid accidental changes
- Controlled rollout process
- Audit trail

**Alternative**: Use CI/CD pipeline for production

## Best Practices

### Development Workflow

```bash
# 1. Start TQServer with hot reload
./bin/tqserver

# 2. Edit worker code
vim workers/api/src/main.go

# 3. Save file
# → Auto-reload in ~500ms

# 4. Test immediately
curl http://localhost:8080/api/test
```

### Handling Build Errors

```go
// Always recover from build failures gracefully
func handleFileChange(workerName string) {
    log.Printf("Rebuilding %s...", workerName)
    
    if err := buildWorker(workerName); err != nil {
        log.Printf("Build failed, keeping old version: %v", err)
        // Old worker keeps running!
        return
    }
    
    if err := swapWorker(workerName); err != nil {
        log.Printf("Swap failed, keeping old version: %v", err)
        return
    }
    
    log.Printf("Successfully reloaded %s", workerName)
}
```

### Health Check Implementation

```go
// Implement comprehensive health checks
func healthHandler(w http.ResponseWriter, r *http.Request) {
    health := HealthStatus{
        Status: "healthy",
        Checks: make(map[string]string),
    }
    
    // Check database
    if err := db.Ping(); err != nil {
        health.Status = "unhealthy"
        health.Checks["database"] = err.Error()
    } else {
        health.Checks["database"] = "ok"
    }
    
    // Check Redis
    if err := redis.Ping(); err != nil {
        health.Status = "unhealthy"
        health.Checks["redis"] = err.Error()
    } else {
        health.Checks["redis"] = "ok"
    }
    
    status := http.StatusOK
    if health.Status == "unhealthy" {
        status = http.StatusServiceUnavailable
    }
    
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(health)
}
```

## Next Steps

- [Supervisor Pattern](supervisor.md) - How supervisor manages reloads
- [Worker Lifecycle](../workers/lifecycle.md) - Worker lifecycle states
- [File Watching](../advanced/file-watching.md) - Deep dive into file watching
- [Graceful Restarts](../advanced/graceful-restarts.md) - Advanced restart patterns

# Migration Guide: Refactored Architecture

This document describes the structural improvements made to TQServer and how to
migrate existing code.

## What Changed

### 1. Project Structure

**Before:**

```
server/
├── config.go
├── main.go
├── proxy.go
├── router.go
└── supervisor.go
```

**After:**

```
cmd/tqserver/main.go          # Entry point
internal/
├── config/config.go          # Configuration (was server/config.go)
├── proxy/proxy.go            # HTTP proxy (was server/proxy.go)
├── router/
│   ├── router.go             # Route management (was server/router.go)
│   └── worker.go             # Worker type extracted
└── supervisor/               # Supervisor package (was server/supervisor.go)
    ├── supervisor.go         # Main logic
    ├── ports.go              # NEW: Port pool management
    ├── healthcheck.go        # NEW: Health checks
    └── cleanup.go            # NEW: Binary cleanup
pkg/worker/runtime.go         # NEW: Shared worker runtime
```

### 2. Configuration Changes

**Field Name Updates** (for consistency):

- `WorkerSettings.GOMAXPROCS` → `WorkerSettings.GoMaxProcs`
- `WorkerSettings.GOMEMLIMIT` → `WorkerSettings.GoMemLimit`

YAML configuration remains backward compatible.

### 3. API Changes

#### Interfaces Introduced

```go
// router/router.go
type RouterInterface interface {
    DiscoverRoutes() error
    GetWorker(path string) *Worker
    GetAllWorkers() []*Worker
    UpdateWorker(route string, worker *Worker)
}

// supervisor/supervisor.go
type SupervisorInterface interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    RestartWorker(worker *router.Worker) error
}
```

#### Context Support

**Before:**

```go
supervisor.Start()
supervisor.Stop()
```

**After:**

```go
ctx := context.Background()
supervisor.Start(ctx)

shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
supervisor.Stop(shutdownCtx)
```

#### Worker Type Location

**Before:**

```go
// Worker was defined in server/router.go
type Worker struct { ... }
```

**After:**

```go
// Worker is now in internal/router/worker.go
import "github.com/mevdschee/tqserver/internal/router"

worker := &router.Worker{ ... }
```

### 4. New Features

#### Port Pool Management

Ports are now managed through a pool instead of simple increment:

```go
// internal/supervisor/ports.go
portPool := NewPortPool(startPort, endPort)
port, err := portPool.Acquire()
// ... use port ...
portPool.Release(port)
```

#### Health Checks

Workers are periodically checked for health via HTTP:

```go
// Automatic health checks every 5 seconds
// Workers must respond to GET /health with 200 OK
```

#### Binary Cleanup

Old worker binaries are automatically cleaned up:

```go
// Runs every hour
// Removes binaries older than 24 hours from temp directory
```

## Migrating Worker Code

### Before (Manual Initialization)

```go
package main

import (
    "net/http"
    "os"
    "strconv"
    "time"
)

func main() {
    port := os.Getenv("PORT")
    if port == "" {
        port = "9000"
    }

    readTimeout := 30 * time.Second
    if val := os.Getenv("READ_TIMEOUT_SECONDS"); val != "" {
        if n, err := strconv.Atoi(val); err == nil && n > 0 {
            readTimeout = time.Duration(n) * time.Second
        }
    }

    // ... more timeout parsing ...

    http.HandleFunc("/", handler)

    server := &http.Server{
        Addr:         ":" + port,
        ReadTimeout:  readTimeout,
        WriteTimeout: writeTimeout,
        IdleTimeout:  idleTimeout,
    }

    server.ListenAndServe()
}
```

### After (Using Worker Runtime)

```go
package main

import (
    "net/http"
    
    "github.com/mevdschee/tqserver/pkg/worker"
)

func main() {
    // All environment parsing handled automatically
    runtime := worker.NewRuntime()

    http.HandleFunc("/", handler)
    
    // Health check is recommended
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    })

    // Start server with configured timeouts
    runtime.StartServer(nil)
}
```

**Benefits:**

- Less boilerplate code
- Consistent timeout parsing
- Access to runtime info via `runtime.Port`, `runtime.Route`

## Building

### Before

```bash
go build -o bin/tqserver ./server
```

### After

```bash
go build -o bin/tqserver ./cmd/tqserver
```

## Import Paths

If you were importing server components:

**Before:**

```go
import "github.com/mevdschee/tqserver/server"
```

**After:**

```go
import (
    "github.com/mevdschee/tqserver/internal/config"
    "github.com/mevdschee/tqserver/internal/router"
    "github.com/mevdschee/tqserver/internal/proxy"
    "github.com/mevdschee/tqserver/internal/supervisor"
    "github.com/mevdschee/tqserver/pkg/worker"  // For pages
)
```

Note: `internal/` packages should only be imported within this project, not by
external projects.

## Backward Compatibility

- The `server/` directory still exists with original files as backup
- Configuration files are fully compatible
- Worker binaries don't need changes (but should add `/health` endpoint)
- Runtime behavior is identical

## Next Steps

1. Update your worker pages to use `pkg/worker` runtime (optional but
   recommended)
2. Add `/health` endpoints to workers for health monitoring
3. Test the new structure with your existing configuration
4. Remove the old `server/` directory once confident in the migration

## Benefits of New Structure

1. **Better Organization**: Clear separation of concerns with proper Go project
   layout
2. **Testability**: Interface-based design enables mocking and unit testing
3. **Maintainability**: Smaller, focused files are easier to understand and
   modify
4. **Resource Management**: Port pool prevents port exhaustion issues
5. **Observability**: Health checks provide visibility into worker status
6. **Cleaner Code**: Shared runtime eliminates duplicate initialization code
7. **Scalability**: Better foundation for future enhancements

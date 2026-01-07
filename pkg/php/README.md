# PHP Package

Go-based PHP-CGI process management for TQServer. This package provides a complete alternative to PHP-FPM with superior process management, health monitoring, and developer experience.

## Features

- ✅ **Auto-detection** of php-cgi binary
- ✅ **Version parsing** and feature detection
- ✅ **Flexible configuration** via php.ini + CLI overrides
- ✅ **Three pool management strategies**: static, dynamic, ondemand
- ✅ **Automatic crash recovery** with replacement spawning
- ✅ **Health monitoring** and statistics collection
- ✅ **Worker recycling** based on max requests
- ✅ **Graceful shutdown** with timeout and force-kill fallback
- ✅ **Process state tracking**: idle, active, terminating, crashed
- ✅ **Stdout/stderr capture** for logging

## Quick Start

```go
package main

import (
    "log"
    "time"
    "github.com/mevdschee/tqserver/pkg/php"
)

func main() {
    // Create configuration
    config := &php.Config{
        Binary:       "",  // Auto-detect
        DocumentRoot: "/var/www/html",
        Settings: map[string]string{
            "memory_limit": "128M",
            "display_errors": "1",
        },
        Pool: php.PoolConfig{
            Manager:        "static",
            MaxWorkers:     4,
            RequestTimeout: 30 * time.Second,
            UnixSocket:     "/tmp/tqserver-php.sock",
        },
    }

    // Detect PHP binary
    binary, err := php.DetectBinary(config.Binary)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Found PHP: %s", binary.String())

    // Create and start manager
    manager, err := php.NewManager(binary, config)
    if err != nil {
        log.Fatal(err)
    }

    if err := manager.Start(); err != nil {
        log.Fatal(err)
    }

    log.Println("PHP workers running!")

    // Use workers...

    // Graceful shutdown
    manager.Stop()
}
```

## Configuration

### Pool Managers

#### Static Pool

Fixed number of workers, all spawned at startup.

```go
Pool: php.PoolConfig{
    Manager:    "static",
    MaxWorkers: 4,  // Always maintain 4 workers
}
```

**Best for:**
- Predictable traffic patterns
- Consistent resource allocation
- Lowest latency (no spawning delay)

#### Dynamic Pool

Variable worker count within min/max bounds.

```go
Pool: php.PoolConfig{
    Manager:      "dynamic",
    MinWorkers:   2,    // Always keep at least 2
    MaxWorkers:   10,   // Never exceed 10
    StartWorkers: 4,    // Start with 4
    IdleTimeout:  10 * time.Second,
}
```

**Best for:**
- Variable traffic patterns
- Resource efficiency
- Handling traffic spikes

**Behavior:**
- Scales up when < 2 idle workers
- Scales down when > 4 idle workers (respects min_workers)
- Kills workers idle > IdleTimeout

#### Ondemand Pool

Zero workers at startup, spawn on-demand.

```go
Pool: php.PoolConfig{
    Manager:     "ondemand",
    MaxWorkers:  5,     // Maximum 5 concurrent workers
    IdleTimeout: 10 * time.Second,
}
```

**Best for:**
- Low-traffic sites
- Development environments
- Minimal resource usage

**Behavior:**
- Starts with 0 workers
- Spawns worker when GetIdleWorker() called
- Aggressively kills idle workers

### PHP Settings

#### Base Configuration

Use an existing php.ini file as a base:

```go
config := &php.Config{
    ConfigFile: "/etc/php/8.2/php.ini",
    // ...
}
```

#### Individual Overrides

Override specific settings via map:

```go
config := &php.Config{
    Settings: map[string]string{
        "memory_limit":       "256M",
        "max_execution_time": "60",
        "upload_max_filesize": "20M",
        "post_max_size":      "20M",
        "display_errors":     "0",
        "error_reporting":    "E_ALL & ~E_DEPRECATED",
        "opcache.enable":     "1",
        "opcache.memory_consumption": "128",
    },
}
```

These are passed as `-d` flags to php-cgi.

### Worker Limits

```go
Pool: php.PoolConfig{
    MaxRequests:    1000,  // Restart worker after 1000 requests
    RequestTimeout: 30 * time.Second,  // Kill worker if request > 30s
    IdleTimeout:    60 * time.Second,  // Kill idle workers (dynamic/ondemand)
}
```

## API Reference

### Binary Detection

```go
// Auto-detect php-cgi in PATH
binary, err := php.DetectBinary("")

// Use specific path
binary, err := php.DetectBinary("/usr/local/bin/php-cgi")

// Check version
fmt.Printf("PHP %d.%d.%d\n", binary.Major, binary.Minor, binary.Patch)

// Check feature support
if binary.SupportsFeature("jit") {
    fmt.Println("JIT is available!")
}
```

### Manager Operations

```go
// Create manager
manager, err := php.NewManager(binary, config)

// Start workers
err = manager.Start()

// Get idle worker for request processing
worker, err := manager.GetIdleWorker()
// ... process request ...
manager.ReleaseWorker(worker)

// Get statistics
stats := manager.GetStats()
// stats = map[string]interface{}{
//     "total_workers": 4,
//     "idle_workers": 3,
//     "active_workers": 1,
//     "total_requests": 1523,
//     "total_restarts": 5,
//     "pool_manager": "static",
// }

// Get detailed worker info
for _, info := range manager.GetWorkerInfo() {
    fmt.Printf("Worker %d: %s\n", info["id"], info["state"])
}

// Graceful shutdown
err = manager.Stop()
```

### Worker States

```go
const (
    WorkerStateIdle        // Ready to accept requests
    WorkerStateActive      // Currently processing request
    WorkerStateTerminating // Shutting down
    WorkerStateCrashed     // Process exited unexpectedly
)

// Check worker state
if worker.GetState() == php.WorkerStateIdle {
    worker.MarkActive()
    // ... process request ...
    worker.MarkIdle()
}

// Get worker stats
fmt.Printf("Requests: %d\n", worker.GetRequestCount())
fmt.Printf("Uptime: %s\n", worker.GetUptime())
fmt.Printf("Idle time: %s\n", worker.GetIdleTime())
```

## Architecture

### Process Hierarchy

```
Manager
├─> Worker 1 (php-cgi process)
│   ├─> monitor goroutine
│   ├─> stdout handler goroutine
│   └─> stderr handler goroutine
├─> Worker 2 (php-cgi process)
│   └─> ...
└─> Health monitor goroutine
```

### Lifecycle

```
Manager.Start()
  └─> spawnWorker() × N
       └─> Worker.Start()
            └─> exec.CommandContext(php-cgi -b socket -d settings...)
                 ├─> monitor() goroutine → watch for crash
                 ├─> handleOutput(stdout) → log output
                 └─> handleOutput(stderr) → log errors

Health Monitor (5s interval)
  ├─> performHealthCheck() → restart unhealthy workers
  └─> managePoolSize() → scale up/down (dynamic/ondemand)

Request Processing
  ├─> manager.GetIdleWorker() → finds or spawns worker
  ├─> worker.MarkActive()
  ├─> ... forward request to php-cgi ...
  ├─> worker.MarkIdle()
  └─> manager.ReleaseWorker(worker)

Manager.Stop()
  └─> worker.Stop() × N
       ├─> context cancel
       └─> wait or kill (5s timeout)
```

## Examples

### Static Pool (Production)

```go
config := &php.Config{
    Binary:       "",
    DocumentRoot: "/var/www/html",
    ConfigFile:   "/etc/php/8.2/php.ini",
    Settings: map[string]string{
        "memory_limit":   "256M",
        "display_errors": "0",
    },
    Pool: php.PoolConfig{
        Manager:        "static",
        MaxWorkers:     8,
        MaxRequests:    5000,
        RequestTimeout: 60 * time.Second,
        UnixSocket:     "/var/run/tqserver-php.sock",
    },
}
```

### Dynamic Pool (Variable Traffic)

```go
config := &php.Config{
    Binary:       "",
    DocumentRoot: "/var/www/html",
    Pool: php.PoolConfig{
        Manager:        "dynamic",
        MinWorkers:     4,
        MaxWorkers:     20,
        StartWorkers:   8,
        MaxRequests:    1000,
        RequestTimeout: 30 * time.Second,
        IdleTimeout:    60 * time.Second,
        UnixSocket:     "/var/run/tqserver-php.sock",
    },
}
```

### Ondemand Pool (Development)

```go
config := &php.Config{
    Binary:       "",
    DocumentRoot: "./public",
    Settings: map[string]string{
        "display_errors":  "1",
        "error_reporting": "E_ALL",
    },
    Pool: php.PoolConfig{
        Manager:        "ondemand",
        MaxWorkers:     2,
        RequestTimeout: 30 * time.Second,
        IdleTimeout:    10 * time.Second,
        UnixSocket:     "/tmp/tqserver-php.sock",
    },
}
```

## Testing

Run tests:

```bash
go test ./pkg/php/... -v
```

Note: Some tests require php-cgi to be installed and will be skipped if not available.

## Performance

### Benchmarks

(To be added after integration testing)

### Best Practices

1. **Static pool** for consistent traffic (lowest latency)
2. **Dynamic pool** for variable traffic (resource efficient)
3. **Ondemand pool** for development/low-traffic
4. Set **MaxRequests** to prevent memory leaks (1000-5000 recommended)
5. Set **RequestTimeout** to prevent hanging workers (30-60s)
6. Use **ConfigFile** for base settings, **Settings** for overrides
7. Monitor **GetStats()** periodically for health insights

## Roadmap

- [x] Binary detection and version parsing
- [x] Worker process spawning
- [x] State management and monitoring
- [x] Static/dynamic/ondemand pool managers
- [x] Automatic crash recovery
- [x] Health checks and statistics
- [ ] FastCGI request forwarding (Phase 1 integration)
- [ ] Unix socket support
- [ ] Advanced metrics (request latency, throughput)
- [ ] Worker affinity/sticky sessions
- [ ] Slow request logging
- [ ] OPcache management

## License

Part of TQServer project.

# TQServer

A high-performance function execution platform built with Go that provides
sub-second hot reloads with native Go performance.

## Features

- **Sub-second hot reloads** - Changes to page code are automatically detected,
  rebuilt, and deployed in ~0.3-1.0 seconds
- **Filesystem-based routing** - URL structure mirrors your filesystem layout
- **Graceful worker restarts** - Zero-downtime deployments with automatic
  traffic switching
- **Native Go performance** - Workers are compiled Go binaries, not interpreted
  scripts
- **Process isolation** - Each route runs in its own process
- **Automatic builds** - File watching and automatic compilation on changes
- **Port pool management** - Efficient port allocation prevents port exhaustion
- **Health monitoring** - Periodic HTTP health checks on worker processes
- **Binary cleanup** - Automatic removal of old compiled binaries (24+ hours
  old)
- **Per-route configuration** - Customize resources, timeouts, and limits per
  route
- **Configuration hot reload** - Automatically detect and apply configuration
  changes without server restart
- **Structured logging** - Server and worker logs with date-based rotation
- **Quiet mode** - Suppress console output for production deployments

## Missing Features / To Be Implemented

The following features are planned but not yet implemented:

- **TLS/HTTPS Support** - Currently only HTTP is supported; need to add SSL/TLS
  certificate configuration
- **Metrics & Monitoring** - Prometheus/OpenTelemetry integration for
  observability (request counts, latencies, worker status)
- **Middleware Support** - Global and per-route middleware for authentication,
  rate limiting, CORS, etc.
- **WebSocket Support** - Currently only HTTP is supported; WebSocket proxying
  needs implementation
- **Static File Serving** - Efficient serving of static assets (CSS, JS, images)
  without worker overhead
- **Request Logging** - Access logs with configurable format (Common Log Format,
  JSON, etc.)
- **Correlation ID** - To log over multiple workers (and remote API's)
- **Load Balancing** - Multiple worker instances per route for horizontal
  scaling
- **Circuit Breaker** - Automatic failure detection and traffic routing around
  unhealthy workers
- **Docker Support** - Containerization with multi-stage builds and compose
  files
- **Graceful Shutdown Improvements** - Better handling of in-flight requests
  during shutdown
- **Worker Pooling** - Reuse worker processes instead of rebuilding for every
  change
- **Template Caching** - Cache compiled templates for better performance
- **Rate Limiting** - Per-route and global rate limiting capabilities, also per
  IP
- **Authentication Middleware** - Built-in JWT, OAuth, or API key authentication
- **Database Connection Pooling** - Shared database connection management across
  workers
- **Background Job Support** - Async task processing and job queues
- **Admin Dashboard** - Web UI for monitoring workers, logs, and system health
- **Testing Framework** - Unit and integration testing utilities for worker
  functions
- **CLI Improvements** - Better command-line interface with subcommands (start,
  stop, reload, status)
- **Log Rotation** - Ensure log files are automatically compressed and/or
  removed
- **Cluster support** - See also Load Balancing, but also take deployment into
  account
- **State management** - Database and cache should be suported
- **Session management** - Session storage should be suported

## Quick Start

### 1. Build the server

```bash
go build -o bin/tqserver ./cmd/tqserver
```

### 2. Configure (optional)

Edit `config/server.yaml` to customize:

- Server port (default: 8080)
- Worker port range (default: 9000-9999)
- Timeouts (read, write, idle)
- Worker startup and restart delays
- Pages directory location

### 3. Run the server

```bash
bin/tqserver
```

The server will listen on port **8080** by default (or as configured) and serve
pages from the `pages/` directory.

Visit http://localhost:8080 to see it in action!

### 4. Edit and watch hot reload

Edit `pages/index/main.go` and save. The server will automatically rebuild and
reload in under 1 second with zero downtime.

## Command Line Options

```bash
bin/tqserver [options]

Options:
  -config string
        Path to config file (default "config/server.yaml")
  -quiet
        Suppress log output to stdout/stderr
```

The `-quiet` flag is useful for production environments where you want logs only
written to files.

## Configuration

The server uses a YAML configuration file (`config/server.yaml`) with the
following options:

```yaml
# Server settings
server:
  port: 8080 # HTTP server port
  read_timeout_seconds: 30 # HTTP read timeout
  write_timeout_seconds: 30 # HTTP write timeout
  idle_timeout_seconds: 120 # HTTP idle timeout
  log_file: "logs/server_{date}.log" # Server log file (use ~, null, or empty to disable)

# Worker settings
workers:
  temp_dir: "/tmp/tqserver/bin" # Directory for temporary worker binaries

  # Port range for worker processes
  port_range_start: 9000 # First port for workers
  port_range_end: 9999 # Last port for workers

  # Timing settings (in milliseconds)
  startup_delay_ms: 100 # Wait time after starting worker
  restart_delay_ms: 100 # Delay before stopping old worker
  shutdown_grace_period_ms: 500 # Grace period for shutdown

  # Default per-worker settings (applies to all workers unless overridden)
  default:
    go_max_procs: 1 # CPU threads (0 = NumCPU)
    max_requests: 0 # Restart worker after N requests (0 = unlimited)
    read_timeout_seconds: 30 # HTTP read timeout for workers
    write_timeout_seconds: 30 # HTTP write timeout for workers
    idle_timeout_seconds: 120 # HTTP idle timeout for workers
    go_mem_limit: "" # Memory limit (e.g., "512MiB", "2GiB", empty = unlimited)
    log_file: "logs/{path}/worker_{date}.log" # Worker log file template

  # Per-path worker overrides (optional)
  paths:
    "/api":
      go_max_procs: 2
      max_requests: 5000
      read_timeout_seconds: 15
      write_timeout_seconds: 15
      idle_timeout_seconds: 60
      go_mem_limit: "256MiB"

# File watching settings
file_watcher:
  debounce_ms: 50 # Debounce for file changes

# Pages directory
pages:
  directory: "pages" # Pages directory path (relative or absolute)
```

### Configuration Options Reference

#### Server Settings

| Option                  | Type   | Default                  | Description                                                                                                    |
| ----------------------- | ------ | ------------------------ | -------------------------------------------------------------------------------------------------------------- |
| `port`                  | int    | 8080                     | HTTP server listening port                                                                                     |
| `read_timeout_seconds`  | int    | 30                       | Maximum time to read request                                                                                   |
| `write_timeout_seconds` | int    | 30                       | Maximum time to write response                                                                                 |
| `idle_timeout_seconds`  | int    | 120                      | Keep-alive timeout                                                                                             |
| `log_file`              | string | `logs/server_{date}.log` | Server log file path. Supports `{date}` placeholder. Use `~`, `null`, or empty string to disable file logging. |

#### Worker Settings

| Option                     | Type   | Default             | Description                                        |
| -------------------------- | ------ | ------------------- | -------------------------------------------------- |
| `temp_dir`                 | string | `/tmp/tqserver/bin` | Directory for compiled worker binaries             |
| `port_range_start`         | int    | 9000                | First port in worker port pool                     |
| `port_range_end`           | int    | 9999                | Last port in worker port pool                      |
| `startup_delay_ms`         | int    | 100                 | Delay after starting worker before routing traffic |
| `restart_delay_ms`         | int    | 100                 | Delay before stopping old worker during restart    |
| `shutdown_grace_period_ms` | int    | 500                 | Time allowed for graceful shutdown                 |

#### Worker Default Settings

These settings apply to all workers unless overridden in `paths`:

| Option                  | Type   | Default                         | Description                                                                                                 |
| ----------------------- | ------ | ------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| `go_max_procs`          | int    | 1                               | Sets Go's GOMAXPROCS (CPU threads). 0 = NumCPU                                                              |
| `max_requests`          | int    | 0                               | Restart worker after N requests. 0 = unlimited                                                              |
| `read_timeout_seconds`  | int    | 30                              | HTTP read timeout for worker                                                                                |
| `write_timeout_seconds` | int    | 30                              | HTTP write timeout for worker                                                                               |
| `idle_timeout_seconds`  | int    | 120                             | HTTP idle timeout for worker                                                                                |
| `go_mem_limit`          | string | ""                              | Go's GOMEMLIMIT (e.g., "512MiB", "2GiB"). Empty = unlimited                                                 |
| `log_file`              | string | `logs/{path}/worker_{date}.log` | Worker log file template. Supports `{path}` and `{date}` placeholders. Use `~`, `null`, or empty to disable |

#### File Watcher Settings

| Option        | Type | Default | Description                                                      |
| ------------- | ---- | ------- | ---------------------------------------------------------------- |
| `debounce_ms` | int  | 50      | Debounce delay to avoid multiple rebuilds for rapid file changes |

#### Pages Settings

| Option      | Type   | Default | Description                                                 |
| ----------- | ------ | ------- | ----------------------------------------------------------- |
| `directory` | string | `pages` | Pages directory path (relative to project root or absolute) |

### Per-Path Worker Configuration

You can configure different resource limits for specific routes using the
`workers.paths` section. This allows you to:

- Limit resource usage for certain endpoints (e.g., public APIs)
- Allocate more resources to critical paths (e.g., webhooks)
- Apply different restart policies per route

**Example:**

```yaml
workers:
  default:
    go_max_procs: 1
    max_requests: 0
    go_mem_limit: "512MiB"

  paths:
    "/api": # More conservative limits for API endpoints
      go_max_procs: 2
      max_requests: 5000
      read_timeout_seconds: 15
      write_timeout_seconds: 15
      go_mem_limit: "256MiB"

    "/webhooks": # More generous for webhooks
      go_max_procs: 1
      max_requests: 20000
      go_mem_limit: "1GiB"
```

Path matching uses the most specific prefix match:

- Exact matches take priority: `/api` exactly matches `/api`
- Prefix matches work: `/api` matches `/api/users`, `/api/posts`, etc.
- Falls back to default settings if no match found

## Configuration Hot Reload

TQServer automatically detects changes to the configuration file and applies
them without requiring a server restart. When you modify `config/server.yaml`
and save:

1. The configuration is automatically reloaded
2. All workers are gracefully restarted with the new settings
3. New configuration values take effect (timeouts, resource limits, port ranges,
   etc.)
4. The server continues handling requests throughout the reload process

This feature is useful for:

- Adjusting worker resource limits in production
- Enabling/disabling features without downtime
- Testing different configurations during development
- Applying security updates to timeouts and limits

**Note:** Changes to the server port (`server.port`) require a manual server
restart as the HTTP server cannot rebind to a new port while running.

**Example workflow:**

```bash
# Edit config file
vim config/server.yaml

# Save changes - TQServer automatically detects and applies them
# Watch the logs to see the reload in action:
# "Config file changed: config/server.yaml"
# "Reloading configuration..."
# "✅ Configuration reloaded successfully"
# "✅ All workers restarted with new configuration"
```

## Documentation

See [project_brief.md](project_brief.md) for complete architecture
documentation.

## Worker Development

### Worker Runtime Package

Workers use the `pkg/worker` package for initialization and HTTP server
configuration. This package provides consistent environment variable parsing and
server setup across all workers.

**Basic Worker Example:**

```go
package main

import (
    "log"
    "net/http"

    "tqserver/pkg/worker"
)

func main() {
    // Initialize worker runtime (reads TQ_* environment variables)
    runtime, err := worker.NewRuntime()
    if err != nil {
        log.Fatal(err)
    }

    // Set up routes
    http.HandleFunc("/", handleIndex)
    http.HandleFunc("/health", handleHealth)

    // Start server with proper timeouts and graceful shutdown
    if err := runtime.StartServer(http.DefaultServeMux); err != nil {
        log.Fatal(err)
    }
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("Hello from worker!"))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}
```

### Worker Runtime API

The `worker.Runtime` struct provides:

```go
type Runtime struct {
    Port             int    // Worker port (from TQ_PORT)
    MaxRequests      int    // Max requests before restart (from TQ_MAX_REQUESTS)
    ReadTimeout      int    // HTTP read timeout in seconds (from TQ_READ_TIMEOUT_SECONDS)
    WriteTimeout     int    // HTTP write timeout in seconds (from TQ_WRITE_TIMEOUT_SECONDS)
    IdleTimeout      int    // HTTP idle timeout in seconds (from TQ_IDLE_TIMEOUT_SECONDS)
    ShutdownTimeout  int    // Graceful shutdown timeout (from TQ_SHUTDOWN_GRACE_PERIOD_MS)
    LogFile          string // Log file path (from TQ_LOG_FILE)
    requestCount     int32  // Atomic request counter
    shutdownRequested int32 // Atomic shutdown flag
}
```

**Methods:**

- `NewRuntime() (*Runtime, error)` - Parse environment and create runtime
- `StartServer(handler http.Handler) error` - Start HTTP server with
  configuration
- `GetRequestCount() int32` - Get current request count
- `ShouldShutdown() bool` - Check if max requests reached

### Health Check Endpoint

All workers **must** implement a `/health` endpoint that returns HTTP 200 OK.
The supervisor uses this endpoint to:

- Determine when a worker is ready after startup
- Monitor worker health continuously (every 30 seconds)
- Mark workers as unhealthy if checks fail (3 consecutive failures)

```go
http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
})
```

### Worker Environment Variables

The supervisor automatically sets these environment variables for each worker:

| Variable                      | Description                                |
| ----------------------------- | ------------------------------------------ |
| `TQ_PORT`                     | Worker listening port                      |
| `TQ_MAX_REQUESTS`             | Max requests before restart (0 = disabled) |
| `TQ_READ_TIMEOUT_SECONDS`     | HTTP read timeout                          |
| `TQ_WRITE_TIMEOUT_SECONDS`    | HTTP write timeout                         |
| `TQ_IDLE_TIMEOUT_SECONDS`     | HTTP idle timeout                          |
| `TQ_SHUTDOWN_GRACE_PERIOD_MS` | Graceful shutdown timeout                  |
| `TQ_LOG_FILE`                 | Log file path                              |
| `GOMAXPROCS`                  | Go CPU thread limit                        |
| `GOMEMLIMIT`                  | Go memory limit                            |

Workers can access these via `worker.NewRuntime()` or directly from
`os.Getenv()`.

## Project Structure

```
tqserver/
├── cmd/
│   └── tqserver/           # Main application entry point
│       └── main.go
├── internal/               # Private application code
│   ├── config/             # Configuration management
│   │   └── config.go
│   ├── proxy/              # HTTP reverse proxy
│   │   └── proxy.go
│   ├── router/             # Route discovery and worker tracking
│   │   ├── router.go
│   │   └── worker.go
│   └── supervisor/         # Worker lifecycle management
│       ├── supervisor.go   # Main supervisor logic
│       ├── ports.go        # Port pool management
│       ├── healthcheck.go  # Health monitoring
│       └── cleanup.go      # Binary cleanup
├── pkg/
│   └── worker/             # Public worker runtime library
│       └── runtime.go      # Shared worker initialization
├── config/
│   ├── server.yaml         # Active configuration
│   └── server.example.yaml # Example configuration
├── pages/                  # Worker pages directory
│   └── index/
│       ├── main.go         # Worker entry point
│       ├── index.html      # Page template
│       └── hello.html      # Additional template
└── templates/
    └── base.html           # Shared template base
```

## Cluster Deployment Architecture

### Scaling from Single-Node to Multi-Node

The transition from single-node to multi-node deployment requires additional
infrastructure components.

**Key additions for cluster mode:**

### 1. Shared Code Distribution

**Challenge:** Updated function code must reach all cluster nodes.

**Solution:** Git pull on all nodes and rebuild binaries on each node.

**Implementation:**

- Each node maintains a local Git repository
- Nodes pull from a central repository when notified
- Notifications trigger rebuild and restart process

**Recommendation:** This is the simplest approach for code distribution.

### 2. Shared Configuration

**Challenge:** Configuration and views must be consistent across all nodes.

**Solution:** Read config and views during runtime, distribute with the code.

**Implementation:**

- Configuration file distributed alongside code
- Version control ensures consistency
- Updates deployed atomically with code changes

**Recommendation:** This is the simplest approach for configuration management.

### 3. Optional: Distributed Cache

**Purpose:** Share state and cached data across cluster nodes for improved
performance and consistency.

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
```

## Project Structure

The project follows Go's standard project layout for better modularity and
maintainability:

```
tqserver/
├── cmd/
│   └── tqserver/          # Application entry point
│       └── main.go
├── internal/              # Private application code
│   ├── config/            # Configuration management
│   │   └── config.go
│   ├── proxy/             # HTTP reverse proxy
│   │   └── proxy.go
│   ├── router/            # Route discovery and worker management
│   │   ├── router.go
│   │   └── worker.go
│   └── supervisor/        # Worker lifecycle management
│       ├── supervisor.go  # Main supervisor logic
│       ├── ports.go       # Port pool management
│       ├── healthcheck.go # Worker health monitoring
│       └── cleanup.go     # Binary cleanup
├── pkg/                   # Public, reusable packages
│   └── worker/            # Common worker runtime
│       └── runtime.go
├── pages/                 # User application code
│   └── index/
│       ├── main.go        # Worker implementation
│       └── *.html         # Templates
├── config/                # Configuration files
│   └── server.yaml
└── templates/             # Shared templates
```

### Architecture Improvements

- **Proper Package Structure**: Separated into `cmd/`, `internal/`, and `pkg/`
  following Go conventions
- **Interface-Based Design**: Key components implement interfaces for better
  testability
- **Port Pool Management**: Efficient port allocation/deallocation instead of
  naive increment
- **Health Checks**: Periodic HTTP health checks on worker processes
- **Binary Cleanup**: Automatic cleanup of old compiled binaries
- **Context Support**: Proper cancellation and graceful shutdown with timeouts
- **Shared Worker Runtime**: Common initialization code in `pkg/worker` reduces
  boilerplate

## Command Line Options

```bash
bin/tqserver [options]

Options:
  -config string
        Path to config file (default "config/server.yaml")
```

## Configuration

The server uses a YAML configuration file (`config/server.yaml`) with the
following options:

```yaml
server:
    port: 8080 # HTTP server port
    read_timeout_seconds: 30 # HTTP read timeout
    write_timeout_seconds: 30 # HTTP write timeout
    idle_timeout_seconds: 120 # HTTP idle timeout

workers:
    port_range_start: 9000 # First port for workers
    port_range_end: 9999 # Last port for workers
    startup_delay_ms: 100 # Wait time after starting worker
    restart_delay_ms: 100 # Delay before stopping old worker
    shutdown_grace_period_ms: 500 # Grace period for shutdown

    # Default per-worker settings (applies to all workers unless overridden)
    default:
        gomaxprocs: 1 # CPU threads (0 = NumCPU)
        max_requests: 0 # Restart worker after N requests (0 = unlimited)
        read_timeout_seconds: 30 # HTTP read timeout for workers
        write_timeout_seconds: 30 # HTTP write timeout for workers
        idle_timeout_seconds: 120 # HTTP idle timeout for workers
        gomemlimit: "" # Memory limit (e.g., "512MiB", empty = unlimited)
        log_file: "logs/{path}/worker_{date}.log" # Log file path template

    # Per-path worker overrides (optional)
    paths:
        "/api":
            gomaxprocs: 2
            max_requests: 5000
            read_timeout_seconds: 15
            write_timeout_seconds: 15
            idle_timeout_seconds: 60
            gomemlimit: "256MiB"

file_watcher:
    debounce_ms: 50 # Debounce for file changes

pages:
    directory: "pages" # Pages directory path
```

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
        gomaxprocs: 1
        max_requests: 0
        gomemlimit: "512MiB"

    paths:
        "/api": # More conservative limits for API endpoints
            gomaxprocs: 2
            max_requests: 5000
            read_timeout_seconds: 15
            write_timeout_seconds: 15
            gomemlimit: "256MiB"

        "/webhooks": # More generous for webhooks
            gomaxprocs: 1
            max_requests: 20000
            gomemlimit: "1GiB"
```

Path matching uses the most specific prefix match:

- Exact matches take priority: `/api` exactly matches `/api`
- Prefix matches work: `/api` matches `/api/users`, `/api/posts`, etc.
- Falls back to default settings if no match found

## Documentation

See [project_brief.md](project_brief.md) for complete architecture
documentation.

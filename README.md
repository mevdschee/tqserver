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
cd server
go build -o ../tqserver
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
./tqserver
```

The server will listen on port **8080** by default (or as configured) and serve
pages from the `pages/` directory.

Visit http://localhost:8080 to see it in action!

### 4. Edit and watch hot reload

Edit `pages/index/main.go` and save. The server will automatically rebuild and
reload in under 1 second with zero downtime.

## Command Line Options

```bash
./tqserver [options]

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

    # Per-worker settings
    num_procs: 1 # Number of processes per worker route
    max_requests: 0 # Restart worker after N requests (0 = unlimited)
    request_timeout_seconds: 30 # Request timeout for workers
    idle_timeout_seconds: 120 # Idle timeout for workers
    memory_limit_mb: 0 # Memory limit per worker in MB (0 = unlimited)

file_watcher:
    debounce_ms: 50 # Debounce for file changes

pages:
    directory: "pages" # Pages directory path
```

## Documentation

See [project_brief.md](project_brief.md) for complete architecture
documentation.

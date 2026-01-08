# Configuration

- [Introduction](#introduction)
- [Server Configuration](#server-configuration)
- [Worker Configuration](#worker-configuration)
- [Port Pool Configuration](#port-pool-configuration)
- [Health Check Configuration](#health-check-configuration)
- [Logging Configuration](#logging-configuration)
- [Environment-Based Configuration](#environment-based-configuration)
- [Configuration Caching](#configuration-caching)

## Introduction

TQServer uses YAML-based configuration files for both server-level and worker-level settings. Configuration changes are automatically detected and applied without requiring a server restart.

## Server Configuration

The main server configuration file is located at `config/server.yaml`:

```yaml
# Server settings
server:
  port: 3000
  read_timeout_seconds: 60
  write_timeout_seconds: 60
  idle_timeout_seconds: 180
  log_file: "logs/tqserver_{date}.log"

# Worker settings
workers:
  directory: "workers"
  port_range_start: 10000
  port_range_end: 19999
  startup_delay_ms: 200
  restart_delay_ms: 200
  shutdown_grace_period_ms: 1000

# File watching settings
file_watcher:
  debounce_ms: 100
```

### Server Options

#### port
The HTTP port for the server.

```yaml
server:
  port: 3000  # Default: 8080
```

#### log_file
Path to the server log file. Use `{date}` placeholder for date-based rotation.

```yaml
server:
  log_file: "logs/tqserver_{date}.log"
  # log_file: "~"  # Disable file logging
```

#### Timeout Settings (in seconds)

```yaml
server:
  read_timeout_seconds: 60    # Max time to read request (default: 30)
  write_timeout_seconds: 60   # Max time to write response (default: 30)
  idle_timeout_seconds: 180   # Max idle time for keep-alive (default: 120)
```

## Worker Configuration

Each worker should have its own `config/worker.yaml` file in its directory:

```yaml
# workers/api/config/worker.yaml

# Path prefix for this worker (required)
path: "/api"

# Worker runtime settings
runtime:
  go_max_procs: 2           # Max CPU cores (0 = use all available)
  go_mem_limit: "512MiB"    # Max memory limit (empty = unlimited)
  max_requests: 10000       # Restart after N requests (0 = unlimited)

# Timeout settings
timeouts:
  read_timeout_seconds: 30     # HTTP read timeout
  write_timeout_seconds: 30    # HTTP write timeout
  idle_timeout_seconds: 120    # HTTP idle timeout

# Logging
logging:
  log_file: "logs/worker_{name}_{date}.log"  # Placeholders: {name}, {date}
```

### Worker Options

#### path (required)
The URL path where this worker will be mounted.

```yaml
path: "/"       # Root path
# path: "/api"  # API prefix
```

#### runtime.go_max_procs
Maximum number of CPU cores to use (0 = use all available).

```yaml
runtime:
  go_max_procs: 2  # Use 2 cores
```

#### runtime.go_mem_limit
Maximum memory limit (e.g., "512MiB", "2GiB", empty = unlimited).

```yaml
runtime:
  go_mem_limit: "512MiB"
```

#### runtime.max_requests
Restart worker after N requests (0 = unlimited). Useful for preventing memory leaks.

```yaml
runtime:
  max_requests: 10000  # Restart after 10k requests
```

#### timeouts
Timeout settings for HTTP requests.

```yaml
timeouts:
  read_timeout_seconds: 30     # Read timeout
  write_timeout_seconds: 30    # Write timeout
  idle_timeout_seconds: 120    # Idle timeout
```

#### logging
Logging configuration for the worker.

```yaml
logging:
  log_file: "logs/worker_{name}_{date}.log"
  # log_file: "~"  # Disable file logging
```

## Port Pool Configuration

TQServer uses a pool of ports for worker processes:

```yaml
workers:
  port_range_start: 10000  # First port in pool (default: 9000)
  port_range_end: 19999    # Last port in pool (default: 9999)
```

The port pool should be large enough to accommodate:
- Active workers
- Workers being restarted
- Health check overlap periods

**Formula**: `pool_size >= (num_workers * instances) * 2 + 10`

Example:
- 10 workers × 2 instances = 20 active workers
- × 2 for overlapping restarts = 40 ports
- + 10 buffer = 50 ports needed
- Port range: 9000-9050

## Worker Startup Configuration

Configure worker startup and shutdown behavior:

```yaml
workers:
  startup_delay_ms: 200           # Wait time before routing traffic (default: 100ms)
  restart_delay_ms: 200           # Delay before stopping old worker (starts after new port is ready)
  shutdown_grace_period_ms: 1000  # Time for graceful shutdown (default: 500ms)
  port_wait_timeout_ms: 5000      # Max time to wait for new port (default: 5000ms)
```

## Logging Configuration

Configure logging behavior:

```yaml
server:
  log_file: "logs/tqserver_{date}.log"  # Server log file
  # log_file: "~"  # Disable file logging
```

### Per-Worker Logging

Workers can have their own log configuration:

```yaml
# workers/api/config/worker.yaml
logging:
  log_file: "logs/worker_{name}_{date}.log"
  # log_file: "~"  # Disable file logging
```

Placeholders:
- `{name}`: Worker name (from directory name)
- `{date}`: Current date (YYYY-MM-DD)

## Environment-Based Configuration

Use environment variables to override configuration:

```bash
# Export environment variables
export TQSERVER_PORT=8081
export TQSERVER_LOG_LEVEL=debug
export TQSERVER_WORKERS_PATH=./workers

# Run with environment overrides
./bin/tqserver
```

### Environment Variable Format

Environment variables follow this pattern:
- Prefix: `TQSERVER_`
- Nested keys: separated by `_`
- Example: `server.port` → `TQSERVER_SERVER_PORT`

## File Watching Configuration

TQServer automatically watches files for changes in development mode:

```yaml
file_watcher:
  debounce_ms: 100  # Debounce delay to avoid multiple rebuilds (default: 50ms)
```

When source or configuration changes are detected:
1. Changes are debounced to avoid multiple rebuilds
2. Workers are rebuilt if source code changed
3. Workers are gracefully restarted
4. No server downtime required

## Example Configurations

### Development Configuration

```yaml
# config/server.dev.yaml
server:
  port: 3000
  log_file: "logs/tqserver_{date}.log"

workers:
  directory: "workers"
  port_range_start: 10000
  port_range_end: 19999
  startup_delay_ms: 100  # Fast startup

file_watcher:
  debounce_ms: 50  # Fast reloads
```

### Production Configuration

```yaml
# config/server.prod.yaml
server:
  port: 80
  read_timeout_seconds: 60
  write_timeout_seconds: 60
  idle_timeout_seconds: 180
  log_file: "/var/log/tqserver/server_{date}.log"

workers:
  directory: "workers"
  port_range_start: 10000
  port_range_end: 19999
  startup_delay_ms: 200
  restart_delay_ms: 200
  shutdown_grace_period_ms: 1000
```

## Next Steps

- [Directory Structure](structure.md) - Understand project organization
- [Deployment](deployment.md) - Deploy TQServer to production
- [Worker Configuration](../workers/configuration.md) - Advanced worker settings
- [Performance Tuning](../advanced/performance.md) - Optimize performance

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
server:
  host: "0.0.0.0"
  port: 8080
  log_file: "logs/server-{date}.log"
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 120s
  
  port_pool:
    start: 9000
    end: 9100
  
  static:
    enabled: true
    directory: "public"
    cache_control: "public, max-age=3600"

workers:
  base_path: "workers"
  build_timeout: 60s
  startup_timeout: 10s
  
  health_check:
    enabled: true
    interval: 30s
    timeout: 5s
    path: "/health"
  
  file_watcher:
    enabled: true
    debounce: 500ms
    ignore_patterns:
      - "*.log"
      - "*.tmp"
      - ".git/**"
      - "bin/**"

cleanup:
  enabled: true
  interval: 1h
  max_age: 24h
```

### Server Options

#### host
The network interface to bind to. Use `0.0.0.0` to listen on all interfaces, or `127.0.0.1` for localhost only.

```yaml
server:
  host: "0.0.0.0"  # Listen on all interfaces
  # host: "127.0.0.1"  # Localhost only
```

#### port
The HTTP port for the server.

```yaml
server:
  port: 8080
```

#### log_file
Path to the server log file. Use `{date}` placeholder for date-based rotation.

```yaml
server:
  log_file: "logs/server-{date}.log"
  # log_file: "~"  # Disable file logging
```

#### Timeout Settings

```yaml
server:
  read_timeout: 30s    # Max time to read request
  write_timeout: 30s   # Max time to write response
  idle_timeout: 120s   # Max idle time for keep-alive
```

## Worker Configuration

Each worker can have its own `config.yaml` file in its directory:

```yaml
# workers/api/config.yaml
worker:
  name: "api"
  instances: 1
  timeout: 30s
  
  resources:
    max_memory: "512M"
    max_cpu: 1.0
  
  health_check:
    enabled: true
    path: "/health"
    interval: 30s
    timeout: 5s
    
  environment:
    DATABASE_URL: "postgresql://localhost/mydb"
    REDIS_URL: "redis://localhost:6379"
    LOG_LEVEL: "info"
  
  routes:
    - pattern: "/api/*"
      methods: ["GET", "POST", "PUT", "DELETE"]
    
  middleware:
    - "cors"
    - "ratelimit"
    
  static:
    public: "public"
    private: "private"
```

### Worker Options

#### name
The worker name (usually matches the directory name).

```yaml
worker:
  name: "api"
```

#### instances
Number of worker instances to run (for load balancing).

```yaml
worker:
  instances: 3  # Run 3 instances
```

#### timeout
Maximum time for worker to process a request.

```yaml
worker:
  timeout: 30s
```

#### resources
Resource limits for the worker process.

```yaml
worker:
  resources:
    max_memory: "512M"  # Maximum memory
    max_cpu: 1.0        # CPU cores
```

#### environment
Environment variables passed to the worker.

```yaml
worker:
  environment:
    DATABASE_URL: "postgresql://localhost/mydb"
    API_KEY: "secret-key"
    DEBUG: "false"
```

## Port Pool Configuration

TQServer uses a pool of ports for worker processes:

```yaml
server:
  port_pool:
    start: 9000      # First port in pool
    end: 9100        # Last port in pool
    reuse_delay: 5s  # Wait before reusing a port
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

## Health Check Configuration

Configure health checks for worker processes:

```yaml
workers:
  health_check:
    enabled: true
    interval: 30s      # Check every 30 seconds
    timeout: 5s        # Fail if no response in 5s
    path: "/health"    # Health check endpoint
    failure_threshold: 3  # Unhealthy after 3 failures
    success_threshold: 2  # Healthy after 2 successes
```

### Health Check Response

Workers should respond to health checks:

```go
// workers/api/src/main.go
func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status": "healthy",
        "timestamp": time.Now().Unix(),
    })
}
```

## Logging Configuration

Configure logging behavior:

```yaml
server:
  log_file: "logs/server-{date}.log"
  log_level: "info"  # debug, info, warn, error
  log_format: "text"  # text or json
  
  log_rotation:
    enabled: true
    max_size: 100M    # Rotate after 100MB
    max_age: 7        # Keep logs for 7 days
    max_backups: 10   # Keep 10 old logs
    compress: true    # Compress old logs
```

### Log Levels

- **debug**: Detailed information for debugging
- **info**: General informational messages
- **warn**: Warning messages
- **error**: Error messages only

### Per-Worker Logging

Workers can have their own log configuration:

```yaml
# workers/api/config.yaml
worker:
  log_file: "logs/api-{date}.log"
  log_level: "debug"
```

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

## Configuration Caching

TQServer automatically watches configuration files for changes:

```yaml
server:
  config_reload:
    enabled: true
    debounce: 1s  # Wait 1s before reloading
```

When configuration changes are detected:
1. New configuration is validated
2. If valid, configuration is hot-reloaded
3. Workers are gracefully restarted if needed
4. No server downtime required

### Disabling Config Reload

For production environments where configuration should be static:

```yaml
server:
  config_reload:
    enabled: false
```

Restart the server to apply configuration changes.

## Configuration Validation

TQServer validates configuration on startup and reload:

```bash
# Validate configuration without starting
./bin/tqserver -validate -config=config/server.yaml
```

Common validation errors:
- Port conflicts
- Invalid time durations
- Missing required fields
- Invalid file paths

## Example Configurations

### Development Configuration

```yaml
# config/server.dev.yaml
server:
  host: "127.0.0.1"
  port: 8080
  log_level: "debug"
  
workers:
  file_watcher:
    enabled: true
    debounce: 100ms  # Fast reloads
  
  health_check:
    interval: 10s  # Frequent checks
```

### Production Configuration

```yaml
# config/server.prod.yaml
server:
  host: "0.0.0.0"
  port: 80
  log_level: "info"
  log_format: "json"
  
  config_reload:
    enabled: false
  
workers:
  file_watcher:
    enabled: false  # Disable hot reload
  
  health_check:
    interval: 60s
    failure_threshold: 5
```

## Next Steps

- [Directory Structure](structure.md) - Understand project organization
- [Deployment](deployment.md) - Deploy TQServer to production
- [Worker Configuration](../workers/configuration.md) - Advanced worker settings
- [Performance Tuning](../advanced/performance.md) - Optimize performance

# Worker Configuration

> **Note**: The configuration examples in this document include properties for illustration purposes. 
> For actual configuration options, please refer to:
> - `config/server.example.yaml` for server configuration
> - `config/worker.example.yaml` for worker configuration

- [Introduction](#introduction)
- [Configuration Files](#configuration-files)
- [Configuration Structure](#configuration-structure)
- [Server Configuration](#server-configuration)
- [Per-Worker Configuration](#per-worker-configuration)
- [Environment Variables](#environment-variables)
- [Resource Limits](#resource-limits)
- [Runtime Configuration](#runtime-configuration)
- [Best Practices](#best-practices)

## Introduction

TQServer provides flexible configuration at both the server level and per-worker level. Configuration can be specified through YAML files, environment variables, or command-line flags.

## Configuration Files

### Configuration Hierarchy

```
config/
├── server.yaml           # Server-wide configuration
└── defaults.yaml         # Default worker configuration

workers/
├── api/
│   └── config.yaml       # API worker specific config
├── admin/
│   └── config.yaml       # Admin worker specific config
└── reports/
    └── config.yaml       # Reports worker specific config
```

### Loading Order

Configuration is loaded in this priority (highest to lowest):

1. **Command-line flags** (highest priority)
2. **Environment variables**
3. **Worker-specific config** (`workers/{name}/config.yaml`)
4. **Server config** (`config/server.yaml`)
5. **Defaults** (built-in defaults)

```go
func LoadConfig(workerName string) (*Config, error) {
    // Start with defaults
    config := DefaultConfig()
    
    // Load server config
    if err := config.LoadFrom("config/server.yaml"); err != nil {
        return nil, err
    }
    
    // Load worker-specific config
    workerConfigPath := fmt.Sprintf("workers/%s/config.yaml", workerName)
    if _, err := os.Stat(workerConfigPath); err == nil {
        if err := config.LoadFrom(workerConfigPath); err != nil {
            return nil, err
        }
    }
    
    // Override with environment variables
    config.LoadFromEnv()
    
    // Override with command-line flags
    config.LoadFromFlags()
    
    return config, nil
}
```

## Configuration Structure

### Full Configuration Example

```yaml
# config/server.yaml
# Note: Many properties shown are for illustration. 
# See config/server.example.yaml for actual supported properties.

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

# File watcher (hot reload)
file_watcher:
  enabled: true
  debounce: 500ms
  ignore_patterns:
    - "*.log"
    - "*.tmp"
    - ".git/**"
    - "**/node_modules/**"

# Logging
logging:
  level: "info"
  format: "json"
  output: "stdout"
  file:
    enabled: false
    path: "logs/tqserver.log"
    max_size: 100  # MB
    max_backups: 3
    max_age: 28    # days

# Default worker settings
workers:
  defaults:
    resources:
      max_memory: "512M"
      max_cpu: 2.0
      max_open_files: 1024
    
    timeouts:
      startup: 30s
      shutdown: 30s
      idle: 120s
    
    environment:
      GO_ENV: "production"
```

## Server Configuration

### Server Settings

```yaml
server:
  # Listen address
  host: "0.0.0.0"          # Bind to all interfaces
  # host: "127.0.0.1"      # Local only
  
  # HTTP port
  port: 8080
  
  # TLS configuration
  tls:
    enabled: false
    cert_file: "certs/server.crt"
    key_file: "certs/server.key"
    min_version: "1.2"       # TLS 1.2
  
  # Timeouts
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 120s
  
  # Limits
  max_header_bytes: 1048576  # 1MB
  max_body_bytes: 10485760   # 10MB
  
  # Proxy settings
  proxy:
    buffer_pool_size: 32
    flush_interval: 100ms
    dial_timeout: 10s
```

### Router Configuration

```yaml
router:
  # Route matching
  strict_slash: true        # /api/ != /api
  trailing_slash: false     # Remove trailing slashes
  
  # Request handling
  forward_headers:
    - "X-Forwarded-For"
    - "X-Forwarded-Proto"
    - "X-Real-IP"
  
  # Middleware
  middleware:
    - "logging"
    - "recovery"
    - "request_id"
    - "cors"
  
  # CORS
  cors:
    allowed_origins:
      - "https://example.com"
    allowed_methods:
      - "GET"
      - "POST"
      - "PUT"
      - "DELETE"
    allowed_headers:
      - "Content-Type"
      - "Authorization"
    expose_headers:
      - "X-Request-ID"
    max_age: 3600
```

## Per-Worker Configuration

### Worker-Specific Config

```yaml
# workers/api/config.yaml

worker:
  # Worker identity
  name: "api"
  description: "Main API worker"
  
  # Resource limits
  resources:
    max_memory: "1G"         # 1GB RAM
    max_cpu: 3.0             # 3 CPU cores
    max_open_files: 2048     # File descriptors
    max_threads: 100         # Thread limit
  
  # Timeouts
  timeouts:
    startup: 45s             # Startup timeout
    shutdown: 60s            # Graceful shutdown
    idle: 300s               # Idle connection timeout
    request: 120s            # Max request duration
  
  # Health checks
  health_check:
    enabled: true
    path: "/health"
    interval: 10s
    timeout: 5s
    failure_threshold: 3
    success_threshold: 2
  
  # Restart policy
  restart_policy:
    enabled: true
    max_restarts: 10
    restart_period: 10m
    backoff_initial: 2s
    backoff_multiplier: 2
    backoff_max: 60s
  
  # Build configuration
  build:
    command: "go build -o bin/api src/*.go"
    timeout: 60s
    env:
      CGO_ENABLED: "0"
      GOOS: "linux"
      GOARCH: "amd64"
  
  # Environment variables
  environment:
    GO_ENV: "production"
    LOG_LEVEL: "info"
    DATABASE_POOL_SIZE: "25"
    REDIS_POOL_SIZE: "50"
  
  # Secrets (loaded from files)
  secrets:
    database_url:
      file: "/run/secrets/database_url"
    api_key:
      file: "/run/secrets/api_key"
```

### Minimal Worker Config

```yaml
# workers/simple/config.yaml

worker:
  resources:
    max_memory: "256M"
    max_cpu: 1.0
  
  environment:
    LOG_LEVEL: "debug"
```

## Environment Variables

### Standard Environment Variables

Every worker receives these standard environment variables:

```bash
# Worker identity
WORKER_NAME=api              # Worker name
WORKER_PATH=/app/workers/api # Worker directory

# Network
WORKER_PORT=9000             # Assigned port
WORKER_NAME=api              # Worker name
WORKER_ROUTE=/api            # Route path
WORKER_MODE=development      # Deployment mode
HOST=0.0.0.0                 # Listen host

# Paths
PUBLIC_DIR=/app/workers/api/public
PRIVATE_DIR=/app/workers/api/private
LOGS_DIR=/app/logs

# Runtime
GO_ENV=production            # Environment (dev/prod)
```

### Custom Environment Variables

Add custom variables in configuration:

```yaml
worker:
  environment:
    # Database
    DATABASE_URL: "postgres://user:pass@localhost/dbname"
    DATABASE_MAX_CONNECTIONS: "25"
    
    # Redis
    REDIS_URL: "redis://localhost:6379/0"
    REDIS_POOL_SIZE: "50"
    
    # Application
    APP_NAME: "My API"
    APP_VERSION: "1.0.0"
    LOG_LEVEL: "info"
    
    # Feature flags
    FEATURE_NEW_UI: "true"
    FEATURE_BETA_API: "false"
```

### Using Environment Variables in Worker

```go
package main

import (
    "os"
    "strconv"
)

func main() {
    // Standard variables
    port := os.Getenv("WORKER_PORT")
    workerName := os.Getenv("WORKER_NAME")
    
    // Custom variables
    dbURL := os.Getenv("DATABASE_URL")
    logLevel := os.Getenv("LOG_LEVEL")
    
    // Parse integers
    poolSize, _ := strconv.Atoi(os.Getenv("DATABASE_POOL_SIZE"))
    
    // Parse booleans
    featureEnabled := os.Getenv("FEATURE_NEW_UI") == "true"
    
    // Defaults
    if logLevel == "" {
        logLevel = "info"
    }
    
    // Initialize app
    app := NewApp(Config{
        Port:        port,
        WorkerName:  workerName,
        DatabaseURL: dbURL,
        LogLevel:    logLevel,
        PoolSize:    poolSize,
    })
    
    app.Run()
}
```

### Environment Variable Precedence

```bash
# 1. System environment (highest)
export DATABASE_URL="postgres://..."

# 2. .env file
# .env
DATABASE_URL=postgres://...

# 3. config.yaml
worker:
  environment:
    DATABASE_URL: "postgres://..."

# 4. Defaults in code (lowest)
dbURL := os.Getenv("DATABASE_URL")
if dbURL == "" {
    dbURL = "postgres://localhost/default"
}
```

## Resource Limits

### Memory Limits

```yaml
worker:
  resources:
    max_memory: "512M"      # Maximum RAM
    # Supports: B, K, M, G suffixes
```

**Enforcement using cgroups**:
```bash
# Create memory cgroup
cgcreate -g memory:/tqserver/workers/api

# Set limit (512MB = 536870912 bytes)
echo 536870912 > /sys/fs/cgroup/memory/tqserver/workers/api/memory.limit_in_bytes

# Run worker in cgroup
cgexec -g memory:/tqserver/workers/api /app/workers/api/bin/api
```

### CPU Limits

```yaml
worker:
  resources:
    max_cpu: 2.0            # 2 CPU cores
    cpu_shares: 1024        # Relative priority
```

**Enforcement using cgroups**:
```bash
# Set CPU quota (200% = 2 cores)
echo 200000 > /sys/fs/cgroup/cpu/tqserver/workers/api/cpu.cfs_quota_us
echo 100000 > /sys/fs/cgroup/cpu/tqserver/workers/api/cpu.cfs_period_us
```

**Using systemd**:
```ini
[Service]
CPUQuota=200%              # 2 cores
CPUWeight=100              # Priority
```

### File Descriptor Limits

```yaml
worker:
  resources:
    max_open_files: 1024    # File descriptor limit
```

**Enforcement using rlimit**:
```go
func setFileDescriptorLimit(limit int) error {
    rlimit := syscall.Rlimit{
        Cur: uint64(limit),
        Max: uint64(limit),
    }
    return syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rlimit)
}
```

### Disk I/O Limits

```yaml
worker:
  resources:
    max_io_read: 10485760   # 10 MB/s read
    max_io_write: 10485760  # 10 MB/s write
```

**Enforcement using cgroups**:
```bash
# Limit read bandwidth (10 MB/s)
echo "8:0 10485760" > /sys/fs/cgroup/blkio/tqserver/workers/api/blkio.throttle.read_bps_device

# Limit write bandwidth (10 MB/s)
echo "8:0 10485760" > /sys/fs/cgroup/blkio/tqserver/workers/api/blkio.throttle.write_bps_device
```

## Runtime Configuration

### Timeouts

```yaml
worker:
  timeouts:
    startup: 30s            # Max time to become healthy
    shutdown: 30s           # Graceful shutdown timeout
    idle: 120s              # Idle connection timeout
    request: 60s            # Max request duration
    read_header: 10s        # HTTP header read timeout
    read_body: 30s          # HTTP body read timeout
    write: 30s              # HTTP write timeout
```

### Restart Policy

```yaml
worker:
  restart_policy:
    enabled: true
    max_restarts: 5         # Max restarts in period
    restart_period: 5m      # Time window for max_restarts
    backoff_initial: 1s     # Initial backoff delay
    backoff_multiplier: 2   # Multiply backoff by this
    backoff_max: 30s        # Maximum backoff delay
```

**Backoff Calculation**:
```
Restart 1: 1s
Restart 2: 2s (1s * 2)
Restart 3: 4s (2s * 2)
Restart 4: 8s (4s * 2)
Restart 5: 16s (8s * 2)
Restart 6: 30s (16s * 2, capped at max)
```

### Health Check Configuration

```yaml
worker:
  health_check:
    enabled: true
    path: "/health"         # Health endpoint path
    method: "GET"           # HTTP method
    interval: 10s           # Check every 10s
    timeout: 5s             # Timeout for each check
    failure_threshold: 3    # Unhealthy after 3 failures
    success_threshold: 2    # Healthy after 2 successes
    headers:
      X-Health-Check: "true"
```

## Best Practices

### Separate Configs by Environment

```
config/
├── server.yaml              # Base config
├── server.dev.yaml          # Development overrides
├── server.staging.yaml      # Staging overrides
└── server.prod.yaml         # Production overrides
```

**Loading environment-specific config**:
```go
env := os.Getenv("GO_ENV")
if env == "" {
    env = "development"
}

configFile := fmt.Sprintf("config/server.%s.yaml", env)
config.LoadFrom(configFile)
```

### Use Environment Variables for Secrets

```yaml
# DON'T: Hardcode secrets
worker:
  environment:
    DATABASE_PASSWORD: "mysecretpassword"  # Bad!

# DO: Reference environment variables
worker:
  environment:
    DATABASE_PASSWORD: "${DATABASE_PASSWORD}"

# OR: Use secret files
worker:
  secrets:
    database_password:
      file: "/run/secrets/db_password"
```

### Document Configuration

```yaml
# workers/api/config.yaml

worker:
  # Resource allocation
  # Adjust based on load testing results
  resources:
    max_memory: "1G"         # Peak usage observed: 750MB
    max_cpu: 2.0             # 95th percentile: 1.5 cores
  
  # Health checks
  # Path must return 200 when healthy
  health_check:
    path: "/health"          # Implements HealthChecker interface
    interval: 10s            # Aligns with LB health check
```

### Validate Configuration

```go
func (c *Config) Validate() error {
    if c.Server.Port < 1 || c.Server.Port > 65535 {
        return errors.New("invalid server port")
    }
    
    if c.PortPool.Start >= c.PortPool.End {
        return errors.New("invalid port pool range")
    }
    
    if c.Worker.Resources.MaxMemory <= 0 {
        return errors.New("max_memory must be positive")
    }
    
    if c.Worker.Timeouts.Startup < time.Second {
        return errors.New("startup timeout too short")
    }
    
    return nil
}
```

### Use Defaults Wisely

```go
func DefaultConfig() *Config {
    return &Config{
        Server: ServerConfig{
            Host:            "0.0.0.0",
            Port:            8080,
            ReadTimeout:     30 * time.Second,
            WriteTimeout:    30 * time.Second,
            IdleTimeout:     120 * time.Second,
            MaxHeaderBytes:  1 << 20, // 1MB
        },
        Worker: WorkerConfig{
            Resources: ResourceConfig{
                MaxMemory:     512 * 1024 * 1024, // 512MB
                MaxCPU:        2.0,
                MaxOpenFiles:  1024,
            },
            Timeouts: TimeoutConfig{
                Startup:  30 * time.Second,
                Shutdown: 30 * time.Second,
                Idle:     120 * time.Second,
            },
        },
    }
}
```

### Reloadable Configuration

```go
// Watch config file for changes
func (s *Supervisor) watchConfig() {
    watcher, _ := fsnotify.NewWatcher()
    watcher.Add("config/server.yaml")
    
    for {
        select {
        case event := <-watcher.Events:
            if event.Op&fsnotify.Write == fsnotify.Write {
                log.Println("Config file changed, reloading...")
                
                newConfig, err := LoadConfig()
                if err != nil {
                    log.Printf("Failed to reload config: %v", err)
                    continue
                }
                
                s.UpdateConfig(newConfig)
            }
        }
    }
}
```

## Configuration Examples

### Development Environment

```yaml
# config/server.dev.yaml
server:
  host: "127.0.0.1"
  port: 8080

file_watcher:
  enabled: true
  debounce: 100ms           # Fast reload

logging:
  level: "debug"
  format: "console"

workers:
  defaults:
    resources:
      max_memory: "256M"    # Lower limits for dev
      max_cpu: 1.0
```

### Production Environment

```yaml
# config/server.prod.yaml
server:
  host: "0.0.0.0"
  port: 8080
  tls:
    enabled: true
    cert_file: "/etc/tqserver/certs/server.crt"
    key_file: "/etc/tqserver/certs/server.key"

file_watcher:
  enabled: false            # No hot reload in prod

logging:
  level: "warn"
  format: "json"
  file:
    enabled: true
    path: "/var/log/tqserver/server.log"

workers:
  defaults:
    resources:
      max_memory: "1G"      # Production limits
      max_cpu: 4.0
```

## Next Steps

- [Building Workers](building.md) - Build system configuration
- [Testing Workers](testing.md) - Test configuration
- [Health Checks](health-checks.md) - Health check implementation
- [Security](../security/authentication.md) - Security configuration

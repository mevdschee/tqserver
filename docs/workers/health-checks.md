# Health Checks

- [Introduction](#introduction)
- [Why Health Checks](#why-health-checks)
- [Health Check Endpoint](#health-check-endpoint)
- [Health Check Types](#health-check-types)
- [Implementation](#implementation)
- [Configuration](#configuration)
- [Best Practices](#best-practices)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)

## Introduction

Health checks are critical for ensuring worker availability and enabling TQServer to make intelligent routing and restart decisions. Every worker should implement a health check endpoint that reports its operational status.

## Why Health Checks

### Use Cases

1. **Supervisor Monitoring**: Detect unhealthy workers and restart them
2. **Load Balancer Integration**: Route traffic only to healthy instances
3. **Zero-Downtime Deployments**: Wait for new version to be healthy before swapping
4. **Debugging**: Quickly identify which dependencies are failing
5. **Alerting**: Trigger alerts when workers become unhealthy

### Health Check Flow

```
TQServer Supervisor
    │
    ├─→ GET /health (Worker A)
    │   ├─ Status: 200 ✓
    │   └─ Continue routing traffic
    │
    ├─→ GET /health (Worker B)
    │   ├─ Status: 503 ✗
    │   └─ Mark unhealthy → Restart
    │
    └─→ GET /health (Worker C)
        ├─ Timeout ✗
        └─ Mark unhealthy → Restart
```

## Health Check Endpoint

### Basic Health Endpoint

```go
// workers/api/src/main.go
package main

import (
    "encoding/json"
    "net/http"
)

type HealthResponse struct {
    Status string            `json:"status"`
    Checks map[string]string `json:"checks"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
    health := HealthResponse{
        Status: "healthy",
        Checks: make(map[string]string),
    }
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(health)
}

func main() {
    http.HandleFunc("/health", healthHandler)
    http.ListenAndServe(":"+os.Getenv("PORT"), nil)
}
```

### Response Format

**Healthy Response** (200 OK):
```json
{
  "status": "healthy",
  "checks": {}
}
```

**Unhealthy Response** (503 Service Unavailable):
```json
{
  "status": "unhealthy",
  "checks": {
    "database": "connection failed: timeout",
    "redis": "healthy",
    "disk_space": "healthy"
  }
}
```

## Health Check Types

### 1. Liveness Check

**Purpose**: Is the worker process alive?

**Implementation**:
```go
func livenessHandler(w http.ResponseWriter, r *http.Request) {
    // Minimal check - just respond
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("alive"))
}
```

**Use Case**: Kubernetes liveness probe

### 2. Readiness Check

**Purpose**: Is the worker ready to handle requests?

**Implementation**:
```go
func readinessHandler(w http.ResponseWriter, r *http.Request) {
    // Check if all dependencies are ready
    if !isReady() {
        w.WriteHeader(http.StatusServiceUnavailable)
        w.Write([]byte("not ready"))
        return
    }
    
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("ready"))
}

func isReady() bool {
    // Check database
    if err := db.Ping(); err != nil {
        return false
    }
    
    // Check cache
    if err := cache.Ping(); err != nil {
        return false
    }
    
    // All checks passed
    return true
}
```

**Use Case**: Kubernetes readiness probe, load balancer health checks

### 3. Startup Check

**Purpose**: Has the worker completed initialization?

**Implementation**:
```go
var started bool

func startupHandler(w http.ResponseWriter, r *http.Request) {
    if !started {
        w.WriteHeader(http.StatusServiceUnavailable)
        w.Write([]byte("starting"))
        return
    }
    
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("started"))
}

func initialize() {
    // Load configuration
    loadConfig()
    
    // Connect to database
    connectDatabase()
    
    // Warm up caches
    warmupCaches()
    
    // Mark as started
    started = true
}
```

**Use Case**: TQServer hot reload, slow-starting workers

## Implementation

### Comprehensive Health Check

```go
package main

import (
    "context"
    "encoding/json"
    "net/http"
    "sync"
    "time"
)

type HealthChecker struct {
    db    *sql.DB
    cache *redis.Client
    disk  DiskChecker
}

type HealthStatus struct {
    Status    string            `json:"status"`
    Timestamp time.Time         `json:"timestamp"`
    Checks    map[string]Check  `json:"checks"`
}

type Check struct {
    Status  string `json:"status"`
    Message string `json:"message,omitempty"`
}

func (hc *HealthChecker) Handler(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()
    
    status := HealthStatus{
        Timestamp: time.Now(),
        Checks:    make(map[string]Check),
    }
    
    // Run checks in parallel
    var wg sync.WaitGroup
    var mu sync.Mutex
    
    checks := []struct {
        name string
        fn   func(context.Context) Check
    }{
        {"database", hc.checkDatabase},
        {"cache", hc.checkCache},
        {"disk_space", hc.checkDiskSpace},
    }
    
    for _, check := range checks {
        wg.Add(1)
        go func(name string, fn func(context.Context) Check) {
            defer wg.Done()
            
            result := fn(ctx)
            
            mu.Lock()
            status.Checks[name] = result
            mu.Unlock()
        }(check.name, check.fn)
    }
    
    wg.Wait()
    
    // Determine overall status
    status.Status = "healthy"
    for _, check := range status.Checks {
        if check.Status == "unhealthy" {
            status.Status = "unhealthy"
            break
        }
    }
    
    // Set HTTP status
    httpStatus := http.StatusOK
    if status.Status == "unhealthy" {
        httpStatus = http.StatusServiceUnavailable
    }
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(httpStatus)
    json.NewEncoder(w).Encode(status)
}

func (hc *HealthChecker) checkDatabase(ctx context.Context) Check {
    if err := hc.db.PingContext(ctx); err != nil {
        return Check{
            Status:  "unhealthy",
            Message: err.Error(),
        }
    }
    
    return Check{Status: "healthy"}
}

func (hc *HealthChecker) checkCache(ctx context.Context) Check {
    if err := hc.cache.Ping(ctx).Err(); err != nil {
        return Check{
            Status:  "unhealthy",
            Message: err.Error(),
        }
    }
    
    return Check{Status: "healthy"}
}

func (hc *HealthChecker) checkDiskSpace(ctx context.Context) Check {
    usage, err := hc.disk.Usage()
    if err != nil {
        return Check{
            Status:  "unhealthy",
            Message: err.Error(),
        }
    }
    
    if usage > 90.0 {
        return Check{
            Status:  "unhealthy",
            Message: fmt.Sprintf("disk usage %.1f%%", usage),
        }
    }
    
    return Check{Status: "healthy"}
}
```

### Graceful Degradation

```go
func (hc *HealthChecker) checkCache(ctx context.Context) Check {
    if err := hc.cache.Ping(ctx).Err(); err != nil {
        // Cache failure doesn't make worker unhealthy
        // Just report the issue
        return Check{
            Status:  "degraded",
            Message: fmt.Sprintf("cache unavailable: %v", err),
        }
    }
    
    return Check{Status: "healthy"}
}

// Overall status considers degraded as healthy
func calculateStatus(checks map[string]Check) string {
    for _, check := range checks {
        if check.Status == "unhealthy" {
            return "unhealthy"
        }
    }
    
    // Degraded still allows traffic
    return "healthy"
}
```

### Deep Health Checks

```go
type DeepHealthChecker struct {
    db *sql.DB
}

func (hc *DeepHealthChecker) checkDatabase(ctx context.Context) Check {
    // Basic connectivity
    if err := hc.db.PingContext(ctx); err != nil {
        return Check{
            Status:  "unhealthy",
            Message: fmt.Sprintf("connection failed: %v", err),
        }
    }
    
    // Query performance
    start := time.Now()
    var result int
    err := hc.db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
    latency := time.Since(start)
    
    if err != nil {
        return Check{
            Status:  "unhealthy",
            Message: fmt.Sprintf("query failed: %v", err),
        }
    }
    
    if latency > 100*time.Millisecond {
        return Check{
            Status:  "degraded",
            Message: fmt.Sprintf("slow query: %v", latency),
        }
    }
    
    // Check connection pool
    stats := hc.db.Stats()
    if stats.OpenConnections >= stats.MaxOpenConnections {
        return Check{
            Status:  "degraded",
            Message: "connection pool exhausted",
        }
    }
    
    return Check{
        Status:  "healthy",
        Message: fmt.Sprintf("latency: %v", latency),
    }
}
```

## Configuration

### Health Check Configuration

```yaml
# workers/api/config.yaml

worker:
  health_check:
    # Health endpoint path
    path: "/health"
    
    # Check interval
    interval: 10s
    
    # Check timeout
    timeout: 5s
    
    # Failure threshold (unhealthy after N failures)
    failure_threshold: 3
    
    # Success threshold (healthy after N successes)
    success_threshold: 2
    
    # HTTP method
    method: "GET"
    
    # Expected status code
    expected_status: 200
    
    # Custom headers
    headers:
      X-Health-Check: "supervisor"
```

### TQServer Configuration

```yaml
# config/server.yaml

supervisor:
  health_checks:
    enabled: true
    interval: 10s
    timeout: 5s
    failure_threshold: 3
    success_threshold: 2
    
    # Retry on network errors
    retry_on_network_error: true
    max_retries: 3
```

## Best Practices

### 1. Fast Health Checks

Health checks should complete quickly (< 1 second):

```go
// Good: Fast check
func healthHandler(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
    defer cancel()
    
    // Quick database ping
    if err := db.PingContext(ctx); err != nil {
        http.Error(w, "unhealthy", 503)
        return
    }
    
    w.WriteHeader(200)
}

// Bad: Slow check
func healthHandler(w http.ResponseWriter, r *http.Request) {
    // Expensive query - too slow!
    rows, _ := db.Query("SELECT * FROM users ORDER BY created_at DESC")
    defer rows.Close()
    
    w.WriteHeader(200)
}
```

### 2. Separate Liveness and Readiness

```go
// Liveness: Basic process check
func livenessHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(200)
    w.Write([]byte("alive"))
}

// Readiness: Check dependencies
func readinessHandler(w http.ResponseWriter, r *http.Request) {
    if err := checkDependencies(); err != nil {
        http.Error(w, "not ready", 503)
        return
    }
    
    w.WriteHeader(200)
    w.Write([]byte("ready"))
}

func main() {
    http.HandleFunc("/health", readinessHandler)  // TQServer default
    http.HandleFunc("/live", livenessHandler)     // Liveness check
    http.HandleFunc("/ready", readinessHandler)   // Readiness check
}
```

### 3. Include Version Information

```go
type HealthResponse struct {
    Status    string            `json:"status"`
    Version   string            `json:"version"`
    BuildTime string            `json:"build_time"`
    Uptime    string            `json:"uptime"`
    Checks    map[string]Check  `json:"checks"`
}

var (
    Version   = "1.0.0"
    BuildTime = "2026-01-06T10:00:00Z"
    StartTime = time.Now()
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
    health := HealthResponse{
        Status:    "healthy",
        Version:   Version,
        BuildTime: BuildTime,
        Uptime:    time.Since(StartTime).String(),
        Checks:    runHealthChecks(),
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(health)
}
```

### 4. Handle Timeouts

```go
func healthHandler(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()
    
    resultChan := make(chan HealthStatus, 1)
    
    go func() {
        status := runHealthChecks(ctx)
        resultChan <- status
    }()
    
    select {
    case status := <-resultChan:
        // Got result in time
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(status)
        
    case <-ctx.Done():
        // Timeout
        http.Error(w, "health check timeout", 503)
    }
}
```

### 5. Cache Health Check Results

```go
type CachedHealthChecker struct {
    mu         sync.RWMutex
    lastCheck  time.Time
    lastResult HealthStatus
    cacheTTL   time.Duration
}

func (c *CachedHealthChecker) Handler(w http.ResponseWriter, r *http.Request) {
    c.mu.RLock()
    
    // Return cached result if fresh
    if time.Since(c.lastCheck) < c.cacheTTL {
        result := c.lastResult
        c.mu.RUnlock()
        
        json.NewEncoder(w).Encode(result)
        return
    }
    c.mu.RUnlock()
    
    // Perform fresh health check
    result := c.performHealthCheck(r.Context())
    
    // Cache result
    c.mu.Lock()
    c.lastCheck = time.Now()
    c.lastResult = result
    c.mu.Unlock()
    
    json.NewEncoder(w).Encode(result)
}
```

## Monitoring

### Metrics

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    healthCheckDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "health_check_duration_seconds",
            Help: "Health check duration",
        },
        []string{"check"},
    )
    
    healthCheckStatus = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "health_check_status",
            Help: "Health check status (1=healthy, 0=unhealthy)",
        },
        []string{"check"},
    )
)

func (hc *HealthChecker) checkDatabase(ctx context.Context) Check {
    start := time.Now()
    defer func() {
        duration := time.Since(start).Seconds()
        healthCheckDuration.WithLabelValues("database").Observe(duration)
    }()
    
    if err := hc.db.PingContext(ctx); err != nil {
        healthCheckStatus.WithLabelValues("database").Set(0)
        return Check{Status: "unhealthy", Message: err.Error()}
    }
    
    healthCheckStatus.WithLabelValues("database").Set(1)
    return Check{Status: "healthy"}
}
```

### Logging

```go
func (hc *HealthChecker) checkDatabase(ctx context.Context) Check {
    start := time.Now()
    
    if err := hc.db.PingContext(ctx); err != nil {
        log.Printf("Health check failed: database: %v (took %v)",
            err, time.Since(start))
        return Check{Status: "unhealthy", Message: err.Error()}
    }
    
    log.Printf("Health check passed: database (took %v)", time.Since(start))
    return Check{Status: "healthy"}
}
```

## Troubleshooting

### Health Check Failures

**Problem**: Health checks timing out

```yaml
# Solution: Increase timeout
worker:
  health_check:
    timeout: 10s  # Increase from 5s
```

**Problem**: Flapping (healthy → unhealthy → healthy)

```yaml
# Solution: Adjust thresholds
worker:
  health_check:
    failure_threshold: 5  # Tolerate more failures
    success_threshold: 3  # Require more successes
```

**Problem**: Database connection pool exhausted

```go
// Solution: Limit max connections
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(10)
db.SetConnMaxLifetime(5 * time.Minute)
```

### Debugging

```go
// Add detailed logging
func healthHandler(w http.ResponseWriter, r *http.Request) {
    log.Printf("Health check started from %s", r.RemoteAddr)
    
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()
    
    status := HealthStatus{
        Checks: make(map[string]Check),
    }
    
    // Check database
    start := time.Now()
    status.Checks["database"] = checkDatabase(ctx)
    log.Printf("Database check: %v (took %v)",
        status.Checks["database"].Status, time.Since(start))
    
    // Check cache
    start = time.Now()
    status.Checks["cache"] = checkCache(ctx)
    log.Printf("Cache check: %v (took %v)",
        status.Checks["cache"].Status, time.Since(start))
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(status)
}
```

## Next Steps

- [Worker Lifecycle](lifecycle.md) - How health checks affect lifecycle
- [Supervisor Pattern](../architecture/supervisor.md) - How supervisor uses health checks
- [Monitoring](../monitoring/health.md) - Advanced health monitoring
- [Deployment](../getting-started/deployment.md) - Production health check setup

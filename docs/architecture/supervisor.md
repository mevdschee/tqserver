# Supervisor Pattern

- [Introduction](#introduction)
- [Responsibilities](#responsibilities)
- [Worker Registry](#worker-registry)
- [Health Monitoring](#health-monitoring)
- [Port Allocation](#port-allocation)
- [Crash Recovery](#crash-recovery)
- [Lifecycle Management](#lifecycle-management)
- [Implementation](#implementation)

## Introduction

The Supervisor is the core orchestration component in TQServer. It discovers, starts, monitors, and manages all worker processes. Think of it as a "manager of workers" that ensures your application runs reliably.

```
┌──────────────────────────────────────────────────┐
│                  TQServer                         │
│                                                   │
│  ┌────────────────────────────────────────────┐ │
│  │           Supervisor                       │ │
│  │                                            │ │
│  │  • Discover workers                        │ │
│  │  • Start/stop processes                    │ │
│  │  • Monitor health                          │ │
│  │  • Allocate ports                          │ │
│  │  │  Recover from crashes                    │ │
│  └────────────────────────────────────────────┘ │
│         │        │        │                      │
│         ├────────┼────────┤                      │
│         ↓        ↓        ↓                      │
│    Worker A  Worker B  Worker C                  │
│    (9000)    (9001)    (9002)                    │
└──────────────────────────────────────────────────┘
```

## Responsibilities

### 1. Worker Discovery

Supervisor scans filesystem for workers:

```go
func (s *Supervisor) DiscoverWorkers() ([]WorkerConfig, error) {
    var workers []WorkerConfig
    
    // Scan workers/ directory
    entries, err := os.ReadDir("workers")
    if err != nil {
        return nil, err
    }
    
    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }
        
        workerName := entry.Name()
        workerPath := filepath.Join("workers", workerName)
        
        // Check for binary
        binaryPath := filepath.Join(workerPath, "bin", workerName)
        if _, err := os.Stat(binaryPath); err != nil {
            continue
        }
        
        // Load configuration
        config, err := s.loadWorkerConfig(workerPath)
        if err != nil {
            log.Printf("Failed to load config for %s: %v", workerName, err)
            continue
        }
        
        workers = append(workers, WorkerConfig{
            Name:       workerName,
            BinaryPath: binaryPath,
            Config:     config,
        })
    }
    
    return workers, nil
}
```

**Discovery Rules**:
- Directory exists in `workers/`
- Has `bin/{worker-name}` binary
- Optionally has `config.yaml`

### 2. Process Management

Start and stop worker processes:

```go
func (s *Supervisor) StartWorker(name string) error {
    config := s.workers[name]
    
    // Allocate port
    port, err := s.portPool.Acquire()
    if err != nil {
        return fmt.Errorf("no ports available: %w", err)
    }
    
    // Prepare command
    cmd := exec.Command(config.BinaryPath)
    cmd.Env = append(os.Environ(),
        fmt.Sprintf("WORKER_PORT=%d", port),
        fmt.Sprintf("WORKER_NAME=%s", worker.Name),
        fmt.Sprintf("WORKER_ROUTE=%s", worker.Route),
        fmt.Sprintf("WORKER_MODE=%s", mode),
        // Timeout settings from worker config
        fmt.Sprintf("WORKER_READ_TIMEOUT_SECONDS=%d", config.Timeouts.ReadTimeout),
        fmt.Sprintf("WORKER_WRITE_TIMEOUT_SECONDS=%d", config.Timeouts.WriteTimeout),
        fmt.Sprintf("WORKER_IDLE_TIMEOUT_SECONDS=%d", config.Timeouts.IdleTimeout),
        // Runtime settings from worker config
        fmt.Sprintf("GOMAXPROCS=%d", config.Runtime.GOMAXPROCS),
        fmt.Sprintf("GOMEMLIMIT=%s", config.Runtime.GOMEMLIMIT),
    )
    cmd.Dir = filepath.Join("workers", name)
    
    // Start process
    if err := cmd.Start(); err != nil {
        s.portPool.Release(port)
        return fmt.Errorf("failed to start: %w", err)
    }
    
    // Register worker
    s.registry.Register(name, &Worker{
        Name:    name,
        Port:    port,
        PID:     cmd.Process.Pid,
        Process: cmd.Process,
        Started: time.Now(),
    })
    
    log.Printf("Started worker %s on port %d (PID %d)", name, port, cmd.Process.Pid)
    return nil
}
```

### 3. Health Monitoring

Continuously check worker health:

```go
func (s *Supervisor) monitorHealth() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        workers := s.registry.All()
        
        for _, worker := range workers {
            healthy, err := s.checkHealth(worker)
            
            if err != nil || !healthy {
                log.Printf("Worker %s unhealthy: %v", worker.Name, err)
                s.handleUnhealthyWorker(worker)
            } else {
                s.registry.UpdateLastSeen(worker.Name, time.Now())
            }
        }
    }
}

func (s *Supervisor) checkHealth(worker *Worker) (bool, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    req, _ := http.NewRequestWithContext(ctx, "GET",
        fmt.Sprintf("http://localhost:%d/health", worker.Port),
        nil,
    )
    
    resp, err := s.httpClient.Do(req)
    if err != nil {
        return false, err
    }
    defer resp.Body.Close()
    
    return resp.StatusCode == 200, nil
}
```

### 4. Crash Recovery

Automatically restart crashed workers:

```go
func (s *Supervisor) handleUnhealthyWorker(worker *Worker) {
    s.registry.MarkUnhealthy(worker.Name)
    
    // Check if process is still running
    if !s.isProcessAlive(worker.PID) {
        log.Printf("Worker %s crashed, restarting...", worker.Name)
        s.restartWorker(worker.Name)
        return
    }
    
    // Process alive but unhealthy - graceful restart
    log.Printf("Worker %s unhealthy, performing graceful restart", worker.Name)
    s.gracefulRestart(worker.Name)
}

func (s *Supervisor) restartWorker(name string) error {
    // Stop old worker (if still running)
    if worker := s.registry.Get(name); worker != nil {
        s.StopWorker(name)
    }
    
    // Wait a moment
    time.Sleep(1 * time.Second)
    
    // Start new instance
    return s.StartWorker(name)
}
```

### 5. Resource Management

Track and limit resource usage:

```go
type WorkerResources struct {
    MaxMemory   int64  // Bytes
    MaxCPU      float64 // CPU cores
    MaxOpenFiles int
}

func (s *Supervisor) enforceResourceLimits(worker *Worker) {
    limits := worker.Config.Resources
    
    // Set memory limit
    if limits.MaxMemory > 0 {
        s.setCgroupMemoryLimit(worker.PID, limits.MaxMemory)
    }
    
    // Set CPU limit
    if limits.MaxCPU > 0 {
        s.setCgroupCPULimit(worker.PID, limits.MaxCPU)
    }
    
    // Set file descriptor limit
    if limits.MaxOpenFiles > 0 {
        s.setRlimitNoFile(worker.PID, limits.MaxOpenFiles)
    }
}
```

## Worker Registry

Central registry of all workers:

```go
package supervisor

import (
    "sync"
    "time"
)

type Worker struct {
    Name      string
    Port      int
    PID       int
    Process   *os.Process
    Started   time.Time
    LastSeen  time.Time
    Healthy   bool
    Restarts  int
    Config    WorkerConfig
}

type Registry struct {
    mu      sync.RWMutex
    workers map[string]*Worker
}

func NewRegistry() *Registry {
    return &Registry{
        workers: make(map[string]*Worker),
    }
}

func (r *Registry) Register(name string, worker *Worker) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.workers[name] = worker
}

func (r *Registry) Unregister(name string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    delete(r.workers, name)
}

func (r *Registry) Get(name string) *Worker {
    r.mu.RLock()
    defer r.mu.RUnlock()
    return r.workers[name]
}

func (r *Registry) All() []*Worker {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    workers := make([]*Worker, 0, len(r.workers))
    for _, w := range r.workers {
        workers = append(workers, w)
    }
    return workers
}

func (r *Registry) UpdateLastSeen(name string, t time.Time) {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if worker, ok := r.workers[name]; ok {
        worker.LastSeen = t
        worker.Healthy = true
    }
}

func (r *Registry) MarkUnhealthy(name string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if worker, ok := r.workers[name]; ok {
        worker.Healthy = false
    }
}
```

### Registry Queries

```go
// Get all healthy workers
func (r *Registry) HealthyWorkers() []*Worker {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    var healthy []*Worker
    for _, w := range r.workers {
        if w.Healthy {
            healthy = append(healthy, w)
        }
    }
    return healthy
}

// Get workers by status
func (r *Registry) WorkersByStatus() map[string][]string {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    status := map[string][]string{
        "healthy":   {},
        "unhealthy": {},
    }
    
    for name, w := range r.workers {
        if w.Healthy {
            status["healthy"] = append(status["healthy"], name)
        } else {
            status["unhealthy"] = append(status["unhealthy"], name)
        }
    }
    
    return status
}
```

## Health Monitoring

### Health Check Types

**1. HTTP Health Endpoint**:
```go
func (s *Supervisor) httpHealthCheck(worker *Worker) error {
    resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", worker.Port))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        return fmt.Errorf("unhealthy status: %d", resp.StatusCode)
    }
    
    return nil
}
```

**2. Process Liveness**:
```go
func (s *Supervisor) processLivenessCheck(worker *Worker) error {
    // Send signal 0 to check if process exists
    err := worker.Process.Signal(syscall.Signal(0))
    if err != nil {
        return fmt.Errorf("process not alive: %w", err)
    }
    return nil
}
```

**3. Port Connectivity**:
```go
func (s *Supervisor) portConnectivityCheck(worker *Worker) error {
    conn, err := net.DialTimeout("tcp",
        fmt.Sprintf("localhost:%d", worker.Port),
        2*time.Second,
    )
    if err != nil {
        return fmt.Errorf("cannot connect to port: %w", err)
    }
    conn.Close()
    return nil
}
```

### Health Check Configuration

```yaml
# config/server.yaml
supervisor:
  health_checks:
    interval: 10s           # How often to check
    timeout: 5s             # Health check timeout
    failure_threshold: 3    # Failures before restart
    success_threshold: 2    # Successes to mark healthy
```

### Composite Health Check

```go
func (s *Supervisor) comprehensiveHealthCheck(worker *Worker) (bool, error) {
    checks := []struct {
        name string
        fn   func(*Worker) error
    }{
        {"process", s.processLivenessCheck},
        {"port", s.portConnectivityCheck},
        {"http", s.httpHealthCheck},
    }
    
    for _, check := range checks {
        if err := check.fn(worker); err != nil {
            return false, fmt.Errorf("%s check failed: %w", check.name, err)
        }
    }
    
    return true, nil
}
```

## Port Allocation

### Port Pool

```go
package supervisor

type PortPool struct {
    mu        sync.Mutex
    available []int
    allocated map[int]bool
}

func NewPortPool(start, end int) *PortPool {
    pool := &PortPool{
        available: make([]int, 0, end-start+1),
        allocated: make(map[int]bool),
    }
    
    // Initialize with available ports
    for port := start; port <= end; port++ {
        pool.available = append(pool.available, port)
    }
    
    return pool
}

func (p *PortPool) Acquire() (int, error) {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    if len(p.available) == 0 {
        return 0, errors.New("no ports available")
    }
    
    // Pop from available
    port := p.available[0]
    p.available = p.available[1:]
    
    // Mark as allocated
    p.allocated[port] = true
    
    return port, nil
}

func (p *PortPool) Release(port int) {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    if !p.allocated[port] {
        return // Already released
    }
    
    delete(p.allocated, port)
    p.available = append(p.available, port)
}

func (p *PortPool) IsAvailable(port int) bool {
    p.mu.Lock()
    defer p.mu.Unlock()
    return !p.allocated[port]
}
```

### Port Configuration

```yaml
# config/server.yaml
workers:
  port_range_start: 10000
  port_range_end: 19999
  # Allows up to 10000 concurrent workers
```

### Port Reuse Strategy

```go
func (s *Supervisor) smartPortAllocation(workerName string) (int, error) {
    // Try to reuse previous port for this worker
    if oldWorker := s.registry.Get(workerName); oldWorker != nil {
        if s.portPool.IsAvailable(oldWorker.Port) {
            return oldWorker.Port, nil
        }
    }
    
    // Allocate new port
    return s.portPool.Acquire()
}
```

## Crash Recovery

### Restart Policies

```go
type RestartPolicy struct {
    MaxRestarts     int           // Max restarts per period
    RestartPeriod   time.Duration // Time window for counting restarts
    BackoffInitial  time.Duration // Initial backoff delay
    BackoffMultiplier float64     // Backoff multiplier
    BackoffMax      time.Duration // Max backoff delay
}

func (s *Supervisor) shouldRestart(worker *Worker) bool {
    policy := worker.Config.RestartPolicy
    
    // Count recent restarts
    recentRestarts := s.countRestarts(worker.Name, policy.RestartPeriod)
    
    if recentRestarts >= policy.MaxRestarts {
        log.Printf("Worker %s exceeded max restarts (%d in %v)",
            worker.Name, policy.MaxRestarts, policy.RestartPeriod)
        return false
    }
    
    return true
}

func (s *Supervisor) calculateBackoff(worker *Worker) time.Duration {
    policy := worker.Config.RestartPolicy
    
    backoff := time.Duration(float64(policy.BackoffInitial) *
        math.Pow(policy.BackoffMultiplier, float64(worker.Restarts)))
    
    if backoff > policy.BackoffMax {
        backoff = policy.BackoffMax
    }
    
    return backoff
}
```

### Example Restart Configuration

```yaml
# config/server.yaml
workers:
  api:
    restart_policy:
      max_restarts: 5        # Max 5 restarts
      restart_period: 5m     # Within 5 minutes
      backoff_initial: 1s    # Start with 1s delay
      backoff_multiplier: 2  # Double each time
      backoff_max: 30s       # Cap at 30s
```

### Restart with Backoff

```go
func (s *Supervisor) restartWithBackoff(workerName string) {
    worker := s.registry.Get(workerName)
    
    if !s.shouldRestart(worker) {
        log.Printf("Not restarting %s (policy limit reached)", workerName)
        return
    }
    
    backoff := s.calculateBackoff(worker)
    log.Printf("Restarting %s after %v backoff", workerName, backoff)
    
    time.Sleep(backoff)
    
    if err := s.StartWorker(workerName); err != nil {
        log.Printf("Failed to restart %s: %v", workerName, err)
    } else {
        worker.Restarts++
        s.recordRestart(workerName, time.Now())
    }
}
```

## Lifecycle Management

### Startup Sequence

```go
func (s *Supervisor) Start() error {
    log.Println("Starting supervisor...")
    
    // 1. Discover workers
    workers, err := s.DiscoverWorkers()
    if err != nil {
        return fmt.Errorf("discovery failed: %w", err)
    }
    log.Printf("Discovered %d workers", len(workers))
    
    // 2. Start workers
    for _, config := range workers {
        if err := s.StartWorker(config.Name); err != nil {
            log.Printf("Failed to start %s: %v", config.Name, err)
            continue
        }
    }
    
    // 3. Wait for workers to be healthy
    time.Sleep(2 * time.Second)
    
    // 4. Start monitoring
    go s.monitorHealth()
    
    // 5. Start file watcher (if enabled)
    if s.config.FileWatcher.Enabled {
        go s.watchFiles()
    }
    
    log.Println("Supervisor started successfully")
    return nil
}
```

### Graceful Shutdown

```go
func (s *Supervisor) Shutdown(ctx context.Context) error {
    log.Println("Shutting down supervisor...")
    
    // Stop accepting new requests
    s.stopAcceptingRequests()
    
    // Get all workers
    workers := s.registry.All()
    
    // Signal all workers to shutdown
    for _, worker := range workers {
        log.Printf("Stopping worker %s", worker.Name)
        worker.Process.Signal(syscall.SIGTERM)
    }
    
    // Wait for workers to shutdown
    done := make(chan struct{})
    go func() {
        for _, worker := range workers {
            worker.Process.Wait()
        }
        close(done)
    }()
    
    // Wait with timeout
    select {
    case <-done:
        log.Println("All workers stopped gracefully")
    case <-ctx.Done():
        log.Println("Shutdown timeout, forcing kill")
        for _, worker := range workers {
            worker.Process.Signal(syscall.SIGKILL)
        }
    }
    
    return nil
}
```

## Implementation

### Complete Supervisor Implementation

```go
package supervisor

type Supervisor struct {
    config      *Config
    registry    *Registry
    portPool    *PortPool
    httpClient  *http.Client
    fileWatcher *watcher.FileWatcher
    
    restartHistory map[string][]time.Time
    mu             sync.RWMutex
}

func NewSupervisor(config *Config) *Supervisor {
    return &Supervisor{
        config:   config,
        registry: NewRegistry(),
        portPool: NewPortPool(
            config.PortPool.Start,
            config.PortPool.End,
        ),
        httpClient: &http.Client{
            Timeout: 5 * time.Second,
        },
        restartHistory: make(map[string][]time.Time),
    }
}

func (s *Supervisor) Run() error {
    // Start supervisor
    if err := s.Start(); err != nil {
        return err
    }
    
    // Wait for interrupt signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    <-sigChan
    
    // Graceful shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    return s.Shutdown(ctx)
}
```

## Best Practices

### Monitoring

```go
// Expose supervisor metrics
func (s *Supervisor) Metrics() SupervisorMetrics {
    workers := s.registry.All()
    
    metrics := SupervisorMetrics{
        TotalWorkers:     len(workers),
        HealthyWorkers:   0,
        UnhealthyWorkers: 0,
        PortsAvailable:   len(s.portPool.available),
        PortsAllocated:   len(s.portPool.allocated),
    }
    
    for _, w := range workers {
        if w.Healthy {
            metrics.HealthyWorkers++
        } else {
            metrics.UnhealthyWorkers++
        }
        metrics.TotalRestarts += w.Restarts
    }
    
    return metrics
}
```

### Logging

```go
// Structured logging
func (s *Supervisor) logWorkerEvent(event, workerName string, details map[string]interface{}) {
    logEntry := map[string]interface{}{
        "timestamp": time.Now().Format(time.RFC3339),
        "component": "supervisor",
        "event":     event,
        "worker":    workerName,
    }
    
    for k, v := range details {
        logEntry[k] = v
    }
    
    log.Printf("%+v", logEntry)
}
```

### Configuration Validation

```go
func (s *Supervisor) ValidateConfig() error {
    if s.config.PortPool.Start >= s.config.PortPool.End {
        return errors.New("invalid port pool range")
    }
    
    if s.config.HealthCheck.Interval < time.Second {
        return errors.New("health check interval too short")
    }
    
    return nil
}
```

## Next Steps

- [Worker Lifecycle](../workers/lifecycle.md) - Detailed worker states
- [Hot Reload System](hot-reload.md) - How supervisor handles reloads
- [Process Isolation](isolation.md) - Why separate processes
- [Health Checks](../workers/health-checks.md) - Implementing health endpoints

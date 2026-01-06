# Worker Lifecycle

- [Introduction](#introduction)
- [Lifecycle States](#lifecycle-states)
- [State Transitions](#state-transitions)
- [Discovery Phase](#discovery-phase)
- [Build Phase](#build-phase)
- [Start Phase](#start-phase)
- [Running Phase](#running-phase)
- [Restart Phase](#restart-phase)
- [Shutdown Phase](#shutdown-phase)
- [Error Handling](#error-handling)

## Introduction

Every worker in TQServer progresses through a well-defined lifecycle from discovery to shutdown. Understanding this lifecycle is essential for building reliable workers and debugging issues.

```
Discovered → Building → Starting → Running → [Restart] → Shutdown
     ↓          ↓          ↓          ↓          ↓           ↓
   Found     Compile   Launch     Serve     Reload       Stop
```

## Lifecycle States

### State Diagram

```
┌──────────────┐
│  DISCOVERED  │  Worker found in filesystem
└──────┬───────┘
       │
       ↓
┌──────────────┐
│   BUILDING   │  Compiling source code
└──────┬───────┘
       │ (success)
       ↓
┌──────────────┐
│   STARTING   │  Process launching, health checking
└──────┬───────┘
       │ (healthy)
       ↓
┌──────────────┐
│   RUNNING    │  Serving requests
└──────┬───────┘
       │
       ├─────────────────────────┐
       │ (file change)           │ (signal/crash)
       ↓                         ↓
┌──────────────┐         ┌──────────────┐
│  RELOADING   │         │  UNHEALTHY   │
└──────┬───────┘         └──────┬───────┘
       │                        │ (restart)
       │                        ↓
       └──────────────→ ┌──────────────┐
                        │  RESTARTING  │
                        └──────┬───────┘
                               │
       ┌───────────────────────┘
       │ (shutdown signal)
       ↓
┌──────────────┐
│  STOPPING    │  Graceful shutdown
└──────┬───────┘
       │
       ↓
┌──────────────┐
│   STOPPED    │  Process terminated
└──────────────┘
```

### State Definitions

```go
package supervisor

type WorkerState string

const (
    StateDiscovered WorkerState = "discovered"  // Found but not started
    StateBuilding   WorkerState = "building"    // Compiling
    StateStarting   WorkerState = "starting"    // Launching process
    StateRunning    WorkerState = "running"     // Healthy and serving
    StateReloading  WorkerState = "reloading"   // Hot reload in progress
    StateUnhealthy  WorkerState = "unhealthy"   // Failed health check
    StateRestarting WorkerState = "restarting"  // Restarting after failure
    StateStopping   WorkerState = "stopping"    // Graceful shutdown
    StateStopped    WorkerState = "stopped"     // Terminated
)
```

## State Transitions

### Valid Transitions

```go
var validTransitions = map[WorkerState][]WorkerState{
    StateDiscovered: {StateBuilding, StateStopped},
    StateBuilding:   {StateStarting, StateStopped},
    StateStarting:   {StateRunning, StateUnhealthy, StateStopped},
    StateRunning:    {StateReloading, StateUnhealthy, StateStopping},
    StateReloading:  {StateRunning, StateUnhealthy},
    StateUnhealthy:  {StateRestarting, StateStopped},
    StateRestarting: {StateBuilding, StateStopped},
    StateStopping:   {StateStopped},
    StateStopped:    {StateDiscovered}, // Can be rediscovered
}

func (w *Worker) CanTransitionTo(newState WorkerState) bool {
    validStates := validTransitions[w.State]
    for _, state := range validStates {
        if state == newState {
            return true
        }
    }
    return false
}
```

### State Transition Implementation

```go
func (w *Worker) TransitionTo(newState WorkerState) error {
    w.mu.Lock()
    defer w.mu.Unlock()
    
    if !w.CanTransitionTo(newState) {
        return fmt.Errorf("invalid transition from %s to %s",
            w.State, newState)
    }
    
    oldState := w.State
    w.State = newState
    w.StateChanged = time.Now()
    
    log.Printf("Worker %s: %s → %s", w.Name, oldState, newState)
    
    // Emit event
    w.emitStateChange(oldState, newState)
    
    return nil
}
```

## Discovery Phase

### What Happens

1. Supervisor scans `workers/` directory
2. Checks for valid worker structure
3. Loads configuration
4. Registers worker in registry

### Implementation

```go
func (s *Supervisor) discoverWorker(path string) (*Worker, error) {
    name := filepath.Base(path)
    
    // Check for binary
    binaryPath := filepath.Join(path, "bin", name)
    if _, err := os.Stat(binaryPath); err != nil {
        return nil, fmt.Errorf("no binary found: %w", err)
    }
    
    // Load config (optional)
    config := s.loadWorkerConfig(path)
    
    // Create worker
    worker := &Worker{
        Name:       name,
        Path:       path,
        BinaryPath: binaryPath,
        Config:     config,
        State:      StateDiscovered,
        Discovered: time.Now(),
    }
    
    return worker, nil
}
```

### Discovery Criteria

**Required**:
- Directory in `workers/`
- Binary at `workers/{name}/bin/{name}`

**Optional**:
- `config.yaml` - Worker configuration
- `src/` - Source code (for building)
- `public/` - Static assets
- `private/` - Templates, views

### Example Worker Structure

```
workers/api/
├── bin/
│   └── api              # Required: executable
├── src/
│   └── main.go          # Optional: source code
├── config.yaml          # Optional: configuration
├── public/              # Optional: static files
└── private/             # Optional: templates
    └── views/
```

## Build Phase

### What Happens

1. Source code compiled
2. Dependencies resolved
3. Binary created/updated
4. Build errors handled

### Build Process

```go
func (s *Supervisor) buildWorker(worker *Worker) error {
    // Transition to building state
    worker.TransitionTo(StateBuilding)
    
    srcPath := filepath.Join(worker.Path, "src")
    outPath := filepath.Join(worker.Path, "bin", worker.Name+".new")
    
    // Build command
    cmd := exec.Command("go", "build",
        "-o", outPath,
        srcPath,
    )
    cmd.Env = append(os.Environ(),
        "CGO_ENABLED=0",
        "GOOS="+runtime.GOOS,
        "GOARCH="+runtime.GOARCH,
    )
    
    // Run build
    output, err := cmd.CombinedOutput()
    if err != nil {
        log.Printf("Build failed for %s:\n%s", worker.Name, output)
        return fmt.Errorf("build failed: %w", err)
    }
    
    // Atomically replace binary
    binaryPath := filepath.Join(worker.Path, "bin", worker.Name)
    if err := os.Rename(outPath, binaryPath); err != nil {
        return fmt.Errorf("failed to replace binary: %w", err)
    }
    
    worker.BinaryPath = binaryPath
    worker.LastBuild = time.Now()
    
    return nil
}
```

### Build Configuration

```yaml
# config.yaml
build:
  command: "go build -o bin/api src/*.go"
  timeout: 30s
  env:
    CGO_ENABLED: "0"
    GOOS: "linux"
```

### Build Artifacts

```
workers/api/bin/
├── api              # Current binary
├── api.new          # Building (temp)
└── api.1704556800   # Previous version (backup)
```

## Start Phase

### What Happens

1. Port allocated from pool
2. Process launched
3. Environment variables set
4. Health checks begin
5. Worker registered as running

### Start Implementation

```go
func (s *Supervisor) startWorker(worker *Worker) error {
    // Transition to starting
    worker.TransitionTo(StateStarting)
    
    // Allocate port
    port, err := s.portPool.Acquire()
    if err != nil {
        return fmt.Errorf("no ports available: %w", err)
    }
    
    // Prepare command
    cmd := exec.Command(worker.BinaryPath)
    cmd.Env = append(os.Environ(),
        fmt.Sprintf("PORT=%d", port),
        fmt.Sprintf("WORKER_NAME=%s", worker.Name),
        fmt.Sprintf("WORKER_PATH=%s", worker.Path),
    )
    cmd.Dir = worker.Path
    cmd.Stdout = worker.LogFile
    cmd.Stderr = worker.LogFile
    
    // Start process
    if err := cmd.Start(); err != nil {
        s.portPool.Release(port)
        return fmt.Errorf("failed to start process: %w", err)
    }
    
    // Update worker
    worker.Process = cmd.Process
    worker.PID = cmd.Process.Pid
    worker.Port = port
    worker.Started = time.Now()
    
    // Wait for health
    if err := s.waitForHealthy(worker, 30*time.Second); err != nil {
        s.stopWorker(worker)
        return fmt.Errorf("worker failed to become healthy: %w", err)
    }
    
    // Transition to running
    worker.TransitionTo(StateRunning)
    
    log.Printf("Worker %s started on port %d (PID %d)",
        worker.Name, port, cmd.Process.Pid)
    
    return nil
}
```

### Health Check Wait

```go
func (s *Supervisor) waitForHealthy(worker *Worker, timeout time.Duration) error {
    client := &http.Client{Timeout: 2 * time.Second}
    deadline := time.Now().Add(timeout)
    
    attempt := 0
    for time.Now().Before(deadline) {
        attempt++
        
        resp, err := client.Get(
            fmt.Sprintf("http://localhost:%d/health", worker.Port),
        )
        
        if err == nil && resp.StatusCode == 200 {
            resp.Body.Close()
            log.Printf("Worker %s healthy after %d attempts", worker.Name, attempt)
            return nil
        }
        
        if resp != nil {
            resp.Body.Close()
        }
        
        time.Sleep(100 * time.Millisecond)
    }
    
    return fmt.Errorf("health check timeout after %v", timeout)
}
```

## Running Phase

### What Happens

1. Worker serves requests
2. Continuous health monitoring
3. Metrics collected
4. Logs written

### Running State Management

```go
func (s *Supervisor) monitorRunningWorker(worker *Worker) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            if worker.State != StateRunning {
                return // Not running anymore
            }
            
            // Health check
            healthy, err := s.checkHealth(worker)
            if !healthy || err != nil {
                log.Printf("Worker %s unhealthy: %v", worker.Name, err)
                worker.TransitionTo(StateUnhealthy)
                s.handleUnhealthyWorker(worker)
                return
            }
            
            // Update metrics
            worker.LastSeen = time.Now()
            worker.RequestCount = s.getRequestCount(worker)
            
        case <-worker.StopChan:
            return // Shutdown requested
        }
    }
}
```

### Request Handling

```go
// Inside worker process
func (w *Worker) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
    // Track request
    atomic.AddInt64(&w.requestCount, 1)
    start := time.Now()
    
    defer func() {
        duration := time.Since(start)
        w.recordMetric("request_duration", duration)
    }()
    
    // Handle request
    w.router.ServeHTTP(rw, req)
}
```

## Restart Phase

### Restart Triggers

1. **File Change** (hot reload)
2. **Health Check Failure**
3. **Process Crash**
4. **Manual Restart**
5. **Resource Limit Exceeded**

### Hot Reload Restart

```go
func (s *Supervisor) hotReload(worker *Worker) error {
    log.Printf("Hot reloading worker %s", worker.Name)
    
    // Transition to reloading
    worker.TransitionTo(StateReloading)
    
    // Build new version
    if err := s.buildWorker(worker); err != nil {
        worker.TransitionTo(StateRunning) // Rollback
        return fmt.Errorf("build failed: %w", err)
    }
    
    // Start new instance (on different port)
    newWorker := worker.Clone()
    if err := s.startWorker(newWorker); err != nil {
        worker.TransitionTo(StateRunning) // Rollback
        return fmt.Errorf("failed to start new instance: %w", err)
    }
    
    // Swap traffic
    s.router.SwapWorker(worker.Name, newWorker)
    
    // Graceful shutdown of old worker
    go s.gracefulShutdown(worker, 30*time.Second)
    
    // Update registry
    s.registry.Replace(worker.Name, newWorker)
    newWorker.TransitionTo(StateRunning)
    
    return nil
}
```

### Crash Restart

```go
func (s *Supervisor) handleCrash(worker *Worker) {
    log.Printf("Worker %s crashed (PID %d)", worker.Name, worker.PID)
    
    // Transition to restarting
    worker.TransitionTo(StateRestarting)
    
    // Check restart policy
    if !s.shouldRestart(worker) {
        log.Printf("Not restarting %s (policy limit)", worker.Name)
        worker.TransitionTo(StateStopped)
        return
    }
    
    // Calculate backoff
    backoff := s.calculateBackoff(worker)
    log.Printf("Restarting %s after %v", worker.Name, backoff)
    time.Sleep(backoff)
    
    // Clean up old process
    s.cleanup(worker)
    
    // Start new instance
    if err := s.startWorker(worker); err != nil {
        log.Printf("Restart failed for %s: %v", worker.Name, err)
        worker.TransitionTo(StateStopped)
    }
}
```

## Shutdown Phase

### Graceful Shutdown

```go
func (s *Supervisor) gracefulShutdown(worker *Worker, timeout time.Duration) error {
    // Transition to stopping
    worker.TransitionTo(StateStopping)
    
    log.Printf("Gracefully shutting down worker %s", worker.Name)
    
    // Send SIGTERM
    if err := worker.Process.Signal(syscall.SIGTERM); err != nil {
        return fmt.Errorf("failed to send SIGTERM: %w", err)
    }
    
    // Wait with timeout
    done := make(chan error, 1)
    go func() {
        _, err := worker.Process.Wait()
        done <- err
    }()
    
    select {
    case err := <-done:
        // Clean shutdown
        log.Printf("Worker %s stopped gracefully", worker.Name)
        worker.TransitionTo(StateStopped)
        s.cleanup(worker)
        return err
        
    case <-time.After(timeout):
        // Timeout - force kill
        log.Printf("Worker %s shutdown timeout, force killing", worker.Name)
        worker.Process.Signal(syscall.SIGKILL)
        worker.Process.Wait()
        worker.TransitionTo(StateStopped)
        s.cleanup(worker)
        return fmt.Errorf("shutdown timeout")
    }
}
```

### Cleanup

```go
func (s *Supervisor) cleanup(worker *Worker) {
    // Release port
    if worker.Port > 0 {
        s.portPool.Release(worker.Port)
        worker.Port = 0
    }
    
    // Close log files
    if worker.LogFile != nil {
        worker.LogFile.Close()
        worker.LogFile = nil
    }
    
    // Unregister from router
    s.router.Unregister(worker.Name)
    
    // Clear PID
    worker.PID = 0
    worker.Process = nil
}
```

## Error Handling

### Build Errors

```go
func (s *Supervisor) handleBuildError(worker *Worker, err error) {
    log.Printf("Build error for %s: %v", worker.Name, err)
    
    // Keep old version running if available
    if worker.State == StateRunning {
        log.Printf("Keeping old version of %s running", worker.Name)
        return
    }
    
    // Mark as stopped
    worker.TransitionTo(StateStopped)
    
    // Notify monitoring
    s.emitEvent("build_failed", worker.Name, err)
}
```

### Startup Errors

```go
func (s *Supervisor) handleStartupError(worker *Worker, err error) {
    log.Printf("Startup error for %s: %v", worker.Name, err)
    
    // Clean up
    s.cleanup(worker)
    
    // Check if should retry
    if worker.StartAttempts < 3 {
        worker.StartAttempts++
        backoff := time.Duration(worker.StartAttempts) * time.Second
        
        log.Printf("Retrying start of %s after %v", worker.Name, backoff)
        time.Sleep(backoff)
        
        s.startWorker(worker)
    } else {
        log.Printf("Max start attempts reached for %s", worker.Name)
        worker.TransitionTo(StateStopped)
    }
}
```

### Runtime Errors

```go
func (s *Supervisor) handleRuntimeError(worker *Worker, err error) {
    log.Printf("Runtime error for %s: %v", worker.Name, err)
    
    // Check error type
    switch {
    case errors.Is(err, ErrHealthCheckFailed):
        worker.TransitionTo(StateUnhealthy)
        s.handleUnhealthyWorker(worker)
        
    case errors.Is(err, ErrProcessDied):
        worker.TransitionTo(StateRestarting)
        s.handleCrash(worker)
        
    default:
        log.Printf("Unknown error type for %s: %v", worker.Name, err)
    }
}
```

## Best Practices

### Implement Health Endpoint

```go
func healthHandler(w http.ResponseWriter, r *http.Request) {
    // Check dependencies
    status := map[string]string{}
    
    if err := db.Ping(); err != nil {
        status["database"] = "unhealthy"
        w.WriteHeader(503)
    } else {
        status["database"] = "healthy"
    }
    
    if err := cache.Ping(); err != nil {
        status["cache"] = "unhealthy"
        w.WriteHeader(503)
    } else {
        status["cache"] = "healthy"
    }
    
    json.NewEncoder(w).Encode(status)
}
```

### Handle Signals Gracefully

```go
func main() {
    // Setup signal handling
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
    
    server := &http.Server{Addr: ":"+os.Getenv("PORT")}
    
    go func() {
        <-sigChan
        log.Println("Shutdown signal received")
        
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        
        if err := server.Shutdown(ctx); err != nil {
            log.Printf("Shutdown error: %v", err)
        }
    }()
    
    server.ListenAndServe()
}
```

### Log State Transitions

```go
func (w *Worker) TransitionTo(newState WorkerState) error {
    oldState := w.State
    
    if err := w.validateTransition(newState); err != nil {
        return err
    }
    
    w.State = newState
    w.StateHistory = append(w.StateHistory, StateTransition{
        From:      oldState,
        To:        newState,
        Timestamp: time.Now(),
    })
    
    log.Printf("[%s] State: %s → %s", w.Name, oldState, newState)
    
    return nil
}
```

## Next Steps

- [Worker Configuration](configuration.md) - Configure worker behavior
- [Building Workers](building.md) - Build system details
- [Health Checks](health-checks.md) - Implement health endpoints
- [Supervisor Pattern](../architecture/supervisor.md) - How supervisor manages lifecycle

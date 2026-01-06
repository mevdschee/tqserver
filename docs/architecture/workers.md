# Worker Architecture

- [Introduction](#introduction)
- [Worker Process Model](#worker-process-model)
- [Communication Protocols](#communication-protocols)
- [Resource Management](#resource-management)
- [State Management](#state-management)
- [Worker Isolation](#worker-isolation)
- [Lifecycle Management](#lifecycle-management)

## Introduction

TQServer's worker architecture is based on the principle of **process isolation** where each worker runs as an independent process. This design provides strong isolation boundaries, enables independent scaling, and ensures that failures in one worker don't affect others.

## Worker Process Model

### One Process Per Worker

Each worker is a standalone Go binary that runs in its own process:

```
┌─────────────────────────────────────────────────────────┐
│                      TQServer                           │
│  ┌────────────┐   ┌────────────┐   ┌────────────┐       │
│  │ Supervisor │   │   Router   │   │   Proxy    │       │
│  └──────┬─────┘   └──────┬─────┘   └──────┬─────┘       │
└─────────┼────────────────┼────────────────┼─────────────┘
          │                │                │
    ┌─────┴───────┬────────┴────┬───────────┴─────┐
    │             │             │                 │
┌───▼───┐     ┌───▼───┐     ┌───▼───┐         ┌───▼───┐
│Worker │     │Worker │     │Worker │         │Worker │
│ PID   │     │ PID   │     │ PID   │         │ PID   │
│ 1234  │     │ 1235  │     │ 1236  │         │ 1237  │
│Port   │     │Port   │     │Port   │         │Port   │
│ 9000  │     │ 9001  │     │ 9002  │         │ 9003  │
└───────┘     └───────┘     └───────┘         └───────┘
  index          api         admin              blog
```

### Worker Registry

The supervisor maintains a registry of all running workers:

```go
type WorkerInstance struct {
    Name      string
    Route     string
    PID       int
    Port      int
    StartedAt time.Time
    
    // File tracking
    BinaryPath   string
    BinaryMtime  time.Time
    PublicPath   string
    PublicMtime  time.Time
    ViewsPath    string
    ViewsMtime   time.Time
    
    // Health
    Status          string
    LastHealthCheck time.Time
}
```

### Process Characteristics

**Independent Execution**:
- Each worker has its own process ID (PID)
- Own memory space
- Own goroutine scheduler
- Own garbage collector

**Isolated Failures**:
- Worker crash doesn't affect other workers
- Worker panic is contained
- Memory leaks isolated
- Resource exhaustion contained

**Individual Scaling**:
- Each worker can be configured independently
- Different resource limits per worker
- Worker-specific timeouts
- Independent restart policies

## Communication Protocols

### HTTP Communication

Workers communicate via HTTP:

```
Client → TQServer (port 8080) → Worker (port 9000+)
```

**Request Flow**:
1. Client sends HTTP request to TQServer
2. Router determines target worker
3. Proxy forwards request to worker port
4. Worker processes request
5. Response proxied back to client

**Example Worker Server**:
```go
package main

import (
    "log"
    "net/http"
    "os"
)

func main() {
    // Port assigned by TQServer
    port := os.Getenv("WORKER_PORT")
    
    // Register handlers
    http.HandleFunc("/", handleRequest)
    http.HandleFunc("/health", handleHealth)
    
    // Start HTTP server
    log.Printf("Worker listening on port %s", port)
    log.Fatal(http.ListenAndServe(":"+port, nil))
}
```

### Environment Variables

TQServer communicates configuration to workers via environment variables:

```bash
WORKER_PORT=9000        # Assigned port
WORKER_NAME=api         # Worker name
WORKER_ROUTE=/api       # Worker route
WORKER_MODE=development # Deployment mode
```

**Accessing in Worker**:
```go
workerPort := os.Getenv("WORKER_PORT")
workerName := os.Getenv("WORKER_NAME")
workerRoute := os.Getenv("WORKER_ROUTE")
workerMode := os.Getenv("WORKER_MODE")
```

### Signal Handling

Workers receive signals for lifecycle management:

- **SIGTERM**: Graceful shutdown
- **SIGINT**: Interrupt (graceful)
- **SIGKILL**: Force kill (last resort)

**Graceful Shutdown Handler**:
```go
func main() {
    server := &http.Server{Addr: ":"+port}
    
    // Start server in goroutine
    go func() {
        if err := server.ListenAndServe(); err != http.ErrServerClosed {
            log.Fatal(err)
        }
    }()
    
    // Wait for shutdown signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
    <-quit
    
    // Graceful shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if err := server.Shutdown(ctx); err != nil {
        log.Printf("Shutdown error: %v", err)
    }
}
```

## Resource Management

### Memory Management

Each worker has its own memory space:

```yaml
# Worker-specific limits in config/worker.yaml
runtime:
  go_mem_limit: "512MiB"
  go_max_procs: 2
```

**Advantages**:
- Independent garbage collection
- No memory contention
- Clear memory attribution
- Isolated leaks

### CPU Allocation

Workers can have individual CPU limits:

```yaml
runtime:
  go_max_procs: 2  # Use 2 cores
```

### Connection Pools

Workers maintain their own connection pools:

```go
// Database connection pool per worker
db, err := sql.Open("postgres", connString)
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)

// Redis pool per worker
redisPool := &redis.Pool{
    MaxIdle: 10,
    MaxActive: 100,
}
```

## State Management

### Stateless Workers

Workers should be stateless by default:

**Good** (Stateless):
```go
func handleRequest(w http.ResponseWriter, r *http.Request) {
    // Get session from store
    session := getSessionFromRedis(r)
    
    // Process request
    result := processRequest(session, r)
    
    // Return response
    json.NewEncoder(w).Encode(result)
}
```

**Bad** (Stateful):
```go
var cache = make(map[string]interface{}) // Shared state

func handleRequest(w http.ResponseWriter, r *http.Request) {
    // State is lost on restart!
    cache[key] = value
}
```

### External State Stores

Use external stores for shared state:

**Redis for Session**:
```go
import "github.com/gomodule/redigo/redis"

func getSession(sessionID string) (*Session, error) {
    conn := redisPool.Get()
    defer conn.Close()
    
    data, err := redis.String(conn.Do("GET", "session:"+sessionID))
    if err != nil {
        return nil, err
    }
    
    var session Session
    json.Unmarshal([]byte(data), &session)
    return &session, nil
}
```

**Database for Persistent Data**:
```go
func getUser(userID int) (*User, error) {
    var user User
    err := db.QueryRow(
        "SELECT id, name, email FROM users WHERE id = $1",
        userID,
    ).Scan(&user.ID, &user.Name, &user.Email)
    return &user, err
}
```

### In-Memory Caching

Workers can cache data locally:

```go
import "github.com/patrickmn/go-cache"

var localCache = cache.New(5*time.Minute, 10*time.Minute)

func getCachedUser(userID int) (*User, error) {
    // Check cache
    if cached, found := localCache.Get(fmt.Sprintf("user:%d", userID)); found {
        return cached.(*User), nil
    }
    
    // Fetch from database
    user, err := getUser(userID)
    if err != nil {
        return nil, err
    }
    
    // Cache result
    localCache.Set(fmt.Sprintf("user:%d", userID), user, cache.DefaultExpiration)
    return user, nil
}
```

**Note**: Cache is per-worker and lost on restart.

## Worker Isolation

### Process Boundaries

Strong isolation through OS process boundaries:

- **Memory**: No shared memory between workers
- **CPU**: Separate CPU allocation
- **Files**: Separate file descriptors
- **Network**: Separate network sockets

### Security Implications

Process isolation provides security benefits:

**Privilege Separation**:
```bash
# Different users for different workers
User=worker-api
Group=worker-api

User=worker-admin
Group=worker-admin
```

**Resource Limits**:
```ini
[Service]
LimitNOFILE=1024
LimitNPROC=512
MemoryMax=512M
CPUQuota=100%
```

### Failure Isolation

Worker failures are contained:

```
Worker A crashes → Only Worker A affected
Worker B continues → No impact
TQServer restarts Worker A → Service restored
```

**Example**:
```go
// Panic in one worker doesn't affect others
func handleRequest(w http.ResponseWriter, r *http.Request) {
    defer func() {
        if err := recover(); err != nil {
            log.Printf("Panic recovered: %v", err)
            http.Error(w, "Internal Server Error", 500)
        }
    }()
    
    // Risky operation
    result := riskyOperation()
    json.NewEncoder(w).Encode(result)
}
```

## Lifecycle Management

### Worker Lifecycle States

```
┌──────────┐     ┌──────────┐     ┌──────────┐
│ Starting │────▶│ Healthy  │────▶│ Stopping │
└──────────┘     └────┬─────┘     └──────────┘
      ▲                │                │
      │                ▼                │
      │           ┌──────────┐         │
      └───────────│ Unhealthy│◀────────┘
                  └──────────┘
```

### State Transitions

**Starting → Healthy**:
- Binary starts
- HTTP server listening
- Health check passes

**Healthy → Unhealthy**:
- Health check fails
- Worker stops responding
- Process exits unexpectedly

**Unhealthy → Starting**:
- Supervisor detects failure
- Old process killed
- New process started

**Healthy/Unhealthy → Stopping**:
- Shutdown signal sent
- Graceful shutdown initiated
- Process terminates cleanly

### Supervisor Responsibilities

The supervisor manages worker lifecycle:

1. **Discovery**: Find workers in filesystem
2. **Building**: Compile worker binaries
3. **Starting**: Launch worker processes
4. **Monitoring**: Health checks
5. **Restarting**: Graceful restarts on changes
6. **Stopping**: Clean shutdown

## Best Practices

### Worker Design

1. **Keep workers focused**: One responsibility per worker
2. **Stateless design**: Use external stores for state
3. **Graceful shutdown**: Handle SIGTERM properly
4. **Health checks**: Implement `/health` endpoint
5. **Error handling**: Recover from panics gracefully
6. **Logging**: Use structured logging
7. **Resource cleanup**: Close connections on shutdown

### Performance

1. **Connection pooling**: Reuse database connections
2. **Caching**: Cache frequently accessed data
3. **Efficient handlers**: Minimize per-request work
4. **Resource limits**: Set appropriate limits
5. **Monitoring**: Track metrics and performance

### Security

1. **Input validation**: Validate all inputs
2. **Output encoding**: Prevent XSS
3. **Authentication**: Verify user identity
4. **Authorization**: Check permissions
5. **Rate limiting**: Prevent abuse
6. **Error messages**: Don't leak information

## Next Steps

- [Process Isolation](isolation.md) - Deep dive into isolation
- [Hot Reload System](hot-reload.md) - How hot reload works
- [Supervisor Pattern](supervisor.md) - Supervisor implementation
- [Creating Workers](../workers/creating.md) - Build your own workers

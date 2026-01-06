# Process Isolation

- [Introduction](#introduction)
- [Isolation Boundaries](#isolation-boundaries)
- [Security Benefits](#security-benefits)
- [Resource Isolation](#resource-isolation)
- [Failure Containment](#failure-containment)
- [Performance Implications](#performance-implications)
- [Trade-offs](#trade-offs)

## Introduction

Process isolation is a fundamental architectural principle in TQServer. By running each worker as a separate OS process, TQServer provides strong isolation boundaries that enhance security, reliability, and resource management.

## Isolation Boundaries

### Memory Isolation

Each worker process has its own memory space:

```
┌─────────────────────────────────────────────────┐
│           Operating System                       │
├─────────────────────────────────────────────────┤
│  Process 1      Process 2      Process 3        │
│  ┌─────────┐   ┌─────────┐   ┌─────────┐       │
│  │ Worker  │   │ Worker  │   │ Worker  │       │
│  │ Memory  │   │ Memory  │   │ Memory  │       │
│  │         │   │         │   │         │       │
│  │ Heap    │   │ Heap    │   │ Heap    │       │
│  │ Stack   │   │ Stack   │   │ Stack   │       │
│  │ Globals │   │ Globals │   │ Globals │       │
│  └─────────┘   └─────────┘   └─────────┘       │
└─────────────────────────────────────────────────┘
```

**Implications**:
- No shared memory between workers
- Cannot accidentally access another worker's data
- Memory leaks contained to single worker
- Independent garbage collection

**Example - No Shared State**:
```go
// Worker A
var cache = make(map[string]string)
cache["key"] = "value from A"

// Worker B (different process)
var cache = make(map[string]string)
// cache["key"] does NOT exist here
// Each worker has its own copy
```

### CPU Isolation

Workers run independently on CPU cores:

```yaml
# Per-worker CPU limits
worker:
  resources:
    max_cpu: 1.5  # 1.5 CPU cores
```

**Benefits**:
- CPU-intensive worker doesn't starve others
- Fair scheduling by OS
- Can prioritize critical workers
- Independent CPU accounting

### File Descriptor Isolation

Each process has its own file descriptor table:

```go
// Worker A opens file
file1, _ := os.Open("data.txt")  // FD 3

// Worker B opens same file
file2, _ := os.Open("data.txt")  // Also FD 3, but different table

// No conflict - separate file descriptor tables
```

**Advantages**:
- No FD exhaustion across workers
- Independent connection limits
- Isolated socket management
- No FD leaks between workers

### Network Isolation

Each worker listens on its own port:

```
Worker A → Port 9000
Worker B → Port 9001
Worker C → Port 9002
```

**Benefits**:
- No port conflicts
- Independent connection handling
- Separate network buffers
- Isolated network errors

## Security Benefits

### Privilege Separation

Run workers with different users/permissions:

```ini
# systemd service for API worker
[Service]
User=api-worker
Group=api-worker
ReadOnlyPaths=/opt/tqserver/config
ReadWritePaths=/var/log/api

# systemd service for admin worker
[Service]
User=admin-worker
Group=admin-worker
ReadOnlyPaths=/opt/tqserver/config
ReadWritePaths=/var/log/admin /opt/admin/uploads
```

**Security Model**:
- Admin worker can write to admin uploads
- API worker cannot access admin uploads
- Each worker has minimal permissions
- Compromised worker has limited access

### Attack Surface Reduction

Isolation limits attack impact:

```
Scenario: SQL Injection in API Worker
┌─────────────────────────────────────────┐
│ Attacker exploits API worker            │
│ ✓ Can access API worker's memory       │
│ ✓ Can read API worker's files          │
│ ✗ CANNOT access admin worker           │
│ ✗ CANNOT access other worker's memory  │
│ ✗ CANNOT escalate to TQServer process  │
└─────────────────────────────────────────┘
```

### Sandboxing

Use Linux namespaces for additional isolation:

```bash
# Run worker in isolated namespace
unshare --net --pid --mount \
    --map-root-user \
    --fork \
    /opt/tqserver/workers/api/bin/api
```

**Isolation Features**:
- **PID namespace**: Worker sees only its own processes
- **Network namespace**: Isolated network stack
- **Mount namespace**: Separate filesystem view
- **User namespace**: UID/GID mapping

### Seccomp Filters

Restrict system calls available to workers:

```json
{
  "defaultAction": "SCMP_ACT_ERRNO",
  "architectures": ["SCMP_ARCH_X86_64"],
  "syscalls": [
    {
      "names": ["read", "write", "open", "close", "stat"],
      "action": "SCMP_ACT_ALLOW"
    }
  ]
}
```

## Resource Isolation

### Memory Limits

Enforce per-worker memory limits:

```yaml
# config/server.yaml
workers:
  api:
    resources:
      max_memory: "512M"
  admin:
    resources:
      max_memory: "256M"
```

**Using cgroups**:
```bash
# Create cgroup for worker
cgcreate -g memory:/tqserver/workers/api
echo 536870912 > /sys/fs/cgroup/memory/tqserver/workers/api/memory.limit_in_bytes

# Run worker in cgroup
cgexec -g memory:/tqserver/workers/api /opt/tqserver/workers/api/bin/api
```

**Benefits**:
- Memory-hungry worker can't OOM the system
- Predictable memory usage
- Early detection of memory leaks
- Guaranteed memory for critical workers

### CPU Quotas

Limit CPU usage per worker:

```yaml
worker:
  resources:
    max_cpu: 2.0     # 2 cores max
    cpu_shares: 1024 # Priority weight
```

**Systemd Implementation**:
```ini
[Service]
CPUQuota=200%        # 2 cores
CPUWeight=100        # Default priority
```

**Enforcement**:
- Worker cannot monopolize CPU
- Fair scheduling among workers
- Consistent performance
- Cost attribution

### Disk I/O Limits

Control disk access per worker:

```yaml
worker:
  resources:
    max_iops: 1000      # I/O operations per second
    max_bandwidth: 10M   # Bytes per second
```

**Using blkio cgroup**:
```bash
# Limit I/O for worker
echo "8:0 10485760" > /sys/fs/cgroup/blkio/tqserver/api/blkio.throttle.read_bps_device
```

### Network Bandwidth

Limit network bandwidth per worker:

```bash
# Traffic control (tc) to limit bandwidth
tc qdisc add dev eth0 root handle 1: htb
tc class add dev eth0 parent 1: classid 1:1 htb rate 10mbit
tc filter add dev eth0 protocol ip parent 1: prio 1 u32 \
    match ip sport 9000 0xffff flowid 1:1
```

## Failure Containment

### Crash Isolation

Worker crash doesn't affect others:

```go
// Worker A panics
func riskyOperation() {
    panic("Something went wrong!")
}

// Result:
// ✓ Worker A process exits
// ✓ TQServer detects failure
// ✓ TQServer restarts Worker A
// ✗ Worker B, C, D unaffected
// ✗ TQServer continues running
```

### Panic Recovery

Even within a worker, isolate panics:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    defer func() {
        if err := recover(); err != nil {
            log.Printf("Panic recovered: %v", err)
            debug.PrintStack()
            
            // Return error to client
            http.Error(w, "Internal Server Error", 500)
            
            // Worker continues running
        }
    }()
    
    // Potentially panicking code
    riskyOperation()
}
```

### Memory Leak Isolation

Memory leak in one worker doesn't affect others:

```go
// Worker A has a memory leak
var leakyCache = make(map[string][]byte)

func handler(w http.ResponseWriter, r *http.Request) {
    // Leak: never cleaned up
    leakyCache[uuid.New().String()] = make([]byte, 1024*1024)
}

// Result:
// ✓ Only Worker A memory grows
// ✓ Other workers unaffected
// ✓ Monitor detects high memory usage
// ✓ Worker A can be restarted independently
```

### Deadlock Isolation

Deadlock in one worker doesn't freeze system:

```go
// Worker A deadlocks
var mu1, mu2 sync.Mutex

func deadlockingFunction() {
    mu1.Lock()
    time.Sleep(100 * time.Millisecond)
    mu2.Lock()  // Deadlock!
    // Never unlocks
}

// Result:
// ✓ Worker A stops responding
// ✓ Health check fails
// ✓ Supervisor restarts Worker A
// ✗ Other workers continue normally
```

### Resource Exhaustion Containment

File descriptor exhaustion isolated:

```go
// Worker A exhausts file descriptors
func leakyHandler(w http.ResponseWriter, r *http.Request) {
    file, _ := os.Open("data.txt")
    // Forgot to close - leak!
    // After 1024 files, Worker A hits limit
}

// Result:
// ✓ Worker A cannot open more files
// ✓ Worker A returns errors
// ✗ Worker B can still open files
// ✗ System not affected
```

## Performance Implications

### Advantages

**Independent Scaling**:
```yaml
# Scale workers independently
workers:
  api:
    instances: 4      # High traffic
  admin:
    instances: 1      # Low traffic
  reports:
    instances: 2      # Periodic load
```

**Parallel Execution**:
- Workers truly run in parallel
- No GIL (like Python)
- Full multi-core utilization
- Independent goroutine schedulers

**Cache Locality**:
- Each worker has hot caches
- No cache contention
- Better CPU cache usage
- Predictable performance

### Overhead

**Process Creation Cost**:
```
fork() + exec():  ~5-10ms
Thread creation:  ~0.1ms
Goroutine:        ~0.001ms
```

**Mitigation**:
- Workers stay running (not created per-request)
- One-time startup cost
- Negligible impact on throughput

**Memory Overhead**:
```
Process overhead: ~2-5 MB per process
Thread overhead:  ~2 KB per thread
Goroutine:       ~2 KB per goroutine
```

**Mitigation**:
- Modern systems handle hundreds of processes easily
- Memory is cheap compared to isolation benefits
- Workers share binary pages (copy-on-write)

**Context Switching**:
```
Process switch:  ~1-10 µs
Thread switch:   ~0.1-1 µs
```

**Mitigation**:
- Workers are long-lived
- Minimal switching between workers
- OS efficiently schedules processes

### Benchmarks

Real-world performance comparison:

```
Test: 10,000 requests across 4 workers

Process-based (TQServer):
- Requests/sec: 8,500
- Latency (p50): 12ms
- Latency (p99): 45ms
- Memory: 180MB

Thread-based:
- Requests/sec: 9,200 (+8%)
- Latency (p50): 11ms
- Latency (p99): 42ms
- Memory: 120MB (-33%)

Verdict: 8% slower, 50% more memory
Trade-off: Strong isolation + better reliability
```

## Trade-offs

### Advantages

✅ **Strong Isolation**
- Security boundaries
- Failure containment
- Resource isolation

✅ **Reliability**
- Crash resilience
- Independent restarts
- No cascading failures

✅ **Operational**
- Easy debugging (separate processes)
- Clear resource attribution
- Simple deployment model

✅ **Scalability**
- Independent scaling
- No shared state contention
- Horizontal scaling ready

### Disadvantages

❌ **Memory Overhead**
- Each process has overhead
- Cannot share memory
- Higher baseline memory

❌ **IPC Complexity**
- Network calls for communication
- Serialization overhead
- Latency between workers

❌ **State Management**
- Must use external stores
- Cannot use in-process cache sharing
- Session management complexity

### When Process Isolation Works Best

**Ideal For**:
- Microservices architecture
- Multi-tenant systems
- Security-critical applications
- Long-running workers
- Independent components

**Not Ideal For**:
- High-frequency inter-worker communication
- Shared in-memory state requirements
- Extremely latency-sensitive (µs level)
- Resource-constrained environments

## Best Practices

### Design for Isolation

```go
// Good: Stateless, isolated
func handleRequest(w http.ResponseWriter, r *http.Request) {
    session := getSessionFromRedis(r)
    user := getUserFromDB(session.UserID)
    processRequest(user, r)
}

// Bad: Shared state
var globalCache = make(map[string]interface{})
func handleRequest(w http.ResponseWriter, r *http.Request) {
    if cached, ok := globalCache[key]; ok {
        // This cache is lost on restart!
    }
}
```

### Use External State

```go
// Use Redis for shared state
func setUserSession(userID int, data SessionData) error {
    conn := redisPool.Get()
    defer conn.Close()
    
    bytes, _ := json.Marshal(data)
    _, err := conn.Do("SETEX", fmt.Sprintf("session:%d", userID), 3600, bytes)
    return err
}
```

### Monitor Per-Worker Metrics

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "worker_request_duration_seconds",
            Help: "Request duration per worker",
        },
        []string{"worker"},
    )
)

func handler(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    defer func() {
        requestDuration.WithLabelValues(os.Getenv("WORKER_NAME")).
            Observe(time.Since(start).Seconds())
    }()
    
    // Handle request
}
```

### Set Resource Limits

```yaml
# Always set limits
workers:
  api:
    resources:
      max_memory: "512M"
      max_cpu: 2.0
      max_open_files: 1024
    
  admin:
    resources:
      max_memory: "256M"
      max_cpu: 1.0
      max_open_files: 512
```

## Next Steps

- [Worker Architecture](workers.md) - Deep dive into worker design
- [Hot Reload System](hot-reload.md) - How isolation enables hot reload
- [Supervisor Pattern](supervisor.md) - Managing isolated workers
- [Security](../security/authentication.md) - Security best practices

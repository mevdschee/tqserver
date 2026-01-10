# Prometheus Metrics Endpoint Implementation Plan

Add a `/metrics` endpoint to TQServer that exposes Prometheus-compatible metrics, similar to those provided by HAProxy and Traefik.

## User Review Required

> [!IMPORTANT]
> This implementation focuses solely on the **metrics endpoint**. The Prometheus scraper is to be implemented as a separate worker with its own route, as specified.

> [!NOTE]
> The endpoint will be exposed on the main server port (8080 by default) at `/metrics`. Consider whether this should be configurable or on a separate admin port.

---

## Proposed Changes

### Metrics Module

#### [NEW] [metrics.go](file:///home/maurits/projects/tqserver/server/src/metrics.go)

Create a new metrics module using the official Prometheus Go client library. This file will:

1. **Define metric collectors**:
   - Process/Global counters (similar to HAProxy process metrics)
   - Frontend/Request counters (proxy-level metrics)
   - Backend/Worker counters (per-worker metrics)
   - Histogram for request duration (latency distribution)

2. **Metric definitions** (following HAProxy/Traefik naming conventions):

   **Process Metrics:**
   | Metric Name | Type | Labels | Description |
   |-------------|------|--------|-------------|
   | `tqserver_process_start_time_seconds` | Gauge | - | Unix timestamp of process start |
   | `tqserver_process_uptime_seconds` | Gauge | - | Number of seconds since process started |
   | `tqserver_process_memory_bytes` | Gauge | `type` | Memory usage in bytes (labels: `heap`, `stack`, `sys`) |
   | `tqserver_process_goroutines` | Gauge | - | Number of active goroutines |

   **Frontend/Proxy Metrics:**
   | Metric Name | Type | Labels | Description |
   |-------------|------|--------|-------------|
   | `tqserver_requests_total` | Counter | `method`, `path`, `status` | Total HTTP requests received |
   | `tqserver_http_responses_total` | Counter | `status_group` | Total responses per status group (`1xx`, `2xx`, `3xx`, `4xx`, `5xx`) |
   | `tqserver_request_duration_seconds` | Histogram | `method`, `path` | Request latency in seconds |
   | `tqserver_bytes_in_total` | Counter | - | Total incoming bytes |
   | `tqserver_bytes_out_total` | Counter | - | Total outgoing bytes |
   | `tqserver_connections_total` | Counter | - | Total connections established |
   | `tqserver_active_requests` | Gauge | - | Number of currently active requests |

   **Backend/Worker Metrics:**
   | Metric Name | Type | Labels | Description |
   |-------------|------|--------|-------------|
   | `tqserver_worker_requests_total` | Counter | `worker`, `status` | Total requests per worker |
   | `tqserver_worker_http_responses_total` | Counter | `worker`, `status_group` | Total responses per worker by status group (`1xx`, `2xx`, `3xx`, `4xx`, `5xx`) |
   | `tqserver_worker_instances` | Gauge | `worker` | Current number of instances per worker |
   | `tqserver_worker_instances_healthy` | Gauge | `worker` | Number of healthy instances per worker |
   | `tqserver_worker_queue_depth` | Gauge | `worker` | Current queue depth per worker |
   | `tqserver_worker_memory_bytes` | Gauge | `worker`, `instance` | Memory usage per worker instance in bytes |
   | `tqserver_worker_restarts_total` | Counter | `worker` | Total worker restarts |
   | `tqserver_worker_build_errors_total` | Counter | `worker` | Total build errors |
   | `tqserver_worker_up` | Gauge | `worker` | Whether worker is healthy (0 or 1) |

   **Health Check Metrics:**
   | Metric Name | Type | Labels | Description |
   |-------------|------|--------|-------------|
   | `tqserver_health_check_duration_seconds` | Histogram | `worker` | Health check latency |
   | `tqserver_health_check_failures_total` | Counter | `worker` | Total health check failures |

3. **Singleton `MetricsRegistry`** struct that holds all collectors and provides update methods

---

### Configuration

#### [MODIFY] [config.go](file:///home/maurits/projects/tqserver/server/src/config.go)

Add optional metrics configuration to the `Config` struct:

```go
Metrics struct {
    Enabled bool   `yaml:"enabled"` // Default: true
    Path    string `yaml:"path"`    // Default: "/metrics"
} `yaml:"metrics"`
```

---

### Proxy Integration

#### [MODIFY] [proxy.go](file:///home/maurits/projects/tqserver/server/src/proxy.go)

1. **Add `/metrics` endpoint handler** in `Start()`:
   ```go
   if p.config.Metrics.Enabled {
       mux.Handle(p.config.Metrics.Path, promhttp.Handler())
   }
   ```

2. **Instrument request handling** in `handleRequest()`:
   - Record request start time
   - Increment request counters on completion
   - Track response status codes
   - Update active request gauge
   - Record bytes in/out (if feasible)

3. **Create middleware wrapper** for capturing metrics:
   ```go
   func (p *Proxy) instrumentedHandler(next http.HandlerFunc) http.HandlerFunc {
       return func(w http.ResponseWriter, r *http.Request) {
           start := time.Now()
           metrics.ActiveRequests.Inc()
           defer metrics.ActiveRequests.Dec()

           // Wrap ResponseWriter to capture status code
           wrapped := &statusCapturingWriter{ResponseWriter: w, statusCode: 200}
           
           next(wrapped, r)
           
           duration := time.Since(start).Seconds()
           metrics.RequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
           metrics.RequestsTotal.WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(wrapped.statusCode)).Inc()
       }
   }
   ```

---

### Router/Worker Integration

#### [MODIFY] [router.go](file:///home/maurits/projects/tqserver/server/src/router.go)

1. **Track per-worker request counts** properly (currently `IncrementRequestCount` returns 0):
   ```go
   func (w *Worker) IncrementRequestCount() int64 {
       return atomic.AddInt64(&w.RequestCount, 1)
   }
   ```

2. **Expose worker-specific metrics** via a new method:
   ```go
   func (w *Worker) GetMetricSnapshot() WorkerMetrics {
       w.mu.RLock()
       defer w.mu.RUnlock()
       return WorkerMetrics{
           Name:             w.Name,
           InstanceCount:    len(w.Instances),
           HealthyCount:     countHealthy(w.Instances),
           QueueDepth:       len(w.Queue),
           TotalRequests:    atomic.LoadInt64(&w.RequestCount),
           HasBuildError:    w.HasBuildError,
       }
   }
   ```

---

### Supervisor Integration

#### [MODIFY] [supervisor.go](file:///home/maurits/projects/tqserver/server/src/supervisor.go)

1. **Track restart counts** per worker (add field to Worker struct or track in supervisor)
2. **Update metrics on worker events**:
   - When a worker restarts: `metrics.WorkerRestarts.WithLabelValues(worker.Name).Inc()`
   - When scaling up/down: Update instance count gauge
   - On build errors: Increment build error counter

---

### Dependencies

#### [MODIFY] [go.mod](file:///home/maurits/projects/tqserver/go.mod)

Add the Prometheus client library:
```
github.com/prometheus/client_golang v1.22.0
```

---

### Documentation

#### [MODIFY] [docs/monitoring/metrics.md](file:///home/maurits/projects/tqserver/docs/monitoring/metrics.md)

Update the documentation to describe:
- Available metrics and their meanings
- How to configure the endpoint
- Example Prometheus scrape configuration
- Example Grafana dashboard queries

---

## Verification Plan

### Automated Tests

1. **Unit tests** for the metrics module:
   ```bash
   cd server/src && go test -v -run TestMetrics
   ```

2. **Integration test** - Start server and verify endpoint:
   ```bash
   ./start.sh &
   sleep 2
   curl -s http://localhost:8080/metrics | grep -E "^tqserver_"
   ```

3. **Verify metric format** matches Prometheus exposition format:
   ```bash
   curl -s http://localhost:8080/metrics | promtool check metrics
   ```

### Manual Verification

1. Start TQServer and verify the `/metrics` endpoint returns valid Prometheus metrics
2. Verify that making requests updates the counters appropriately
3. Verify worker-specific metrics are labeled correctly
4. Test with an actual Prometheus instance to confirm scraping works

---

## Implementation Order

1. Add `prometheus/client_golang` dependency to `go.mod`
2. Create `metrics.go` with all metric definitions
3. Update `config.go` with metrics configuration
4. Update `router.go` to properly track request counts
5. Update `proxy.go` to expose endpoint and instrument requests
6. Update `supervisor.go` to emit restart/scaling metrics
7. Update documentation
8. Run verification tests

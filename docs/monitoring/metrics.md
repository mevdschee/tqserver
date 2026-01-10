# Metrics

TQServer exposes a `/metrics` endpoint for Prometheus scraping, providing detailed metrics about request processing, worker status, and system health.

## Configuration

Metrics are enabled by default. Configure via `server.yaml`:

```yaml
metrics:
  enabled: true     # Enable/disable metrics endpoint
  path: "/metrics"  # Endpoint path
```

## Available Metrics

### Process Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `tqserver_process_start_time_seconds` | Gauge | - | Unix timestamp of process start |
| `tqserver_process_uptime_seconds` | Gauge | - | Seconds since process started |
| `tqserver_process_memory_bytes` | Gauge | `type` | Memory usage (`heap`, `stack`, `sys`) |
| `tqserver_process_goroutines` | Gauge | - | Number of active goroutines |

### Frontend/Proxy Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `tqserver_requests_total` | Counter | `method`, `path`, `status` | Total HTTP requests received |
| `tqserver_http_responses_total` | Counter | `status_group` | Responses by status group (`1xx`, `2xx`, `3xx`, `4xx`, `5xx`) |
| `tqserver_request_duration_seconds` | Histogram | `method`, `path` | Request latency distribution |
| `tqserver_bytes_in_total` | Counter | - | Total incoming bytes |
| `tqserver_bytes_out_total` | Counter | - | Total outgoing bytes |
| `tqserver_connections_total` | Counter | - | Total connections established |
| `tqserver_active_requests` | Gauge | - | Currently active requests |

### Worker Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `tqserver_worker_requests_total` | Counter | `worker`, `status` | Requests per worker |
| `tqserver_worker_http_responses_total` | Counter | `worker`, `status_group` | Responses per worker by status group |
| `tqserver_worker_instances` | Gauge | `worker` | Current instance count per worker |
| `tqserver_worker_instances_healthy` | Gauge | `worker` | Healthy instances per worker |
| `tqserver_worker_queue_depth` | Gauge | `worker` | Current queue depth |
| `tqserver_worker_memory_bytes` | Gauge | `worker`, `instance` | Memory per worker instance |
| `tqserver_worker_restarts_total` | Counter | `worker` | Total worker restarts |
| `tqserver_worker_build_errors_total` | Counter | `worker` | Total build errors |
| `tqserver_worker_up` | Gauge | `worker` | Worker health (0 or 1) |

### Health Check Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `tqserver_health_check_duration_seconds` | Histogram | `worker` | Health check latency |
| `tqserver_health_check_failures_total` | Counter | `worker` | Total health check failures |

## Prometheus Scrape Configuration

Add to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'tqserver'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

## Example Queries

### Request Rate
```promql
rate(tqserver_requests_total[5m])
```

### Request Duration (P99)
```promql
histogram_quantile(0.99, rate(tqserver_request_duration_seconds_bucket[5m]))
```

### Error Rate by Status Group
```promql
rate(tqserver_http_responses_total{status_group=~"4xx|5xx"}[5m])
```

### Worker Availability
```promql
tqserver_worker_up
```

### Memory Usage
```promql
tqserver_process_memory_bytes{type="heap"}
```

## Grafana Dashboard

Import the example dashboard for visualizing:
- Request throughput and latency
- Error rates by status code
- Worker health and scaling
- Memory and resource usage

See `docs/monitoring/grafana-dashboard.json` for a ready-to-import dashboard configuration.

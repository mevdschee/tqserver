package main

import (
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for TQServer
type Metrics struct {
	// Process metrics
	ProcessStartTime   prometheus.Gauge
	ProcessUptime      prometheus.Gauge
	ProcessMemoryBytes *prometheus.GaugeVec
	ProcessGoroutines  prometheus.Gauge

	// Frontend/Proxy metrics
	RequestsTotal      *prometheus.CounterVec
	HTTPResponsesTotal *prometheus.CounterVec
	RequestDuration    *prometheus.HistogramVec
	BytesInTotal       prometheus.Counter
	BytesOutTotal      prometheus.Counter
	ConnectionsTotal   prometheus.Counter
	ActiveRequests     prometheus.Gauge

	// Backend/Worker metrics
	WorkerRequestsTotal      *prometheus.CounterVec
	WorkerHTTPResponsesTotal *prometheus.CounterVec
	WorkerInstances          *prometheus.GaugeVec
	WorkerInstancesHealthy   *prometheus.GaugeVec
	WorkerQueueDepth         *prometheus.GaugeVec
	WorkerMemoryBytes        *prometheus.GaugeVec
	WorkerRestartsTotal      *prometheus.CounterVec
	WorkerBuildErrorsTotal   *prometheus.CounterVec
	WorkerUp                 *prometheus.GaugeVec

	// Health check metrics
	HealthCheckDuration      *prometheus.HistogramVec
	HealthCheckFailuresTotal *prometheus.CounterVec

	startTime time.Time
	mu        sync.RWMutex
}

// Global metrics instance
var metrics *Metrics
var metricsOnce sync.Once

// GetMetrics returns the singleton metrics instance
func GetMetrics() *Metrics {
	metricsOnce.Do(func() {
		metrics = newMetrics()
	})
	return metrics
}

// newMetrics creates and registers all Prometheus metrics
func newMetrics() *Metrics {
	m := &Metrics{
		startTime: time.Now(),

		// Process metrics
		ProcessStartTime: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "tqserver_process_start_time_seconds",
			Help: "Unix timestamp of process start",
		}),
		ProcessUptime: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "tqserver_process_uptime_seconds",
			Help: "Number of seconds since process started",
		}),
		ProcessMemoryBytes: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "tqserver_process_memory_bytes",
			Help: "Memory usage in bytes",
		}, []string{"type"}),
		ProcessGoroutines: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "tqserver_process_goroutines",
			Help: "Number of active goroutines",
		}),

		// Frontend/Proxy metrics
		RequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "tqserver_requests_total",
			Help: "Total HTTP requests received",
		}, []string{"method", "path", "status"}),
		HTTPResponsesTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "tqserver_http_responses_total",
			Help: "Total responses per status group",
		}, []string{"status_group"}),
		RequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "tqserver_request_duration_seconds",
			Help:    "Request latency in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),
		BytesInTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "tqserver_bytes_in_total",
			Help: "Total incoming bytes",
		}),
		BytesOutTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "tqserver_bytes_out_total",
			Help: "Total outgoing bytes",
		}),
		ConnectionsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "tqserver_connections_total",
			Help: "Total connections established",
		}),
		ActiveRequests: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "tqserver_active_requests",
			Help: "Number of currently active requests",
		}),

		// Backend/Worker metrics
		WorkerRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "tqserver_worker_requests_total",
			Help: "Total requests per worker",
		}, []string{"worker", "status"}),
		WorkerHTTPResponsesTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "tqserver_worker_http_responses_total",
			Help: "Total responses per worker by status group",
		}, []string{"worker", "status_group"}),
		WorkerInstances: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "tqserver_worker_instances",
			Help: "Current number of instances per worker",
		}, []string{"worker"}),
		WorkerInstancesHealthy: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "tqserver_worker_instances_healthy",
			Help: "Number of healthy instances per worker",
		}, []string{"worker"}),
		WorkerQueueDepth: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "tqserver_worker_queue_depth",
			Help: "Current queue depth per worker",
		}, []string{"worker"}),
		WorkerMemoryBytes: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "tqserver_worker_memory_bytes",
			Help: "Memory usage per worker instance in bytes",
		}, []string{"worker", "instance"}),
		WorkerRestartsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "tqserver_worker_restarts_total",
			Help: "Total worker restarts",
		}, []string{"worker"}),
		WorkerBuildErrorsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "tqserver_worker_build_errors_total",
			Help: "Total build errors",
		}, []string{"worker"}),
		WorkerUp: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "tqserver_worker_up",
			Help: "Whether worker is healthy (0 or 1)",
		}, []string{"worker"}),

		// Health check metrics
		HealthCheckDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "tqserver_health_check_duration_seconds",
			Help:    "Health check latency",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		}, []string{"worker"}),
		HealthCheckFailuresTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "tqserver_health_check_failures_total",
			Help: "Total health check failures",
		}, []string{"worker"}),
	}

	// Set process start time
	m.ProcessStartTime.Set(float64(m.startTime.Unix()))

	// Start background goroutine to update process metrics
	go m.updateProcessMetrics()

	return m
}

// updateProcessMetrics periodically updates process-level metrics
func (m *Metrics) updateProcessMetrics() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Update uptime
		m.ProcessUptime.Set(time.Since(m.startTime).Seconds())

		// Update memory stats
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		m.ProcessMemoryBytes.WithLabelValues("heap").Set(float64(memStats.HeapAlloc))
		m.ProcessMemoryBytes.WithLabelValues("stack").Set(float64(memStats.StackInuse))
		m.ProcessMemoryBytes.WithLabelValues("sys").Set(float64(memStats.Sys))

		// Update goroutine count
		m.ProcessGoroutines.Set(float64(runtime.NumGoroutine()))
	}
}

// GetStatusGroup returns the status group (1xx, 2xx, etc.) for a status code
func GetStatusGroup(statusCode int) string {
	switch {
	case statusCode >= 100 && statusCode < 200:
		return "1xx"
	case statusCode >= 200 && statusCode < 300:
		return "2xx"
	case statusCode >= 300 && statusCode < 400:
		return "3xx"
	case statusCode >= 400 && statusCode < 500:
		return "4xx"
	default:
		return "5xx"
	}
}

// RecordRequest records a completed request with all relevant metrics
func (m *Metrics) RecordRequest(method, path string, statusCode int, duration time.Duration, workerName string) {
	statusStr := string(rune('0'+statusCode/100)) + "xx"
	statusGroup := GetStatusGroup(statusCode)

	// Frontend metrics
	m.RequestsTotal.WithLabelValues(method, path, statusStr).Inc()
	m.HTTPResponsesTotal.WithLabelValues(statusGroup).Inc()
	m.RequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())

	// Worker metrics (if worker handled the request)
	if workerName != "" {
		m.WorkerRequestsTotal.WithLabelValues(workerName, statusStr).Inc()
		m.WorkerHTTPResponsesTotal.WithLabelValues(workerName, statusGroup).Inc()
	}
}

// UpdateWorkerMetrics updates worker-specific gauge metrics
func (m *Metrics) UpdateWorkerMetrics(workerName string, instanceCount, healthyCount, queueDepth int, isUp bool) {
	m.WorkerInstances.WithLabelValues(workerName).Set(float64(instanceCount))
	m.WorkerInstancesHealthy.WithLabelValues(workerName).Set(float64(healthyCount))
	m.WorkerQueueDepth.WithLabelValues(workerName).Set(float64(queueDepth))

	if isUp {
		m.WorkerUp.WithLabelValues(workerName).Set(1)
	} else {
		m.WorkerUp.WithLabelValues(workerName).Set(0)
	}
}

// RecordWorkerRestart increments the restart counter for a worker
func (m *Metrics) RecordWorkerRestart(workerName string) {
	m.WorkerRestartsTotal.WithLabelValues(workerName).Inc()
}

// RecordBuildError increments the build error counter for a worker
func (m *Metrics) RecordBuildError(workerName string) {
	m.WorkerBuildErrorsTotal.WithLabelValues(workerName).Inc()
}

// RecordHealthCheck records a health check result
func (m *Metrics) RecordHealthCheck(workerName string, duration time.Duration, success bool) {
	m.HealthCheckDuration.WithLabelValues(workerName).Observe(duration.Seconds())
	if !success {
		m.HealthCheckFailuresTotal.WithLabelValues(workerName).Inc()
	}
}

// SetWorkerInstanceMemory sets the memory usage for a specific worker instance
func (m *Metrics) SetWorkerInstanceMemory(workerName, instanceID string, memoryBytes uint64) {
	m.WorkerMemoryBytes.WithLabelValues(workerName, instanceID).Set(float64(memoryBytes))
}

// statusCapturingWriter wraps http.ResponseWriter to capture the status code
type statusCapturingWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func (w *statusCapturingWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusCapturingWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.written += int64(n)
	return n, err
}

// Unwrap returns the underlying ResponseWriter (for http.Flusher etc.)
func (w *statusCapturingWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

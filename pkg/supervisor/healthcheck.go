package supervisor

import (
"context"
"fmt"
"net/http"
"time"
)

// HealthChecker performs HTTP health checks on workers.
type HealthChecker struct {
registry *WorkerRegistry
interval time.Duration
timeout  time.Duration
client   *http.Client
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker(registry *WorkerRegistry, interval, timeout time.Duration) *HealthChecker {
return &HealthChecker{
registry: registry,
interval: interval,
timeout:  timeout,
client: &http.Client{
Timeout: timeout,
},
}
}

// Start begins periodic health checking.
func (h *HealthChecker) Start(ctx context.Context) {
go func() {
ticker := time.NewTicker(h.interval)
defer ticker.Stop()

for {
select {
case <-ctx.Done():
return
case <-ticker.C:
h.checkAll()
}
}
}()
}

// checkAll checks health of all registered workers.
func (h *HealthChecker) checkAll() {
workers := h.registry.List()
for _, worker := range workers {
healthy := h.checkWorker(worker)

// Update status
worker.Status = "unhealthy"
if healthy {
worker.Status = "healthy"
}
worker.LastHealthCheck = time.Now()

// Re-register to update the registry
h.registry.Register(worker)
}
}

// checkWorker performs an HTTP health check on a single worker.
func (h *HealthChecker) checkWorker(worker *WorkerInstance) bool {
if worker.Port == 0 {
return false
}

url := fmt.Sprintf("http://localhost:%d/health", worker.Port)
resp, err := h.client.Get(url)
if err != nil {
return false
}
defer resp.Body.Close()

return resp.StatusCode == http.StatusOK
}

// CheckWorkerOnce performs a single health check on a worker.
func (h *HealthChecker) CheckWorkerOnce(worker *WorkerInstance) bool {
return h.checkWorker(worker)
}

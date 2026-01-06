package router

import (
	"os"
	"sync"
	"time"
)

// Worker represents a running worker process
type Worker struct {
	Path         string // Path to the page directory (e.g., "pages/api/users")
	Route        string // URL route (e.g., "/api/users")
	Port         int    // Port the worker listens on
	Binary       string // Path to compiled binary
	Process      *os.Process
	StartTime    time.Time
	RequestCount int // Number of requests handled
	healthy      bool
	mu           sync.RWMutex
}

// IsHealthy checks if the worker is healthy
func (w *Worker) IsHealthy() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.healthy
}

// IncrementRequestCount increments the request counter and returns the new count
func (w *Worker) IncrementRequestCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.RequestCount++
	return w.RequestCount
}

// GetRequestCount returns the current request count
func (w *Worker) GetRequestCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.RequestCount
}

// SetHealthy sets the worker health status
func (w *Worker) SetHealthy(healthy bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.healthy = healthy
}

// ResetRequestCount resets the request counter
func (w *Worker) ResetRequestCount() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.RequestCount = 0
}

package php

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// WorkerState represents the current state of a PHP worker
type WorkerState int

const (
	WorkerStateIdle WorkerState = iota
	WorkerStateActive
	WorkerStateTerminating
	WorkerStateCrashed
)

func (s WorkerState) String() string {
	switch s {
	case WorkerStateIdle:
		return "idle"
	case WorkerStateActive:
		return "active"
	case WorkerStateTerminating:
		return "terminating"
	case WorkerStateCrashed:
		return "crashed"
	default:
		return "unknown"
	}
}

// Worker represents a single php-cgi process
type Worker struct {
	ID int

	state        atomic.Value // WorkerState
	requestCount atomic.Int64
	startTime    time.Time
	lastUsed     time.Time
	mu           sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc

	// Channels for lifecycle management
	done   chan struct{}
	errors chan error
}

// NewWorker creates a new PHP worker
func NewWorker(id int) *Worker {
	ctx, cancel := context.WithCancel(context.Background())

	w := &Worker{
		ID:        id,
		startTime: time.Now(),
		lastUsed:  time.Now(),
		ctx:       ctx,
		cancel:    cancel,
		done:      make(chan struct{}),
		errors:    make(chan error, 1),
	}

	w.setState(WorkerStateIdle)
	return w
}

// Start spawns the php-cgi process
func (w *Worker) Start() error {
	// In php-fpm mode the worker is a logical slot backed by a central php-fpm instance.
	// Start is a no-op for the adapter-backed worker but we record the start time/state.
	w.mu.Lock()
	defer w.mu.Unlock()

	w.startTime = time.Now()
	w.lastUsed = time.Now()
	w.setState(WorkerStateIdle)
	return nil
}

// Stop gracefully stops the worker
func (w *Worker) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Mark terminating and signal done. No OS process to manage in php-fpm mode.
	w.setState(WorkerStateTerminating)
	select {
	case <-w.done:
		// already closed
	default:
		close(w.done)
	}
	// Cancel context as a courtesy
	w.cancel()
	return nil
}

// monitor watches the process and handles crashes
// Note: process monitoring and output handling are intentionally omitted for php-fpm
// adapter-backed workers since php-fpm is managed centrally by the supervisor.
// MarkActive marks the worker as actively processing a request
func (w *Worker) MarkActive() {
	w.setState(WorkerStateActive)
	w.requestCount.Add(1)
	w.mu.Lock()
	w.lastUsed = time.Now()
	w.mu.Unlock()
}

// MarkIdle marks the worker as idle
func (w *Worker) MarkIdle() {
	w.setState(WorkerStateIdle)
	w.mu.Lock()
	w.lastUsed = time.Now()
	w.mu.Unlock()
}

// GetState returns the current worker state
func (w *Worker) GetState() WorkerState {
	return w.getState()
}

func (w *Worker) getState() WorkerState {
	if state := w.state.Load(); state != nil {
		return state.(WorkerState)
	}
	return WorkerStateIdle
}

func (w *Worker) setState(state WorkerState) {
	w.state.Store(state)
}

// GetRequestCount returns the total number of requests handled
func (w *Worker) GetRequestCount() int64 {
	return w.requestCount.Load()
}

// GetUptime returns how long the worker has been running
func (w *Worker) GetUptime() time.Duration {
	return time.Since(w.startTime)
}

// GetIdleTime returns how long the worker has been idle
func (w *Worker) GetIdleTime() time.Duration {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.getState() == WorkerStateIdle {
		return time.Since(w.lastUsed)
	}
	return 0
}

// ShouldRestart checks if the worker should be restarted
func (w *Worker) ShouldRestart() bool {
	// Only consider a restart when the worker has crashed; max-requests is managed by the manager.
	return w.getState() == WorkerStateCrashed
}

// Wait waits for the worker to exit
func (w *Worker) Wait() error {
	<-w.done
	return nil
}

// Errors returns a channel for worker errors
func (w *Worker) Errors() <-chan error {
	return w.errors
}

// IsHealthy checks if the worker is in a healthy state
func (w *Worker) IsHealthy() bool {
	state := w.getState()
	return state == WorkerStateIdle || state == WorkerStateActive
}

// GetPID returns the process ID, or 0 if not running
func (w *Worker) GetPID() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	// Adapter-backed workers have no OS process; return 0 for php-fpm mode
	return 0
}

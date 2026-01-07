package php

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Manager manages a pool of PHP workers
type Manager struct {
	binary  *Binary
	config  *Config
	workers []*Worker
	mu      sync.RWMutex

	nextID     int
	baseSocket string // Base socket path (will add worker ID)

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Monitoring
	totalRequests int64
	totalRestarts int64
}

// NewManager creates a new PHP worker manager
func NewManager(binary *Binary, config *Config) (*Manager, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Determine base socket path
	baseSocket := config.Pool.ListenAddr
	if config.Pool.UnixSocket != "" {
		baseSocket = config.Pool.UnixSocket
	}

	m := &Manager{
		binary:     binary,
		config:     config,
		workers:    make([]*Worker, 0),
		baseSocket: baseSocket,
		ctx:        ctx,
		cancel:     cancel,
	}

	return m, nil
}

// Start initializes and starts the worker pool
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get initial worker count based on pool manager type
	initialCount := m.config.Pool.GetWorkerCount()

	log.Printf("Starting PHP worker manager with %d initial workers (%s mode)",
		initialCount, m.config.Pool.Manager)

	// Start initial workers
	for i := 0; i < initialCount; i++ {
		if err := m.spawnWorker(); err != nil {
			// Clean up already started workers
			for _, w := range m.workers {
				w.Stop()
			}
			return fmt.Errorf("failed to start worker %d: %w", i, err)
		}
	}

	// Start monitoring goroutine
	m.wg.Add(1)
	go m.monitor()

	log.Printf("PHP worker manager started successfully with %d workers", len(m.workers))
	return nil
}

// Stop gracefully stops all workers
func (m *Manager) Stop() error {
	log.Printf("Stopping PHP worker manager...")

	// Cancel context to signal shutdown
	m.cancel()

	// Wait for monitoring goroutine
	m.wg.Wait()

	// Stop all workers
	m.mu.Lock()
	workers := make([]*Worker, len(m.workers))
	copy(workers, m.workers)
	m.mu.Unlock()

	var wg sync.WaitGroup
	for _, w := range workers {
		wg.Add(1)
		go func(worker *Worker) {
			defer wg.Done()
			if err := worker.Stop(); err != nil {
				log.Printf("Error stopping worker %d: %v", worker.ID, err)
			}
		}(w)
	}

	wg.Wait()

	m.mu.Lock()
	m.workers = nil
	m.mu.Unlock()

	log.Printf("PHP worker manager stopped")
	return nil
}

// spawnWorker creates and starts a new worker (must be called with lock held)
func (m *Manager) spawnWorker() error {
	workerID := m.nextID
	m.nextID++

	// Generate unique socket path for this worker
	socketPath := fmt.Sprintf("%s.%d", m.baseSocket, workerID)

	worker := NewWorker(workerID, m.binary, m.config, socketPath)

	if err := worker.Start(); err != nil {
		return err
	}

	m.workers = append(m.workers, worker)

	// Monitor worker for crashes
	m.wg.Add(1)
	go m.monitorWorker(worker)

	return nil
}

// monitorWorker monitors a single worker for crashes and handles restarts
func (m *Manager) monitorWorker(worker *Worker) {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			return
		case err := <-worker.Errors():
			log.Printf("[Worker %d] Error: %v", worker.ID, err)

			// Remove crashed worker from pool
			m.mu.Lock()
			for i, w := range m.workers {
				if w.ID == worker.ID {
					m.workers = append(m.workers[:i], m.workers[i+1:]...)
					break
				}
			}

			// Spawn replacement worker if needed
			if m.shouldSpawnReplacement() {
				log.Printf("[Worker %d] Spawning replacement worker", worker.ID)
				if err := m.spawnWorker(); err != nil {
					log.Printf("Failed to spawn replacement worker: %v", err)
				} else {
					m.totalRestarts++
				}
			}
			m.mu.Unlock()

			return
		case <-worker.done:
			return
		}
	}
}

// shouldSpawnReplacement determines if a replacement worker should be spawned
func (m *Manager) shouldSpawnReplacement() bool {
	currentCount := len(m.workers)

	switch m.config.Pool.Manager {
	case "static":
		// Always maintain the configured number of workers
		return currentCount < m.config.Pool.MaxWorkers

	case "dynamic":
		// Ensure we have at least min_workers
		return currentCount < m.config.Pool.MinWorkers

	case "ondemand":
		// Don't automatically spawn replacements in ondemand mode
		return false

	default:
		return false
	}
}

// monitor performs periodic health checks and pool management
func (m *Manager) monitor() {
	defer m.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.performHealthCheck()
			m.managePoolSize()
		}
	}
}

// performHealthCheck checks worker health and restarts unhealthy workers
func (m *Manager) performHealthCheck() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := len(m.workers) - 1; i >= 0; i-- {
		worker := m.workers[i]

		// Check if worker should be restarted
		if worker.ShouldRestart() {
			log.Printf("[Worker %d] Restarting (requests: %d, state: %s)",
				worker.ID, worker.GetRequestCount(), worker.GetState())

			// Remove from pool
			m.workers = append(m.workers[:i], m.workers[i+1:]...)

			// Stop old worker
			go worker.Stop()

			// Spawn replacement
			if err := m.spawnWorker(); err != nil {
				log.Printf("Failed to spawn replacement worker: %v", err)
			} else {
				m.totalRestarts++
			}
		}
	}
}

// managePoolSize adjusts the pool size based on load (for dynamic/ondemand modes)
func (m *Manager) managePoolSize() {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentCount := len(m.workers)

	switch m.config.Pool.Manager {
	case "dynamic":
		// Count idle workers
		idleCount := 0
		for _, w := range m.workers {
			if w.GetState() == WorkerStateIdle {
				idleCount++
			}
		}

		// Scale up if we have too few idle workers
		if idleCount < 2 && currentCount < m.config.Pool.MaxWorkers {
			log.Printf("Scaling up: idle=%d, total=%d", idleCount, currentCount)
			if err := m.spawnWorker(); err != nil {
				log.Printf("Failed to spawn worker: %v", err)
			}
		}

		// Scale down if we have too many idle workers (but keep min_workers)
		if idleCount > 4 && currentCount > m.config.Pool.MinWorkers {
			// Find an idle worker to remove
			for i, w := range m.workers {
				if w.GetState() == WorkerStateIdle && w.GetIdleTime() > m.config.Pool.IdleTimeout {
					log.Printf("Scaling down: removing idle worker %d", w.ID)
					m.workers = append(m.workers[:i], m.workers[i+1:]...)
					go w.Stop()
					break
				}
			}
		}

	case "ondemand":
		// Kill idle workers after timeout
		for i := len(m.workers) - 1; i >= 0; i-- {
			w := m.workers[i]
			if w.GetState() == WorkerStateIdle && w.GetIdleTime() > m.config.Pool.IdleTimeout {
				log.Printf("[Worker %d] Killing idle worker (idle: %v)", w.ID, w.GetIdleTime())
				m.workers = append(m.workers[:i], m.workers[i+1:]...)
				go w.Stop()
			}
		}
	}
}

// GetIdleWorker returns an idle worker, or spawns one if needed (ondemand mode)
func (m *Manager) GetIdleWorker() (*Worker, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find idle worker
	for _, w := range m.workers {
		if w.GetState() == WorkerStateIdle {
			w.MarkActive()
			return w, nil
		}
	}

	// No idle workers available
	if m.config.Pool.Manager == "ondemand" && len(m.workers) < m.config.Pool.MaxWorkers {
		// Spawn a new worker
		if err := m.spawnWorker(); err != nil {
			return nil, fmt.Errorf("failed to spawn ondemand worker: %w", err)
		}
		// Return the newly spawned worker
		worker := m.workers[len(m.workers)-1]
		worker.MarkActive()
		return worker, nil
	}

	return nil, fmt.Errorf("no idle workers available")
}

// ReleaseWorker marks a worker as idle
func (m *Manager) ReleaseWorker(worker *Worker) {
	worker.MarkIdle()
}

// GetStats returns current manager statistics
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_workers":  len(m.workers),
		"total_requests": m.totalRequests,
		"total_restarts": m.totalRestarts,
		"pool_manager":   m.config.Pool.Manager,
	}

	// Count workers by state
	idleCount := 0
	activeCount := 0
	for _, w := range m.workers {
		switch w.GetState() {
		case WorkerStateIdle:
			idleCount++
		case WorkerStateActive:
			activeCount++
		}
	}

	stats["idle_workers"] = idleCount
	stats["active_workers"] = activeCount

	return stats
}

// GetWorkerInfo returns detailed information about all workers
func (m *Manager) GetWorkerInfo() []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info := make([]map[string]interface{}, len(m.workers))
	for i, w := range m.workers {
		info[i] = map[string]interface{}{
			"id":            w.ID,
			"pid":           w.GetPID(),
			"state":         w.GetState().String(),
			"request_count": w.GetRequestCount(),
			"uptime":        w.GetUptime().String(),
			"idle_time":     w.GetIdleTime().String(),
		}
	}

	return info
}

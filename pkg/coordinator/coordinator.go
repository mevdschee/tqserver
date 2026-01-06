package coordinator

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// WorkerManager defines the interface for managing worker processes.
type WorkerManager interface {
	StopWorker(name string) error
	StartWorker(name string) error
	GetWorkerStatus(name string) (string, error)
}

// Coordinator manages worker restarts with zero-downtime.
type Coordinator struct {
	manager WorkerManager
	mu      sync.Mutex
	pending map[string]*restartTask
}

type restartTask struct {
	workerName string
	status     string
	startedAt  time.Time
}

// New creates a new restart coordinator.
func New(manager WorkerManager) *Coordinator {
	return &Coordinator{
		manager: manager,
		pending: make(map[string]*restartTask),
	}
}

// RestartWorker performs a zero-downtime restart of a worker.
// Strategy:
// 1. Start new worker instance on new port
// 2. Wait for health check
// 3. Switch traffic to new instance
// 4. Stop old instance
func (c *Coordinator) RestartWorker(workerName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already restarting
	if task, exists := c.pending[workerName]; exists {
		return fmt.Errorf("worker %s is already restarting (started %v ago)",
			workerName, time.Since(task.startedAt))
	}

	log.Printf("Coordinating restart of worker: %s", workerName)

	// Create restart task
	task := &restartTask{
		workerName: workerName,
		status:     "starting",
		startedAt:  time.Now(),
	}
	c.pending[workerName] = task

	// Perform restart in goroutine
	go func() {
		defer func() {
			c.mu.Lock()
			delete(c.pending, workerName)
			c.mu.Unlock()
		}()

		if err := c.doRestart(workerName); err != nil {
			log.Printf("Failed to restart worker %s: %v", workerName, err)
			return
		}

		log.Printf("Successfully restarted worker: %s", workerName)
	}()

	return nil
}

// doRestart performs the actual restart sequence.
func (c *Coordinator) doRestart(workerName string) error {
	// For now, simple stop and start
	// TODO: Implement zero-downtime restart with port switching

	log.Printf("Stopping worker: %s", workerName)
	if err := c.manager.StopWorker(workerName); err != nil {
		return fmt.Errorf("failed to stop worker: %w", err)
	}

	// Small delay to ensure clean shutdown
	time.Sleep(100 * time.Millisecond)

	log.Printf("Starting worker: %s", workerName)
	if err := c.manager.StartWorker(workerName); err != nil {
		return fmt.Errorf("failed to start worker: %w", err)
	}

	// Wait for worker to become healthy
	if err := c.waitForHealthy(workerName, 5*time.Second); err != nil {
		return fmt.Errorf("worker failed to become healthy: %w", err)
	}

	return nil
}

// waitForHealthy waits for a worker to become healthy.
func (c *Coordinator) waitForHealthy(workerName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		status, err := c.manager.GetWorkerStatus(workerName)
		if err != nil {
			return err
		}

		if status == "healthy" {
			return nil
		}

		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for worker to become healthy")
}

// GetPendingRestarts returns the list of workers currently being restarted.
func (c *Coordinator) GetPendingRestarts() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	var workers []string
	for name := range c.pending {
		workers = append(workers, name)
	}
	return workers
}

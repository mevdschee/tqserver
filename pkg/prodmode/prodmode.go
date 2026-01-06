package prodmode

import (
	"log"
	"sync"

	"github.com/mevdschee/tqserver/pkg/supervisor"
)

// RestartHandler is called when a worker needs to be restarted.
type RestartHandler func(workerName string, changeType string)

// ProdMode manages production mode with SIGHUP-triggered checking.
type ProdMode struct {
	registry       *supervisor.WorkerRegistry
	watcher        *supervisor.SignalWatcher
	restartHandler RestartHandler
	mu             sync.Mutex
	running        bool
}

// Config holds production mode configuration.
type Config struct {
	ServerBinPath  string
	RestartHandler RestartHandler
}

// New creates a new production mode manager.
func New(cfg Config) *ProdMode {
	registry := supervisor.NewWorkerRegistry()

	pm := &ProdMode{
		registry:       registry,
		restartHandler: cfg.RestartHandler,
	}

	// Create signal watcher with our change handler
	pm.watcher = supervisor.NewSignalWatcher(
		registry,
		cfg.ServerBinPath,
		pm.handleChange,
	)

	return pm
}

// Start begins listening for SIGHUP signals.
func (pm *ProdMode) Start() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.running {
		return nil
	}

	log.Println("Starting production mode...")
	pm.watcher.Start()
	pm.running = true
	log.Println("Production mode started - listening for SIGHUP signals")
	return nil
}

// Stop stops the signal watcher.
func (pm *ProdMode) Stop() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.running {
		return
	}

	log.Println("Stopping production mode...")
	pm.watcher.Stop()
	pm.running = false
}

// RegisterWorker registers a worker in the tracking registry.
func (pm *ProdMode) RegisterWorker(worker *supervisor.WorkerInstance) {
	pm.registry.Register(worker)
}

// UnregisterWorker removes a worker from the tracking registry.
func (pm *ProdMode) UnregisterWorker(workerName string) {
	pm.registry.Remove(workerName)
}

// CheckNow manually triggers a change check (for testing).
func (pm *ProdMode) CheckNow() {
	pm.watcher.CheckNow()
}

// handleChange processes change detection events.
func (pm *ProdMode) handleChange(workerName string, changeType string) {
	log.Printf("Production mode: change detected for %s (type: %s)", workerName, changeType)

	if pm.restartHandler != nil {
		pm.restartHandler(workerName, changeType)
	}
}

// GetRegistry returns the worker registry.
func (pm *ProdMode) GetRegistry() *supervisor.WorkerRegistry {
	return pm.registry
}

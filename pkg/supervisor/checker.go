package supervisor

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// ChangeHandler is a function that handles file changes.
// It receives the worker name and the type of change ("binary").
type ChangeHandler func(workerName string, changeType string)

// SignalWatcher watches for SIGHUP signals and checks for file changes.
type SignalWatcher struct {
	registry       *WorkerRegistry
	changeHandler  ChangeHandler
	serverBinPath  string
	serverBinMtime time.Time
	stopChan       chan struct{}
}

// NewSignalWatcher creates a new signal watcher.
func NewSignalWatcher(registry *WorkerRegistry, serverBinPath string, handler ChangeHandler) *SignalWatcher {
	return &SignalWatcher{
		registry:       registry,
		changeHandler:  handler,
		serverBinPath:  serverBinPath,
		serverBinMtime: GetFileMtime(serverBinPath),
		stopChan:       make(chan struct{}),
	}
}

// Start begins watching for SIGHUP signals.
func (w *SignalWatcher) Start() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP)

	go func() {
		for {
			select {
			case <-sigChan:
				log.Println("Received SIGHUP, checking for file changes...")
				w.checkChanges()
			case <-w.stopChan:
				signal.Stop(sigChan)
				return
			}
		}
	}()
}

// Stop stops the signal watcher.
func (w *SignalWatcher) Stop() {
	close(w.stopChan)
}

// checkChanges checks all workers and the server binary for changes.
func (w *SignalWatcher) checkChanges() {
	// Check server binary
	if HasFileChanged(w.serverBinPath, w.serverBinMtime) {
		log.Println("Server binary changed, restart required")
		// Update recorded mtime
		w.serverBinMtime = GetFileMtime(w.serverBinPath)
		// In a real implementation, this would trigger a graceful server restart
		// For now, just log it
	}

	// Check each worker
	for _, worker := range w.registry.List() {
		changed, changeType := w.registry.CheckChanges(worker.Name)
		if changed {
			log.Printf("Worker %s changed (type: %s), triggering handler", worker.Name, changeType)

			// Call the change handler
			if w.changeHandler != nil {
				w.changeHandler(worker.Name, changeType)
			}

			// Update recorded mtimes after handling
			w.registry.UpdateMtimes(worker.Name)
		}
	}
}

// CheckNow manually triggers a change check (for testing or manual reload).
func (w *SignalWatcher) CheckNow() {
	w.checkChanges()
}

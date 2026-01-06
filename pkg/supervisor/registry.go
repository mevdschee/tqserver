package supervisor

import (
	"sync"
	"time"
)

// WorkerInstance represents a running worker process with file tracking.
type WorkerInstance struct {
	Name      string
	Route     string
	PID       int
	Port      int
	StartedAt time.Time

	// File tracking
	BinaryPath   string
	BinaryMtime  time.Time
	PublicPath   string
	PublicMtime  time.Time
	PrivatePath  string
	PrivateMtime time.Time

	// Health
	Status          string // "starting", "healthy", "stopping"
	LastHealthCheck time.Time
}

// WorkerRegistry maintains a registry of running workers with their file timestamps.
type WorkerRegistry struct {
	mu      sync.RWMutex
	workers map[string]*WorkerInstance
}

// NewWorkerRegistry creates a new worker registry.
func NewWorkerRegistry() *WorkerRegistry {
	return &WorkerRegistry{
		workers: make(map[string]*WorkerInstance),
	}
}

// Register adds or updates a worker in the registry.
func (r *WorkerRegistry) Register(worker *WorkerInstance) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workers[worker.Name] = worker
}

// Get retrieves a worker by name.
func (r *WorkerRegistry) Get(name string) (*WorkerInstance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	worker, ok := r.workers[name]
	return worker, ok
}

// Remove removes a worker from the registry.
func (r *WorkerRegistry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.workers, name)
}

// List returns all registered workers.
func (r *WorkerRegistry) List() []*WorkerInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	workers := make([]*WorkerInstance, 0, len(r.workers))
	for _, worker := range r.workers {
		workers = append(workers, worker)
	}
	return workers
}

// UpdateMtimes updates the recorded mtimes for a worker.
func (r *WorkerRegistry) UpdateMtimes(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	worker, ok := r.workers[name]
	if !ok {
		return
	}

	// Update mtimes from filesystem
	worker.BinaryMtime = GetFileMtime(worker.BinaryPath)
	worker.PublicMtime = GetDirLatestMtime(worker.PublicPath)
	worker.PrivateMtime = GetDirLatestMtime(worker.PrivatePath)
}

// CheckChanges checks if any files have changed for a worker.
// Returns true and the type of change ("binary", "assets", or "both").
func (r *WorkerRegistry) CheckChanges(name string) (bool, string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	worker, ok := r.workers[name]
	if !ok {
		return false, ""
	}

	binaryChanged := HasFileChanged(worker.BinaryPath, worker.BinaryMtime)
	publicChanged := HasDirChanged(worker.PublicPath, worker.PublicMtime)
	privateChanged := HasDirChanged(worker.PrivatePath, worker.PrivateMtime)

	assetsChanged := publicChanged || privateChanged

	if binaryChanged && assetsChanged {
		return true, "both"
	} else if binaryChanged {
		return true, "binary"
	} else if assetsChanged {
		return true, "assets"
	}

	return false, ""
}

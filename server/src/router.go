package main

import (
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// WorkerInstance represents a single process instance of a worker service
type WorkerInstance struct {
	ID          string
	Port        int
	Process     *os.Process
	StartTime   time.Time
	LastRequest time.Time
	Healthy     bool
}

// WorkerRequest represents a request for a worker instance
type WorkerRequest struct {
	ResponseChan chan *WorkerInstance
}

// Worker represents a worker service (load balancer)
type Worker struct {
	Name  string // Worker name
	Route string // URL route
	Type  string // Worker type: "go", "bun", "php"

	// Cluster state
	Instances    []*WorkerInstance
	NextInstance int                 // Round robin index
	Queue        chan *WorkerRequest // Request queue

	// Configuration (snapshot)
	MinWorkers     int
	MaxWorkers     int
	QueueThreshold int
	ScaleDownDelay int

	// Health & Status
	HasBuildError bool
	BuildError    string
	RequestCount  int64

	mu sync.RWMutex
}

// IsHealthy checks if the worker service has at least one healthy instance
func (w *Worker) IsHealthy() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// If it has build error, it's not healthy
	if w.HasBuildError {
		return false
	}

	// For PHP (FastCGI), we track it differently (external pool usually), but here we might treat it same?
	// Existing code had IsPHP. The new requirement assumes Bun/Go mainly.
	// Let's assume for now at least one instance must be running.
	if len(w.Instances) == 0 {
		return false
	}
	// Check if any instance is healthy
	for _, inst := range w.Instances {
		if inst.Healthy {
			return true
		}
	}
	return false
}

// IncrementRequestCount increments the global request counter
func (w *Worker) IncrementRequestCount() int64 {
	return 0 // Implemented via atomic or just unused? The LB tracks queue.
	// We can keep a global counter for stats.
}

// GetStats returns current worker stats
func (w *Worker) GetStats() (int, int, int64) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.Instances), len(w.Queue), w.RequestCount
}

// SetBuildError sets the build error status and message
func (w *Worker) SetBuildError(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err != nil {
		w.HasBuildError = true
		w.BuildError = err.Error()
	} else {
		w.HasBuildError = false
		w.BuildError = ""
	}
}

// GetBuildError returns the build error message if any
func (w *Worker) GetBuildError() (bool, string) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.HasBuildError, w.BuildError
}

// Router manages routing from URL paths to workers
type Router struct {
	workersDir    string
	projectRoot   string
	workerConfigs []*WorkerConfigWithMeta
	workers       map[string]*Worker // route -> worker
	mu            sync.RWMutex
}

// NewRouter creates a new router
func NewRouter(workersDir, projectRoot string, workerConfigs []*WorkerConfigWithMeta) *Router {
	return &Router{
		workersDir:    workersDir,
		projectRoot:   projectRoot,
		workerConfigs: workerConfigs,
		workers:       make(map[string]*Worker),
	}
}

// DiscoverRoutes loads route configuration from worker configs
func (r *Router) DiscoverRoutes() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Printf("Loading routes from worker configs...")

	// Workers will be registered when supervisor builds and starts them
	for _, workerMeta := range r.workerConfigs {
		log.Printf("Route configured: %s -> %s", workerMeta.Config.Path, workerMeta.Name)
	}

	return nil
}

// RegisterWorker registers a worker for a specific route
func (r *Router) RegisterWorker(worker *Worker) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.workers[worker.Route] = worker
	log.Printf("Registered worker: %s -> %s", worker.Route, worker.Name)
}

// GetWorker returns the worker for a given route
func (r *Router) GetWorker(path string) *Worker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try exact match first
	if worker, ok := r.workers[path]; ok {
		return worker
	}

	// Try to find longest prefix match
	longestMatch := ""
	var matchedWorker *Worker

	for route, worker := range r.workers {
		if strings.HasPrefix(path, route) && len(route) > len(longestMatch) {
			longestMatch = route
			matchedWorker = worker
		}
	}

	// Check for fallback "/" or "/index"
	if matchedWorker == nil {
		if worker, ok := r.workers["/"]; ok {
			return worker
		}
	}

	return matchedWorker
}

// GetAllWorkers returns all workers
func (r *Router) GetAllWorkers() []*Worker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	workers := make([]*Worker, 0, len(r.workers))
	for _, worker := range r.workers {
		workers = append(workers, worker)
	}
	return workers
}

// UpdateWorker updates a worker entry
func (r *Router) UpdateWorker(route string, worker *Worker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workers[route] = worker
}

// hasGoSourceFiles checks if a directory contains .go files
func hasGoSourceFiles(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			return true, nil
		}
	}

	return false, nil
}

package main

import (
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// Worker represents a running worker process
type Worker struct {
	Name          string // Worker name (directory name, e.g., "api", "index")
	Route         string // URL route (e.g., "/api/users")
	Port          int    // Port the worker listens on (0 for PHP workers using FastCGI)
	Binary        string // Path to compiled binary
	Process       *os.Process
	StartTime     time.Time
	RequestCount  int    // Number of requests handled
	IsPHP         bool   // True if this is a PHP worker (uses FastCGI)
	FastCGIAddr   string // FastCGI address for PHP workers (e.g., "127.0.0.1:9001")
	healthy       bool
	HasBuildError bool   // True if the last build failed
	BuildError    string // The compilation error message
	mu            sync.RWMutex
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

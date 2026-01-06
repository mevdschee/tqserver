package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"sync"
)

// Proxy handles incoming HTTP requests and routes them to backend workers
type Proxy struct {
	config      *Config
	router      *Router
	server      *http.Server
	projectRoot string
	mu          sync.RWMutex
}

// NewProxy creates a new reverse proxy
func NewProxy(config *Config, router *Router, projectRoot string) *Proxy {
	return &Proxy{
		config:      config,
		router:      router,
		projectRoot: projectRoot,
	}
}

// Start starts the HTTP server
func (p *Proxy) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handleRequest)

	p.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", p.config.Server.Port),
		Handler:      mux,
		ReadTimeout:  p.config.GetReadTimeout(),
		WriteTimeout: p.config.GetWriteTimeout(),
		IdleTimeout:  p.config.GetIdleTimeout(),
	}

	log.Printf("Proxy listening on http://localhost:%d", p.config.Server.Port)
	return p.server.ListenAndServe()
}

// Stop gracefully stops the proxy
func (p *Proxy) Stop() error {
	if p.server != nil {
		return p.server.Close()
	}
	return nil
}

// handleRequest routes incoming requests to appropriate workers
func (p *Proxy) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Get worker for this route
	worker := p.router.GetWorker(r.URL.Path)

	if worker == nil {
		http.Error(w, "404 Not Found", http.StatusNotFound)
		log.Printf("No worker found for path: %s", r.URL.Path)
		return
	}

	// Priority 1: Try to serve from worker's public directory
	workerPublicPath := filepath.Join(p.projectRoot, p.config.Workers.Directory, worker.Name, "public", r.URL.Path)
	if p.serveFile(w, r, workerPublicPath) {
		log.Printf("%s %s -> static file (worker: %s)", r.Method, r.URL.Path, worker.Name)
		return
	}

	// Priority 2: Try to serve from server's public directory
	serverPublicPath := filepath.Join(p.projectRoot, "server", "public", r.URL.Path)
	if p.serveFile(w, r, serverPublicPath) {
		log.Printf("%s %s -> static file (server)", r.Method, r.URL.Path)
		return
	}

	// Priority 3: Let the worker handle the request (proxy to worker)
	// Check if worker is healthy
	if !worker.IsHealthy() {
		http.Error(w, "503 Service Unavailable", http.StatusServiceUnavailable)
		log.Printf("Worker unhealthy for path: %s", r.URL.Path)
		return
	}

	// Proxy request to worker
	target, err := url.Parse(fmt.Sprintf("http://localhost:%d", worker.Port))
	if err != nil {
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		log.Printf("Failed to parse worker URL: %v", err)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Proxy error for %s: %v", r.URL.Path, err)
		http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
	}

	log.Printf("%s %s -> worker on port %d", r.Method, r.URL.Path, worker.Port)
	proxy.ServeHTTP(w, r)

	// Increment request count for this worker (used for monitoring)
	worker.IncrementRequestCount()
}

// serveFile attempts to serve a file from the given path
// Returns true if the file was served successfully, false otherwise
func (p *Proxy) serveFile(w http.ResponseWriter, r *http.Request, filePath string) bool {
	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		return false
	}

	// Serve the file
	http.ServeFile(w, r, filePath)
	return true
}

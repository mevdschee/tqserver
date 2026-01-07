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
	"time"

	"github.com/mevdschee/tqtemplate"
)

// Proxy handles incoming HTTP requests and routes them to backend workers
type Proxy struct {
	config            *Config
	router            *Router
	server            *http.Server
	projectRoot       string
	tmpl              *tqtemplate.Template
	reloadBroadcaster *ReloadBroadcaster
	mu                sync.RWMutex
}

// NewProxy creates a new reverse proxy
func NewProxy(config *Config, router *Router, projectRoot string) *Proxy {
	// Initialize template loader
	loader := func(name string) (string, error) {
		content, err := os.ReadFile(name)
		return string(content), err
	}
	tmpl := tqtemplate.NewTemplateWithLoader(loader)

	return &Proxy{
		config:            config,
		router:            router,
		projectRoot:       projectRoot,
		tmpl:              tmpl,
		reloadBroadcaster: NewReloadBroadcaster(),
	}
}

// Start starts the HTTP server
func (p *Proxy) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handleRequest)

	// Add WebSocket endpoint for live reload (dev mode only)
	if p.config.IsDevelopmentMode() {
		mux.HandleFunc("/ws/reload", p.reloadBroadcaster.HandleWebSocket)
		log.Printf("Live reload WebSocket enabled at ws://localhost:%d/ws/reload", p.config.Server.Port)
	}

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

// BroadcastReload sends reload message to all connected WebSocket clients
func (p *Proxy) BroadcastReload() {
	if p.reloadBroadcaster != nil {
		p.reloadBroadcaster.BroadcastReload()
	}
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
	// In dev mode, check if there's a build error and serve error page
	if p.config.IsDevelopmentMode() {
		if hasBuildError, buildError := worker.GetBuildError(); hasBuildError {
			p.serveBuildErrorPage(w, r, worker.Name, buildError)
			return
		}
	}

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

// serveBuildErrorPage serves an HTML error page showing compilation errors
func (p *Proxy) serveBuildErrorPage(w http.ResponseWriter, r *http.Request, workerName string, buildError string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK) // Use 200 so browser doesn't show its own error page

	data := map[string]interface{}{
		"WorkerName": workerName,
		"BuildError": buildError,
		"DevMode":    p.config.IsDevelopmentMode(),
		"BuildTime":  time.Now().Format("2006-01-02 15:04:05"),
	}

	templatePath := filepath.Join(p.projectRoot, "server", "views", "build-error.html")
	output, err := p.tmpl.RenderFile(templatePath, data)
	if err != nil {
		log.Printf("Failed to render error template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(output)))
	w.Write([]byte(output))

	log.Printf("%s %s -> build error page (worker: %s)", r.Method, r.URL.Path, workerName)
}

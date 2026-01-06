package proxy

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	"github.com/mevdschee/tqserver/internal/config"
	"github.com/mevdschee/tqserver/internal/router"
)

// Proxy handles incoming HTTP requests and routes them to backend workers
type Proxy struct {
	config *config.Config
	router router.RouterInterface
	server *http.Server
	mu     sync.RWMutex
}

// NewProxy creates a new reverse proxy
func NewProxy(cfg *config.Config, rtr router.RouterInterface) *Proxy {
	return &Proxy{
		config: cfg,
		router: rtr,
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

	// Increment request count for this worker
	requestCount := worker.IncrementRequestCount()

	// Get worker settings for this path
	settings := p.config.GetWorkerSettings(worker.Route)

	// Check if worker needs to be restarted due to max_requests limit
	if settings.MaxRequests > 0 && requestCount >= settings.MaxRequests {
		log.Printf("Worker on port %d (route: %s) reached max requests (%d), will be restarted", worker.Port, worker.Route, settings.MaxRequests)
		// Note: The actual restart will be handled by the supervisor
	}
}

package worker

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Runtime provides common worker initialization and server management
type Runtime struct {
	Port         string
	Path         string
	Mode         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// NewRuntime creates a new runtime with configuration from environment variables
func NewRuntime() *Runtime {
	port := os.Getenv("WORKER_PORT")
	if port == "" {
		port = "9000"
	}

	path := os.Getenv("WORKER_PATH")
	if path == "" {
		path = "/"
	}

	mode := os.Getenv("WORKER_MODE")
	if mode == "" {
		mode = "dev"
	}

	// Get timeout settings from environment
	readTimeout := parseTimeout("WORKER_READ_TIMEOUT_SECONDS", 30)
	writeTimeout := parseTimeout("WORKER_WRITE_TIMEOUT_SECONDS", 30)
	idleTimeout := parseTimeout("WORKER_IDLE_TIMEOUT_SECONDS", 120)

	return &Runtime{
		Port:         port,
		Path:         path,
		Mode:         mode,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}
}

// IsDevelopmentMode returns true if the worker is running in development mode
func (r *Runtime) IsDevelopmentMode() bool {
	return r.Mode == "dev" || r.Mode == "development"
}

// StartServer starts the HTTP server with the given handler
func (r *Runtime) StartServer(handler http.Handler) error {
	if handler == nil {
		handler = http.DefaultServeMux
	}

	// Wrap with logging middleware
	handler = r.loggingMiddleware(handler)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", r.Port),
		Handler:      handler,
		ReadTimeout:  r.ReadTimeout,
		WriteTimeout: r.WriteTimeout,
		IdleTimeout:  r.IdleTimeout,
	}

	log.Printf("Worker starting on port %s for path %s", r.Port, r.Path)
	return server.ListenAndServe()
}

// loggingMiddleware captures and logs request details
func (r *Runtime) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()

		// Create a wrapper to capture status code (simple version)
		// sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(w, req)

		duration := time.Since(start)
		log.Printf("[Worker %s] %s %s took %v", r.Port, req.Method, req.URL.Path, duration)
	})
}

// parseTimeout parses a timeout from environment variable
func parseTimeout(envVar string, defaultSeconds int) time.Duration {
	if val := os.Getenv(envVar); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return time.Duration(defaultSeconds) * time.Second
}

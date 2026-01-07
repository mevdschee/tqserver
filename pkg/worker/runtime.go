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
	Route        string
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

	route := os.Getenv("WORKER_ROUTE")
	if route == "" {
		route = "/"
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
		Route:        route,
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
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", r.Port),
		Handler:      handler,
		ReadTimeout:  r.ReadTimeout,
		WriteTimeout: r.WriteTimeout,
		IdleTimeout:  r.IdleTimeout,
	}

	log.Printf("Worker starting on port %s for route %s", r.Port, r.Route)
	return server.ListenAndServe()
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

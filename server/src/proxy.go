package main

import (
	"fmt"
	"html/template"
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
	const errorTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Compilation Error - {{.WorkerName}}</title>
	<style>
		body {
			font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
			margin: 0;
			padding: 20px;
			background: #1e1e1e;
			color: #d4d4d4;
		}
		.container {
			max-width: 1200px;
			margin: 0 auto;
		}
		.header {
			background: #d32f2f;
			color: white;
			padding: 20px;
			border-radius: 8px 8px 0 0;
			margin-bottom: 0;
		}
		.header h1 {
			margin: 0 0 10px 0;
			font-size: 24px;
		}
		.header p {
			margin: 0;
			opacity: 0.9;
		}
		.error-content {
			background: #252525;
			padding: 20px;
			border-radius: 0 0 8px 8px;
			border: 1px solid #333;
			border-top: none;
		}
		.error-box {
			background: #1e1e1e;
			border: 1px solid #d32f2f;
			border-left: 4px solid #d32f2f;
			padding: 15px;
			border-radius: 4px;
			overflow-x: auto;
			font-family: 'Courier New', Courier, monospace;
			font-size: 13px;
			line-height: 1.5;
			white-space: pre-wrap;
			word-wrap: break-word;
		}
		.info {
			margin-top: 20px;
			padding: 15px;
			background: #264f78;
			border-left: 4px solid #0e639c;
			border-radius: 4px;
		}
		.info p {
			margin: 5px 0;
		}
		.refresh-note {
			margin-top: 20px;
			padding: 15px;
			background: #2d2d2d;
			border-radius: 4px;
			text-align: center;
			color: #888;
		}
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<h1>⚠️ Compilation Error</h1>
			<p>Worker: <strong>{{.WorkerName}}</strong></p>
		</div>
		<div class="error-content">
			<div class="error-box">{{.BuildError}}</div>
			<div class="info">
				<p><strong>ℹ️ Development Mode</strong></p>
				<p>The compilation failed. Fix the errors in your code and save the file.</p>
				<p>The page will automatically reload once the build succeeds.</p>
			</div>
			<div class="refresh-note">
				<p>💡 This error page is only shown in development mode.</p>
			</div>
		</div>
		<script>
			// Auto-refresh every 2 seconds to check if build is fixed
			setTimeout(() => {
				location.reload();
			}, 2000);
		</script>
	</div>
</body>
</html>`

	tmpl, err := template.New("error").Parse(errorTemplate)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("Failed to parse error template: %v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK) // Use 200 so browser doesn't show its own error page

	data := struct {
		WorkerName string
		BuildError string
	}{
		WorkerName: workerName,
		BuildError: buildError,
	}

	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Failed to execute error template: %v", err)
	}

	log.Printf("%s %s -> build error page (worker: %s)", r.Method, r.URL.Path, workerName)
}

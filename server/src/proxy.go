package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mevdschee/tqserver/pkg/fastcgi"
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

	// In dev mode, set X-TQWorker-* headers for all worker types
	setDevHeaders := func(header http.Header) {
		header.Set("X-TQWorker-Name", worker.Name)
		header.Set("X-TQWorker-Type", worker.Type)
		header.Set("X-TQWorker-Route", worker.Route)
		header.Set("X-TQWorker-Port", fmt.Sprintf("%d", worker.Port))
	}
	devHeadersSet := p.config.IsDevelopmentMode()

	// Check if this is a PHP worker
	if worker.IsPHP {
		// In dev mode, set headers directly (before any write)
		if devHeadersSet {
			setDevHeaders(w.Header())
		}
		// Handle PHP worker via FastCGI protocol
		p.handlePHPRequest(w, r, worker)
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

	if devHeadersSet {
		proxy.ModifyResponse = func(resp *http.Response) error {
			setDevHeaders(resp.Header)
			return nil
		}
	}

	// Trim the worker route prefix before proxying so the backend
	// receives the path it registers (e.g. worker registers /items,
	// proxy should send /items instead of /api/items).
	proxiedReq := r.Clone(r.Context())
	trimmedPath := strings.TrimPrefix(r.URL.Path, worker.Route)
	if trimmedPath == "" {
		trimmedPath = "/"
	}
	proxiedReq.URL.Path = trimmedPath
	proxiedReq.URL.RawPath = trimmedPath
	proxiedReq.RequestURI = ""

	log.Printf("%s %s -> worker on port %d (proxied path: %s)", r.Method, r.URL.Path, worker.Port, trimmedPath)
	proxy.ServeHTTP(w, proxiedReq)

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

// handlePHPRequest converts HTTP request to FastCGI and sends to PHP worker
func (p *Proxy) handlePHPRequest(w http.ResponseWriter, r *http.Request, worker *Worker) {
	// Determine script filename
	documentRoot := filepath.Join(p.projectRoot, p.config.Workers.Directory, worker.Name, "public")

	// Remove route prefix from URL path
	scriptPath := strings.TrimPrefix(r.URL.Path, worker.Route)
	if scriptPath == "" || scriptPath == "/" {
		scriptPath = "/index.php"
	}

	// If path doesn't end in .php, assume it's a directory request for index.php
	if !strings.HasSuffix(scriptPath, ".php") {
		scriptPath = filepath.Join(scriptPath, "index.php")
	}

	scriptFilename := filepath.Join(documentRoot, scriptPath)

	// Read request body
	var requestBody []byte
	if r.Body != nil {
		var err error
		requestBody, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			log.Printf("Failed to read request body: %v", err)
			return
		}
		r.Body.Close()
	}

	// Build FastCGI parameters from HTTP request
	params := make(map[string]string)
	params["GATEWAY_INTERFACE"] = "CGI/1.1"
	params["SERVER_SOFTWARE"] = "TQServer"
	params["SERVER_PROTOCOL"] = r.Proto
	params["SERVER_NAME"] = r.Host
	params["SERVER_PORT"] = fmt.Sprintf("%d", p.config.Server.Port)
	params["REQUEST_METHOD"] = r.Method
	params["REQUEST_URI"] = r.URL.RequestURI()
	params["SCRIPT_FILENAME"] = scriptFilename
	params["SCRIPT_NAME"] = scriptPath
	params["DOCUMENT_ROOT"] = documentRoot
	params["DOCUMENT_URI"] = scriptPath
	params["QUERY_STRING"] = r.URL.RawQuery
	params["REMOTE_ADDR"] = r.RemoteAddr
	params["REMOTE_PORT"] = "0"
	params["CONTENT_TYPE"] = r.Header.Get("Content-Type")
	params["CONTENT_LENGTH"] = fmt.Sprintf("%d", len(requestBody))
	params["REDIRECT_STATUS"] = "200" // Required by CGI-based runtimes (e.g., php-cgi)

	// Add HTTP headers as FastCGI params
	for key, values := range r.Header {
		headerName := "HTTP_" + strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
		params[headerName] = strings.Join(values, ", ")
	}

	// Connect to FastCGI server
	fcgiAddress := fmt.Sprintf("localhost:%d", worker.Port)
	conn, err := net.DialTimeout("tcp", fcgiAddress, 5*time.Second)
	if err != nil {
		http.Error(w, "Failed to connect to PHP worker", http.StatusBadGateway)
		log.Printf("Failed to connect to FastCGI server at %s: %v", fcgiAddress, err)
		return
	}
	defer conn.Close()

	// Create FastCGI connection
	fcgiConn := fastcgi.NewConn(conn, 60*time.Second, 60*time.Second)

	// Send FastCGI request
	requestID := uint16(1)

	// Send BeginRequest
	if err := fcgiConn.SendBeginRequest(requestID, fastcgi.RoleResponder, false); err != nil {
		http.Error(w, "Failed to send FastCGI request", http.StatusInternalServerError)
		log.Printf("Failed to send BeginRequest: %v", err)
		return
	}

	// Send Params
	if err := fcgiConn.SendParams(requestID, params); err != nil {
		http.Error(w, "Failed to send FastCGI parameters", http.StatusInternalServerError)
		log.Printf("Failed to send Params: %v", err)
		return
	}

	// Send empty params to signal end
	if err := fcgiConn.SendParams(requestID, nil); err != nil {
		http.Error(w, "Failed to send FastCGI parameters", http.StatusInternalServerError)
		log.Printf("Failed to send empty Params: %v", err)
		return
	}

	// Send Stdin (request body)
	if len(requestBody) > 0 {
		if err := fcgiConn.SendStdin(requestID, requestBody); err != nil {
			http.Error(w, "Failed to send request body", http.StatusInternalServerError)
			log.Printf("Failed to send Stdin: %v", err)
			return
		}
	}

	// Send empty stdin to signal end
	if err := fcgiConn.SendStdin(requestID, nil); err != nil {
		http.Error(w, "Failed to send request body", http.StatusInternalServerError)
		log.Printf("Failed to send empty Stdin: %v", err)
		return
	}

	// Read response
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	readDone := false
	for !readDone {
		record, err := fcgiConn.ReadRecord()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Failed to read FastCGI response: %v", err)
			break
		}

		switch record.Header.Type {
		case fastcgi.TypeStdout:
			if len(record.Content) > 0 {
				stdout.Write(record.Content)
			}
		case fastcgi.TypeStderr:
			if len(record.Content) > 0 {
				stderr.Write(record.Content)
			}
		case fastcgi.TypeEndRequest:
			// Request complete
			readDone = true
		}
	}

	// Log any stderr output
	if stderr.Len() > 0 {
		log.Printf("[PHP stderr] %s", stderr.String())
	}

	// Parse response headers and body
	responseData := stdout.Bytes()
	headerEnd := bytes.Index(responseData, []byte("\r\n\r\n"))
	if headerEnd == -1 {
		// Try just \n\n
		headerEnd = bytes.Index(responseData, []byte("\n\n"))
		if headerEnd != -1 {
			headerEnd += 2
		}
	} else {
		headerEnd += 4
	}

	if headerEnd > 0 {
		// Parse headers
		headerLines := bytes.Split(responseData[:headerEnd], []byte("\n"))
		for _, line := range headerLines {
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}

			parts := bytes.SplitN(line, []byte(":"), 2)
			if len(parts) == 2 {
				key := string(bytes.TrimSpace(parts[0]))
				value := string(bytes.TrimSpace(parts[1]))

				// Handle special headers
				if strings.ToLower(key) == "status" {
					// Parse status code
					statusParts := strings.SplitN(value, " ", 2)
					if len(statusParts) > 0 {
						var statusCode int
						fmt.Sscanf(statusParts[0], "%d", &statusCode)
						if statusCode > 0 {
							w.WriteHeader(statusCode)
						}
					}
				} else {
					w.Header().Set(key, value)
				}
			}
		}

		// Write body
		w.Write(responseData[headerEnd:])
	} else {
		// No headers, just write all output
		w.Write(responseData)
	}

	// Increment request count
	worker.IncrementRequestCount()

	log.Printf("%s %s -> PHP worker (FastCGI: %s)", r.Method, r.URL.Path, fcgiAddress)
}

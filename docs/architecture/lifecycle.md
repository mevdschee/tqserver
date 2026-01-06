# Request Lifecycle

- [Introduction](#introduction)
- [Lifecycle Overview](#lifecycle-overview)
- [Server Startup](#server-startup)
- [Request Handling](#request-handling)
- [Worker Execution](#worker-execution)
- [Response Flow](#response-flow)
- [Hot Reload Cycle](#hot-reload-cycle)

## Introduction

Understanding the TQServer request lifecycle is essential for building efficient applications. This guide walks through the entire journey of a request, from when it enters the server to when the response is sent back to the client.

## Lifecycle Overview

A typical request flows through these stages:

1. **HTTP Request** - Client sends request to TQServer
2. **Router** - Determines which worker should handle the request
3. **Supervisor** - Ensures worker is running and healthy
4. **Proxy** - Forwards request to worker process
5. **Worker** - Processes request and generates response
6. **Response** - Proxied back through TQServer to client

```
┌──────────┐     ┌──────────┐     ┌────────────┐     ┌──────────┐     ┌────────┐
│  Client  │────▶│ TQServer │────▶│ Supervisor │────▶│  Proxy   │────▶│ Worker │
└──────────┘     └──────────┘     └────────────┘     └──────────┘     └────────┘
     ▲                                                                        │
     │                                                                        │
     └────────────────────────────────────────────────────────────────────────┘
                              Response flows back
```

## Server Startup

When TQServer starts, it follows this initialization sequence:

### 1. Configuration Loading

```go
// Load server.yaml
config := LoadConfig("config/server.yaml")
```

The server reads configuration from `config/server.yaml`, including:
- Server host and port
- Port pool configuration
- Worker settings
- Health check configuration

### 2. Worker Discovery

```go
// Scan workers directory
workers := DiscoverWorkers("workers/")
```

TQServer scans the `workers/` directory to find all available workers. Each subdirectory with a `src/main.go` file is considered a worker.

### 3. Worker Building

```go
// Build each worker
for _, worker := range workers {
    BuildWorker(worker)
}
```

Each worker's source code is compiled into a binary:
- `go build -o workers/{name}/bin/{name} workers/{name}/src`
- Build errors are logged and reported
- Successful builds are tracked in the supervisor

### 4. Port Pool Initialization

```go
// Initialize port pool
portPool := NewPortPool(config.PortPoolStart, config.PortPoolEnd)
```

A pool of available ports is created for worker processes. Ports are allocated dynamically as workers start.

### 5. HTTP Server Start

```go
// Start HTTP server
http.ListenAndServe(":8080", router)
```

The main HTTP server starts listening for incoming requests.

### 6. File Watcher Start

```go
// Watch for file changes
watcher := NewFileWatcher("workers/")
watcher.OnChange(func(path string) {
    RebuildWorker(path)
})
```

The file watcher monitors source files for changes to enable hot reloading.

## Request Handling

When a request arrives at TQServer:

### 1. Request Reception

```go
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Log the request
    log.Printf("%s %s", r.Method, r.URL.Path)
```

The HTTP server receives the request and begins processing.

### 2. Route Matching

```go
    // Determine worker from URL path
    workerName := DetermineWorker(r.URL.Path)
    
    // Example: "/api/users" -> "api" worker
    // Example: "/" -> "index" worker
```

The router analyzes the URL path to determine which worker should handle the request:
- `/` → `index` worker
- `/api/...` → `api` worker
- `/admin/...` → `admin` worker

### 3. Worker Lookup

```go
    // Get worker from supervisor
    worker := supervisor.GetWorker(workerName)
    if worker == nil {
        http.Error(w, "Worker not found", 404)
        return
    }
```

The supervisor is queried for the worker. If the worker doesn't exist, a 404 is returned.

### 4. Health Check

```go
    // Ensure worker is healthy
    if !worker.IsHealthy() {
        // Try to start or restart worker
        supervisor.EnsureWorkerRunning(workerName)
    }
```

Before proxying the request, TQServer verifies the worker is healthy:
- If unhealthy, attempt to restart
- If restart fails, return 503 Service Unavailable

### 5. Port Acquisition

```go
    // Get worker's port
    port := worker.GetPort()
    if port == 0 {
        http.Error(w, "Worker not available", 503)
        return
    }
```

The worker's assigned port is retrieved from the supervisor.

## Worker Execution

The worker process handles the request:

### 1. Request Proxy

```go
    // Forward request to worker
    proxyURL := fmt.Sprintf("http://localhost:%d", port)
    proxy := httputil.NewSingleHostReverseProxy(proxyURL)
    proxy.ServeHTTP(w, r)
}
```

TQServer proxies the request to the worker process running on the allocated port.

### 2. Worker Processing

```go
// Inside worker process
func handler(w http.ResponseWriter, r *http.Request) {
    // Parse request
    data := ParseRequest(r)
    
    // Business logic
    result := ProcessData(data)
    
    // Generate response
    SendResponse(w, result)
}
```

The worker:
- Receives the request
- Executes business logic
- Generates a response
- Sends the response back

### 3. Template Rendering

```go
// Render template
tmpl := template.Must(template.ParseFiles("views/page.html"))
tmpl.Execute(w, data)
```

If using templates, the worker renders HTML with data.

## Response Flow

The response travels back to the client:

### 1. Worker Response

```go
// Worker sends response
w.WriteHeader(http.StatusOK)
w.Write([]byte("Response data"))
```

The worker writes the response to its HTTP response writer.

### 2. Proxy Forwarding

The reverse proxy forwards the response back through TQServer to the client.

### 3. Client Reception

The client receives the complete response.

### 4. Logging

```go
// Log response
log.Printf("%s %s - %d %s", r.Method, r.URL.Path, statusCode, duration)
```

TQServer logs the request details including status code and duration.

## Hot Reload Cycle

When source files change:

### 1. File Change Detection

```go
// File watcher detects change
watcher.OnChange("workers/api/src/main.go", func(path string) {
```

The file watcher detects when a source file is modified.

### 2. Debouncing

```go
    // Wait for rapid changes to settle
    time.Sleep(config.DebounceTime)
```

Multiple rapid changes are debounced to avoid unnecessary rebuilds.

### 3. Worker Rebuild

```go
    // Rebuild the worker
    log.Printf("Rebuilding worker: api")
    err := BuildWorker("api")
    if err != nil {
        log.Printf("Build failed: %v", err)
        return
    }
```

The worker is recompiled from source. Build errors are logged but don't affect the running worker.

### 4. Graceful Restart

```go
    // Start new worker instance
    newWorker := StartWorker("api", newPort)
    
    // Wait for health check
    if !WaitForHealthy(newWorker, 10*time.Second) {
        log.Printf("New worker failed health check")
        StopWorker(newWorker)
        return
    }
    
    // Switch traffic to new worker
    supervisor.SwapWorker(oldWorker, newWorker)
    
    // Gracefully stop old worker
    GracefulShutdown(oldWorker, 30*time.Second)
})
```

A new worker instance is started:
1. New worker starts on a different port
2. Health check confirms it's ready
3. Traffic is switched to the new worker
4. Old worker is gracefully shutdown

This process typically takes 0.3-1.0 seconds and results in zero dropped requests.

## Performance Considerations

### Request Overhead

TQServer adds minimal overhead to requests:
- **Routing**: ~0.1ms
- **Health Check**: ~0.01ms (cached)
- **Proxy**: ~0.5ms
- **Total**: ~1ms overhead

### Concurrency

- Multiple requests are handled concurrently
- Each worker can handle multiple concurrent requests
- Port pool must be sized appropriately

### Worker Lifecycle

- Workers remain running between requests
- No cold start penalty
- Binary is already compiled

## Next Steps

- [Worker Architecture](workers.md) - Deep dive into worker design
- [Hot Reload System](hot-reload.md) - How hot reloading works
- [Supervisor Pattern](supervisor.md) - Worker supervision details
- [Process Isolation](isolation.md) - Security and stability benefits

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9000"
	}

	route := os.Getenv("ROUTE")
	if route == "" {
		route = "/"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Welcome to TQServer</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
            line-height: 1.6;
        }
        h1 { color: #2c3e50; }
        .info { background: #f8f9fa; padding: 15px; border-radius: 5px; margin: 20px 0; }
        .success { color: #27ae60; font-weight: bold; }
    </style>
</head>
<body>
    <h1>✅ TQServer is running!</h1>
    
    <div class="info">
        <p><strong>Route:</strong> %s</p>
        <p><strong>Worker Port:</strong> %s</p>
        <p><strong>Request Method:</strong> %s</p>
        <p><strong>Request Path:</strong> %s</p>
        <p><strong>Time:</strong> %s</p>
    </div>

    <h2>About TQServer</h2>
    <p>TQServer is a high-performance function execution platform built with Go.</p>
    <p>This page is served by a worker process that was automatically built and started by the supervisor.</p>
    
    <h3>Features:</h3>
    <ul>
        <li>Sub-second hot reloads</li>
        <li>Filesystem-based routing</li>
        <li>Graceful worker restarts</li>
        <li>Native Go performance</li>
    </ul>

    <p class="success">Try editing pages/index/main.go and watch it reload automatically!</p>
</body>
</html>`, route, port, r.Method, r.URL.Path, time.Now().Format("2006-01-02 15:04:05"))
	})

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	log.Printf("Worker listening on port %s for route %s", port, route)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

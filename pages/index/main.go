package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/mevdschee/tqtemplate"
)

var tmpl *tqtemplate.Template

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9000"
	}

	route := os.Getenv("ROUTE")
	if route == "" {
		route = "/"
	}

	// Initialize templates with file loader
	loader := func(name string) (string, error) {
		content, err := os.ReadFile(name)
		return string(content), err
	}
	tmpl = tqtemplate.NewTemplateWithLoader(loader)

	// Index route
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)

		// Set content type first
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		data := map[string]interface{}{
			"Route":     route,
			"Port":      port,
			"Method":    r.Method,
			"Path":      r.URL.Path,
			"Time":      time.Now().Format("2006-01-02 15:04:05"),
			"PageTitle": "Welcome to TQServer",
		}

		output, err := tmpl.RenderFile("pages/index/index.html", data)
		if err != nil {
			log.Printf("Template error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(output)))
		io.WriteString(w, output)
	})

	// Hello world route
	http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)

		// Set content type first
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		data := map[string]interface{}{
			"PageTitle": "Hello World",
			"Message":   "Hello, World! This is a simple route.",
			"Time":      time.Now().Format("2006-01-02 15:04:05"),
		}

		output, err := tmpl.RenderFile("pages/index/hello.html", data)
		if err != nil {
			log.Printf("Template error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(output)))
		io.WriteString(w, output)
	})

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Printf("Worker listening on port %s for route %s", port, route)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

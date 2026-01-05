package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	port := flag.Int("port", 8080, "Port to listen on")
	pagesDir := flag.String("pages", "pages", "Directory containing page handlers")
	flag.Parse()

	// Get project root (current working directory)
	projectRoot, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	log.Printf("TQServer starting...")
	log.Printf("Project root: %s", projectRoot)
	log.Printf("Pages directory: %s", *pagesDir)
	log.Printf("Listening on port: %d", *port)

	// Initialize router
	router := NewRouter(*pagesDir, projectRoot)

	// Initialize supervisor
	supervisor := NewSupervisor(*pagesDir, projectRoot, router)

	// Start supervisor (watches for changes and builds workers)
	if err := supervisor.Start(); err != nil {
		log.Fatalf("Failed to start supervisor: %v", err)
	}

	// Initialize and start HTTP proxy/load balancer
	proxy := NewProxy(*port, router)

	// Start proxy in a goroutine
	go func() {
		if err := proxy.Start(); err != nil {
			log.Fatalf("Failed to start proxy: %v", err)
		}
	}()

	log.Printf("✅ TQServer ready on http://localhost:%d", *port)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")

	// Cleanup
	supervisor.Stop()
	proxy.Stop()

	log.Println("Goodbye!")
}

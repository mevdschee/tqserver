package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func main() {
	configPath := flag.String("config", "config/server.yaml", "Path to config file")
	mode := flag.String("mode", "", "Server mode: dev or prod (defaults to TQSERVER_MODE env var or 'dev')")
	flag.Parse()

	// Get project root (current working directory)
	projectRoot, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	// Load configuration
	configFile := filepath.Join(projectRoot, *configPath)
	config, err := LoadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Override mode if specified via flag
	if *mode != "" {
		config.Mode = *mode
	}

	log.Printf("TQServer starting...")
	log.Printf("Mode: %s", config.Mode)
	log.Printf("Project root: %s", projectRoot)
	log.Printf("Config file: %s", configFile)
	log.Printf("Workers directory: %s", config.Workers.Directory)
	log.Printf("Listening on port: %d", config.Server.Port)
	log.Printf("Worker port range: %d-%d", config.Workers.PortRangeStart, config.Workers.PortRangeEnd)

	// Load worker configs
	workerConfigs, err := LoadWorkerConfigs(config.Workers.Directory)
	if err != nil {
		log.Fatalf("Failed to load worker configs: %v", err)
	}
	log.Printf("Loaded %d worker(s)", len(workerConfigs))

	// Initialize router
	router := NewRouter(config.Workers.Directory, projectRoot, workerConfigs)

	// Initialize supervisor
	supervisor := NewSupervisor(config, projectRoot, router, workerConfigs)

	// Start supervisor (watches for changes and builds workers)
	if err := supervisor.Start(); err != nil {
		log.Fatalf("Failed to start supervisor: %v", err)
	}

	// Initialize and start HTTP proxy/load balancer
	proxy := NewProxy(config, router, projectRoot)

	// Connect supervisor with proxy for reload broadcasting
	supervisor.SetProxy(proxy)

	// Start proxy in a goroutine
	go func() {
		if err := proxy.Start(); err != nil {
			log.Fatalf("Failed to start proxy: %v", err)
		}
	}()

	log.Printf("✅ TQServer ready on http://localhost:%d", config.Server.Port)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	for {
		sig := <-sigChan
		if sig == syscall.SIGHUP {
			log.Println("Received SIGHUP, reloading configuration...")

			// Reload configuration
			newConfig, err := LoadConfig(configFile)
			if err != nil {
				log.Printf("Failed to reload config: %v", err)
				continue
			}
			// Override mode if specified via flag
			if *mode != "" {
				newConfig.Mode = *mode
			}

			// Reload worker configs
			newWorkerConfigs, err := LoadWorkerConfigs(newConfig.Workers.Directory)
			if err != nil {
				log.Printf("Failed to reload worker configs: %v", err)
				continue
			}
			log.Printf("Reloaded %d worker(s)", len(newWorkerConfigs))

			supervisor.Reload(newConfig, newWorkerConfigs)
		} else {
			break
		}
	}

	log.Println("Shutting down...")

	// Cleanup
	supervisor.Stop()
	proxy.Stop()

	log.Println("Goodbye!")
}

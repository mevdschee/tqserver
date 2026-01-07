package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mevdschee/tqserver/pkg/php"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	projectRoot := filepath.Join(cwd, "..", "..")
	docRoot := filepath.Join(projectRoot, "workers", "blog", "public")

	config := &php.Config{
		Binary:       "",
		DocumentRoot: docRoot,
		Settings: map[string]string{
			"memory_limit":       "128M",
			"max_execution_time": "30",
			"display_errors":     "1",
			"error_reporting":    "E_ALL",
		},
		Pool: php.PoolConfig{
			Manager:        "static",
			MaxWorkers:     2,
			RequestTimeout: 30 * time.Second,
			IdleTimeout:    10 * time.Second,
			ListenAddr:     "127.0.0.1:9001",
		},
	}

	log.Println("Detecting php-cgi binary...")
	binary, err := php.DetectBinary(config.Binary)
	if err != nil {
		log.Fatalf("Failed to detect php-cgi: %v", err)
	}

	log.Printf("Found PHP: %s", binary.String())

	log.Println("Creating PHP worker manager...")
	manager, err := php.NewManager(binary, config)
	if err != nil {
		log.Fatalf("Failed to create manager: %v", err)
	}

	log.Println("Starting PHP workers...")
	if err := manager.Start(); err != nil {
		log.Fatalf("Failed to start workers: %v", err)
	}

	log.Println("PHP workers started successfully!")
	log.Printf("Workers listening on: %s.<worker_id>", config.Pool.ListenAddr)
	log.Println("\nPress Ctrl+C to stop")

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			stats := manager.GetStats()
			fmt.Println("\n=== PHP Worker Stats ===")
			for k, v := range stats {
				fmt.Printf("%s: %v\n", k, v)
			}
			fmt.Println("\n=== Worker Details ===")
			for _, info := range manager.GetWorkerInfo() {
				fmt.Printf("Worker %d: state=%s, requests=%d, uptime=%s, pid=%d\n",
					info["id"], info["state"], info["request_count"], info["uptime"], info["pid"])
			}
			fmt.Println()
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("\nShutting down PHP workers...")
	if err := manager.Stop(); err != nil {
		log.Fatalf("Error stopping manager: %v", err)
	}

	log.Println("Shutdown complete")
}

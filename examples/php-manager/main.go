package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

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
		PHPFPMBinary: "",
		DocumentRoot: docRoot,
		Settings: map[string]string{
			"memory_limit":       "128M",
			"max_execution_time": "30",
			"display_errors":     "1",
			"error_reporting":    "E_ALL",
		},
		PHPFPM: php.PHPFPMConfig{
			Enabled:   true,
			Listen:    "127.0.0.1:9000",
			Transport: "tcp",
			Pool: php.PoolConfig{
				Name:        "blog",
				PM:          "static",
				MaxChildren: 2,
			},
		},
	}

	// Choose php-fpm binary path (config overrides, otherwise default to "php-fpm")
	binaryPath := config.PHPFPMBinary
	if binaryPath == "" {
		binaryPath = "php-fpm"
	}

	log.Printf("Using php-fpm binary: %s", binaryPath)

	log.Println("Creating PHP worker manager...")
	manager, err := php.NewManager(config)
	if err != nil {
		log.Fatalf("Failed to create manager: %v", err)
	}

	log.Println("Starting PHP workers...")
	if err := manager.Start(); err != nil {
		log.Fatalf("Failed to start workers: %v", err)
	}

	log.Println("PHP workers started successfully!")
	log.Printf("Workers listening on php-fpm listen: %s", config.PHPFPM.Listen)
	log.Println("\nPress Ctrl+C to stop")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("\nShutting down PHP workers...")
	if err := manager.Stop(); err != nil {
		log.Fatalf("Error stopping manager: %v", err)
	}

	log.Println("Shutdown complete")
}

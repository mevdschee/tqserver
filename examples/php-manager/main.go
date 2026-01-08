package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mevdschee/tqserver/pkg/php"
	"github.com/mevdschee/tqserver/pkg/phpfpm"
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

	log.Println("Starting php-fpm via Launcher...")
	launcher := phpfpm.NewLauncher(config)
	if err := launcher.Start(); err != nil {
		log.Fatalf("Failed to start php-fpm: %v", err)
	}

	log.Println("php-fpm started successfully!")
	log.Printf("php-fpm listen: %s", config.PHPFPM.Listen)
	log.Println("\nPress Ctrl+C to stop")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("\nShutting down php-fpm...")
	shutdownTimeout := 5 * time.Second
	if err := launcher.Stop(shutdownTimeout); err != nil {
		log.Fatalf("Error stopping php-fpm: %v", err)
	}

	log.Println("Shutdown complete")
}

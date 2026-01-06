package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mevdschee/tqserver/internal/config"
	"github.com/mevdschee/tqserver/internal/proxy"
	"github.com/mevdschee/tqserver/internal/router"
	"github.com/mevdschee/tqserver/internal/supervisor"
)

func main() {
	configPath := flag.String("config", "config/server.yaml", "Path to config file")
	quiet := flag.Bool("quiet", false, "Suppress log output to stdout/stderr")
	flag.Parse()

	// Configure logging based on quiet flag
	if *quiet {
		log.SetOutput(io.Discard)
	}

	// Get project root (current working directory)
	projectRoot, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	// Load configuration
	configFile := filepath.Join(projectRoot, *configPath)
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Setup server log file if configured and not in quiet mode
	if !*quiet && cfg.Server.LogFile != "" && cfg.Server.LogFile != "~" {
		logFilePath := cfg.Server.LogFile

		// Replace {date} with current date
		dateStr := time.Now().Format("2006-01-02")
		logFilePath = filepath.Join(projectRoot, filepath.FromSlash(logFilePath))
		logFilePath = filepath.Clean(strings.ReplaceAll(logFilePath, "{date}", dateStr))

		// Create log directory if it doesn't exist
		logDir := filepath.Dir(logFilePath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			log.Fatalf("Failed to create log directory: %v", err)
		}

		// Open log file
		logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
		defer logFile.Close()

		// Set log output to both stdout and file
		multiWriter := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(multiWriter)

		log.Printf("Server logging to: %s", logFilePath)
	}

	log.Printf("TQServer starting...")
	log.Printf("Project root: %s", projectRoot)
	log.Printf("Config file: %s", configFile)
	log.Printf("Pages directory: %s", cfg.Pages.Directory)
	log.Printf("Listening on port: %d", cfg.Server.Port)
	log.Printf("Worker port range: %d-%d", cfg.Workers.PortRangeStart, cfg.Workers.PortRangeEnd)

	// Initialize router
	rtr := router.NewRouter(cfg.Pages.Directory, projectRoot)

	// Initialize supervisor
	sup := supervisor.NewSupervisor(cfg, configFile, projectRoot, rtr)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start supervisor (watches for changes and builds workers)
	if err := sup.Start(ctx); err != nil {
		log.Fatalf("Failed to start supervisor: %v", err)
	}

	// Initialize and start HTTP proxy/load balancer
	prx := proxy.NewProxy(sup, rtr)

	// Start proxy in a goroutine
	go func() {
		if err := prx.Start(); err != nil {
			log.Fatalf("Failed to start proxy: %v", err)
		}
	}()

	log.Printf("✅ TQServer ready on http://localhost:%d", cfg.Server.Port)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Cleanup
	sup.Stop(shutdownCtx)
	prx.Stop()

	log.Println("Goodbye!")
}

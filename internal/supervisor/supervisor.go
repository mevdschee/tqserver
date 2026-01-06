package supervisor

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mevdschee/tqserver/internal/config"
	"github.com/mevdschee/tqserver/internal/router"
)

// SupervisorInterface defines the interface for worker supervision
type SupervisorInterface interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	RestartWorker(worker *router.Worker) error
	GetConfig() *config.Config
}

// Supervisor manages worker lifecycle: building, starting, stopping, and restarting
type Supervisor struct {
	config      *config.Config
	configPath  string
	projectRoot string
	router      router.RouterInterface
	watcher     *fsnotify.Watcher
	portPool    *PortPool
	stopChan    chan struct{}
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	mu          sync.Mutex
	configMu    sync.RWMutex
}

// NewSupervisor creates a new supervisor
func NewSupervisor(cfg *config.Config, configPath string, projectRoot string, rtr router.RouterInterface) *Supervisor {
	return &Supervisor{
		config:      cfg,
		configPath:  configPath,
		projectRoot: projectRoot,
		router:      rtr,
		portPool:    NewPortPool(cfg.Workers.PortRangeStart, cfg.Workers.PortRangeEnd),
		stopChan:    make(chan struct{}),
	}
}

// Start starts the supervisor
func (s *Supervisor) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Discover all routes
	if err := s.router.DiscoverRoutes(); err != nil {
		return fmt.Errorf("failed to discover routes: %w", err)
	}

	// Build and start all workers
	workers := s.router.GetAllWorkers()
	for _, worker := range workers {
		if err := s.buildWorker(worker); err != nil {
			log.Printf("Failed to build worker for %s: %v", worker.Route, err)
			continue
		}

		if err := s.startWorker(worker); err != nil {
			log.Printf("Failed to start worker for %s: %v", worker.Route, err)
			continue
		}
	}

	// Setup file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	s.watcher = watcher

	// Watch pages directory
	pagesPath := filepath.Join(s.projectRoot, s.config.Pages.Directory)
	if err := s.watchDirectory(pagesPath); err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	// Watch config file directory
	configDir := filepath.Dir(s.configPath)
	if err := s.watcher.Add(configDir); err != nil {
		log.Printf("Warning: failed to watch config directory %s: %v", configDir, err)
	} else {
		log.Printf("Watching config directory: %s", configDir)
	}

	// Start watching for changes
	s.wg.Add(1)
	go s.watchForChanges()

	// Start health checks
	s.startHealthChecks(s.ctx)

	// Start monitoring worker request counts for max_requests enforcement
	s.wg.Add(1)
	go s.monitorWorkerLimits()

	// Start cleanup of old binaries
	s.wg.Add(1)
	go s.cleanupOldBinaries()

	return nil
}

// Stop stops the supervisor
func (s *Supervisor) Stop(ctx context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	close(s.stopChan)

	if s.watcher != nil {
		s.watcher.Close()
	}

	// Stop all workers
	workers := s.router.GetAllWorkers()
	for _, worker := range workers {
		s.stopWorker(worker)
	}

	// Wait for goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetConfig returns the current configuration (thread-safe)
func (s *Supervisor) GetConfig() *config.Config {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.config
}

// watchDirectory recursively watches a directory and its subdirectories
func (s *Supervisor) watchDirectory(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := s.watcher.Add(path); err != nil {
				log.Printf("Warning: failed to watch %s: %v", path, err)
			} else {
				log.Printf("Watching: %s", path)
			}
		}
		return nil
	})
}

// watchForChanges monitors file system changes
func (s *Supervisor) watchForChanges() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChan:
			return
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}

			// Check if it's the config file
			if event.Name == s.configPath && (event.Op&(fsnotify.Write|fsnotify.Create) != 0) {
				log.Printf("Config file changed: %s", event.Name)
				s.handleConfigChange()
				continue
			}

			// Only process write and create events for .go files
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 && filepath.Ext(event.Name) == ".go" {
				log.Printf("File changed: %s", event.Name)
				s.handleFileChange(event.Name)
			}
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

// handleFileChange handles a file change event
func (s *Supervisor) handleFileChange(filePath string) {
	// Find the page directory for this file
	pageDir := filepath.Dir(filePath)

	// Find the worker for this page
	workers := s.router.GetAllWorkers()
	for _, worker := range workers {
		if worker.Path == pageDir || filepath.Dir(worker.Path) == pageDir {
			log.Printf("Rebuilding worker for %s", worker.Route)

			// Rebuild the worker
			if err := s.buildWorker(worker); err != nil {
				log.Printf("Failed to rebuild worker for %s: %v", worker.Route, err)
				return
			}

			// Restart the worker
			if err := s.RestartWorker(worker); err != nil {
				log.Printf("Failed to restart worker for %s: %v", worker.Route, err)
				return
			}

			log.Printf("✅ Worker reloaded for %s", worker.Route)
			return
		}
	}
}

// handleConfigChange handles a config file change event
func (s *Supervisor) handleConfigChange() {
	log.Printf("Reloading configuration...")

	s.configMu.Lock()
	defer s.configMu.Unlock()

	// Reload the configuration
	if err := s.config.Reload(s.configPath); err != nil {
		log.Printf("Failed to reload config: %v", err)
		return
	}

	log.Printf("✅ Configuration reloaded successfully")

	// Restart all workers with new configuration
	// Note: Workers will pick up new settings (GOMAXPROCS, GOMEMLIMIT, timeouts, etc.) on restart
	workers := s.router.GetAllWorkers()
	for _, worker := range workers {
		log.Printf("Restarting worker for %s with new configuration", worker.Route)
		if err := s.RestartWorker(worker); err != nil {
			log.Printf("Failed to restart worker for %s: %v", worker.Route, err)
		}
	}

	log.Printf("✅ All workers restarted with new configuration")
}

// buildWorker compiles a worker binary
func (s *Supervisor) buildWorker(worker *router.Worker) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create temp directory for binaries if it doesn't exist
	tempDir := s.config.Workers.TempDir
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Generate binary name
	binaryName := fmt.Sprintf("worker_%s_%d",
		filepath.Base(worker.Path),
		time.Now().Unix())
	binaryPath := filepath.Join(tempDir, binaryName)

	log.Printf("Building %s -> %s", worker.Path, binaryPath)

	// Build the worker
	cmd := exec.Command("go", "build", "-o", binaryPath)
	cmd.Dir = worker.Path
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build failed: %w\n%s", err, output)
	}

	// Update worker binary path
	worker.Binary = binaryPath

	log.Printf("✅ Built: %s", binaryPath)
	return nil
}

// startWorker starts a worker process
func (s *Supervisor) startWorker(worker *router.Worker) error {
	port, err := s.portPool.Acquire()
	if err != nil {
		return fmt.Errorf("failed to acquire port: %w", err)
	}

	worker.Port = port

	log.Printf("Starting worker for %s on port %d", worker.Route, port)

	// Get worker settings for this path
	settings := s.config.GetWorkerSettings(worker.Route)

	// Generate log file path from template
	logFileTemplate := settings.LogFile

	// Check if logging to file is disabled (~ or empty string means no file logging)
	var logFile *os.File
	var logFilePath string
	if logFileTemplate != "~" && logFileTemplate != "" {
		// Replace {path} with relative path from pages directory
		// Extract relative path: if worker.Path is "pages/api/users", relPath becomes "api/users"
		pagesDir := filepath.Join(s.projectRoot, s.config.Pages.Directory)
		relPath, relErr := filepath.Rel(pagesDir, worker.Path)
		if relErr != nil {
			relPath = filepath.Base(worker.Path) // Fallback to just the name
		}
		// Normalize to forward slashes for consistent path representation
		pathForLog := strings.ReplaceAll(relPath, string(filepath.Separator), "/")

		// Replace placeholders in the template first
		logFilePath = strings.ReplaceAll(logFileTemplate, "{path}", pathForLog)

		// Replace {date} with current date
		dateStr := time.Now().Format("2006-01-02")
		logFilePath = strings.ReplaceAll(logFilePath, "{date}", dateStr)

		// Make path absolute if not already
		if !filepath.IsAbs(logFilePath) {
			logFilePath = filepath.Join(s.projectRoot, logFilePath)
		}

		// Clean the path
		logFilePath = filepath.Clean(logFilePath)

		// Create log directory if it doesn't exist
		logDir := filepath.Dir(logFilePath)
		if mkdirErr := os.MkdirAll(logDir, 0755); mkdirErr != nil {
			return fmt.Errorf("failed to create log directory: %w", mkdirErr)
		}

		// Create log file for this worker
		var openErr error
		logFile, openErr = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if openErr != nil {
			return fmt.Errorf("failed to create log file: %w", openErr)
		}
	}

	// Start the worker process
	cmd := exec.Command(worker.Binary)
	cmd.Dir = s.projectRoot // Set working directory to project root
	envVars := []string{
		fmt.Sprintf("PORT=%d", port),
		fmt.Sprintf("ROUTE=%s", worker.Route),
		fmt.Sprintf("READ_TIMEOUT_SECONDS=%d", settings.GetReadTimeout()),
		fmt.Sprintf("WRITE_TIMEOUT_SECONDS=%d", settings.GetWriteTimeout()),
		fmt.Sprintf("IDLE_TIMEOUT_SECONDS=%d", settings.IdleTimeoutSeconds),
	}
	// Set GOMAXPROCS if configured
	if settings.GoMaxProcs > 0 {
		envVars = append(envVars, fmt.Sprintf("GOMAXPROCS=%d", settings.GoMaxProcs))
	}
	// Set GOMEMLIMIT if configured
	if settings.GoMemLimit != "" {
		envVars = append(envVars, fmt.Sprintf("GOMEMLIMIT=%s", settings.GoMemLimit))
	}
	cmd.Env = append(os.Environ(), envVars...)

	// Configure output: log to file if configured, otherwise just stdout/stderr
	if logFile != nil {
		cmd.Stdout = io.MultiWriter(os.Stdout, logFile)
		cmd.Stderr = io.MultiWriter(os.Stderr, logFile)
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start worker: %w", err)
	}

	worker.Process = cmd.Process
	worker.StartTime = time.Now()
	worker.SetHealthy(true)

	if logFilePath != "" {
		log.Printf("✅ Worker started for %s on port %d (PID: %d) - logs: %s",
			worker.Route, port, cmd.Process.Pid, logFilePath)
	} else {
		log.Printf("✅ Worker started for %s on port %d (PID: %d) - no file logging",
			worker.Route, port, cmd.Process.Pid)
	}

	// Monitor the process (but don't mark unhealthy on exit if we're restarting)
	go func() {
		procPid := cmd.Process.Pid
		err := cmd.Wait()
		if err != nil {
			log.Printf("Worker for %s (PID: %d) exited: %v", worker.Route, procPid, err)
		} else {
			log.Printf("Worker for %s (PID: %d) exited cleanly", worker.Route, procPid)
		}
		// Only mark unhealthy if this is still the current process
		if worker.Process != nil && worker.Process.Pid == procPid {
			worker.SetHealthy(false)
		}
	}()

	// Give it a moment to start
	time.Sleep(s.config.GetStartupDelay())

	return nil
}

// stopWorker stops a worker process
func (s *Supervisor) stopWorker(worker *router.Worker) error {
	if worker.Process == nil {
		return nil
	}

	log.Printf("Stopping worker for %s (PID: %d)", worker.Route, worker.Process.Pid)

	// Release port back to pool
	if worker.Port > 0 {
		s.portPool.Release(worker.Port)
	}

	if err := worker.Process.Signal(os.Interrupt); err != nil {
		// If graceful shutdown fails, kill it
		worker.Process.Kill()
	}

	worker.SetHealthy(false)
	worker.Process = nil

	return nil
}

// monitorWorkerLimits periodically checks if workers need to be restarted due to limits
func (s *Supervisor) monitorWorkerLimits() {
	defer s.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			workers := s.router.GetAllWorkers()
			for _, worker := range workers {
				if !worker.IsHealthy() {
					continue
				}

				settings := s.config.GetWorkerSettings(worker.Route)

				// Check max_requests limit
				if settings.MaxRequests > 0 {
					requestCount := worker.GetRequestCount()
					if requestCount >= settings.MaxRequests {
						log.Printf("Worker for %s reached max requests (%d/%d), restarting",
							worker.Route, requestCount, settings.MaxRequests)
						if err := s.RestartWorker(worker); err != nil {
							log.Printf("Failed to restart worker for %s: %v", worker.Route, err)
						}
					}
				}
			}
		}
	}
}

// RestartWorker performs a graceful restart of a worker
func (s *Supervisor) RestartWorker(worker *router.Worker) error {
	oldProcess := worker.Process
	oldPort := worker.Port

	// Reset request count before restarting
	worker.ResetRequestCount()

	// Start new worker on new port
	if err := s.startWorker(worker); err != nil {
		return fmt.Errorf("failed to start new worker: %w", err)
	}

	// Stop old worker after a brief delay
	time.Sleep(s.config.GetRestartDelay())
	if oldProcess != nil {
		log.Printf("Stopping old worker on port %d", oldPort)
		// Release old port back to pool
		if oldPort > 0 {
			s.portPool.Release(oldPort)
		}
		oldProcess.Signal(os.Interrupt)

		// Wait a bit for graceful shutdown
		time.Sleep(s.config.GetShutdownGracePeriod())
		// Force kill if still running
		oldProcess.Kill()
	}

	return nil
}

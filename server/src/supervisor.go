package main

import (
    "fmt"
    "log"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "sync"
    "time"
    "context"

    "github.com/fsnotify/fsnotify"
    "github.com/mevdschee/tqserver/pkg/fastcgi"
	"github.com/mevdschee/tqserver/pkg/php"
)

// Supervisor manages worker lifecycle: building, starting, stopping, and restarting
type Supervisor struct {
	config        *Config
	projectRoot   string
	router        *Router
	workerConfigs []*WorkerConfigWithMeta
	watcher       *fsnotify.Watcher
	nextPort      int
	stopChan      chan struct{}
	wg            sync.WaitGroup
	mu            sync.Mutex
	proxy         *Proxy

	// PHP support
	phpManagers    map[string]*php.Manager    // keyed by worker name
	fastcgiServers map[string]*fastcgi.Server // keyed by worker name
}

// NewSupervisor creates a new supervisor
func NewSupervisor(config *Config, projectRoot string, router *Router, workerConfigs []*WorkerConfigWithMeta) *Supervisor {
	return &Supervisor{
		config:         config,
		projectRoot:    projectRoot,
		router:         router,
		workerConfigs:  workerConfigs,
		nextPort:       config.Workers.PortRangeStart,
		stopChan:       make(chan struct{}),
		phpManagers:    make(map[string]*php.Manager),
		fastcgiServers: make(map[string]*fastcgi.Server),
	}
}

// getWorkerConfig finds the worker config for a given worker name
func (s *Supervisor) getWorkerConfig(workerName string) *WorkerConfigWithMeta {
	for _, wc := range s.workerConfigs {
		if wc.Name == workerName {
			return wc
		}
	}
	return nil
}

// SetProxy sets the proxy reference for broadcasting reload events
func (s *Supervisor) SetProxy(proxy *Proxy) {
	s.proxy = proxy
}

// Start starts the supervisor
func (s *Supervisor) Start() error {
	// Discover all routes (just logs them)
	if err := s.router.DiscoverRoutes(); err != nil {
		return fmt.Errorf("failed to discover routes: %w", err)
	}

	// Build and start all workers from worker configs
	for _, workerMeta := range s.workerConfigs {
		// Skip disabled workers
		if !workerMeta.Config.Enabled {
			log.Printf("Worker %s is disabled, skipping", workerMeta.Name)
			continue
		}

		// Create worker entry
		worker := &Worker{
			Name:  workerMeta.Name,
			Route: workerMeta.Config.Path,
		}

		// Register worker with router
		s.router.RegisterWorker(worker)

		// Check if this is a PHP worker
		if workerMeta.Config.Type == "php" {
			worker.IsPHP = true
			if workerMeta.Config.FastCGI != nil {
				worker.FastCGIAddr = workerMeta.Config.FastCGI.Listen
			}
			if err := s.startPHPWorker(worker, workerMeta); err != nil {
				log.Printf("Failed to start PHP worker %s: %v", workerMeta.Name, err)
				continue
			}
		} else {
			// Standard Go worker
			// Build and start the worker
			if err := s.buildWorker(worker); err != nil {
				log.Printf("Failed to build worker %s: %v", workerMeta.Name, err)
				continue
			}

			// Skip starting if there was a build error (in dev mode, error is stored)
			if hasBuildError, _ := worker.GetBuildError(); hasBuildError {
				log.Printf("Skipping start of worker %s due to build error", workerMeta.Name)
				continue
			}

			if err := s.startWorker(worker); err != nil {
				log.Printf("Failed to start worker %s: %v", workerMeta.Name, err)
				continue
			}
		}
	}

	// Setup file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	s.watcher = watcher

	// Watch each worker's source directory
	workersPath := filepath.Join(s.projectRoot, s.config.Workers.Directory)
	if err := s.watchDirectory(workersPath); err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	// Start watching for changes
	s.wg.Add(1)
	go s.watchForChanges()

	// Start monitoring worker request counts for max_requests enforcement
	s.wg.Add(1)
	go s.monitorWorkerLimits()

	return nil
}

// Stop stops the supervisor
func (s *Supervisor) Stop() {
	close(s.stopChan)

	if s.watcher != nil {
		s.watcher.Close()
	}

	// Stop all Go workers
	workers := s.router.GetAllWorkers()
	for _, worker := range workers {
		s.stopWorker(worker)
	}

	// Stop all PHP managers
	s.mu.Lock()
	for name, manager := range s.phpManagers {
		log.Printf("Stopping PHP manager for %s", name)
		manager.Stop()
	}
	// Stop all FastCGI servers
	for name, server := range s.fastcgiServers {
		log.Printf("Stopping FastCGI server for %s", name)
		server.Shutdown(context.Background())
	}
	s.mu.Unlock()

	s.wg.Wait()
}

// watchDirectory recursively watches a directory and its subdirectories
func (s *Supervisor) watchDirectory(dir string) error {
	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Printf("Warning: directory does not exist, skipping watch: %s", dir)
		return nil
	}

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Only watch src/ and config/ directories
			dirName := filepath.Base(path)
			parentPath := filepath.Dir(path)
			parentName := filepath.Base(parentPath)

			// Watch if it's a src or config directory, or if parent is workers directory
			shouldWatch := dirName == "src" || dirName == "config" || parentName == filepath.Base(s.config.Workers.Directory) || path == dir

			if shouldWatch {
				if err := s.watcher.Add(path); err != nil {
					log.Printf("Warning: failed to watch %s: %v", path, err)
				} else {
					log.Printf("Watching: %s", path)
				}
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

			// Only process write and create events
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				ext := filepath.Ext(event.Name)
				dir := filepath.Base(filepath.Dir(event.Name))

				// Handle .go files in src/ directories
				if ext == ".go" && dir == "src" {
					log.Printf("Go file changed: %s", event.Name)
					s.handleFileChange(event.Name)
				}

				// Handle .yaml files in config/ directories
				if (ext == ".yaml" || ext == ".yml") && dir == "config" {
					log.Printf("Config file changed: %s", event.Name)
					s.handleConfigChange(event.Name)
				}
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
		// Check if the changed file belongs to this worker's directory
		workerDir := filepath.Join(s.projectRoot, s.config.Workers.Directory, worker.Name)
		if strings.HasPrefix(pageDir, workerDir) {
			log.Printf("Rebuilding worker for %s", worker.Route)

			// Rebuild the worker
			if err := s.buildWorker(worker); err != nil {
				log.Printf("Failed to rebuild worker for %s: %v", worker.Route, err)
				return
			}

			// If there was a build error (in dev mode), broadcast reload to show error page
			if hasBuildError, _ := worker.GetBuildError(); hasBuildError {
				log.Printf("Worker %s has build error, not restarting", worker.Route)

				// Broadcast reload to show error page in dev mode
				if s.config.IsDevelopmentMode() && s.proxy != nil {
					s.proxy.BroadcastReload()
				}
				return
			}

			// Restart the worker
			if err := s.restartWorker(worker); err != nil {
				log.Printf("Failed to restart worker for %s: %v", worker.Route, err)
				return
			}

			log.Printf("✅ Worker reloaded for %s", worker.Route)

			// Broadcast reload to connected clients in dev mode
			if s.config.IsDevelopmentMode() && s.proxy != nil {
				s.proxy.BroadcastReload()
			}
			return
		}
	}
}

// handleConfigChange handles a worker config file change
func (s *Supervisor) handleConfigChange(filePath string) {
	// Find the worker for this config file
	workers := s.router.GetAllWorkers()
	for _, worker := range workers {
		// Check if the changed config belongs to this worker
		workerDir := filepath.Join(s.projectRoot, s.config.Workers.Directory, worker.Name)
		if strings.HasPrefix(filePath, workerDir) {
			log.Printf("Config changed for worker %s, restarting...", worker.Route)

			// Reload worker config
			configPath := filepath.Join(workerDir, "config", "worker.yaml")
			for _, wc := range s.workerConfigs {
				if wc.Name == worker.Name {
					newConfig, err := LoadWorkerConfig(configPath)
					if err != nil {
						log.Printf("Failed to reload config for worker %s: %v", worker.Name, err)
						return
					}
					wc.Config = *newConfig

					// Update ModTime to prevent re-triggering
					stat, err := os.Stat(configPath)
					if err == nil {
						wc.ModTime = stat.ModTime()
					}

					log.Printf("Reloaded config for worker %s", worker.Name)
					break
				}
			}

			// Restart the worker with new config
			if err := s.restartWorker(worker); err != nil {
				log.Printf("Failed to restart worker for %s: %v", worker.Route, err)
				return
			}

			log.Printf("✅ Worker restarted with new config for %s", worker.Route)

			// Broadcast reload to connected clients in dev mode
			if s.config.IsDevelopmentMode() && s.proxy != nil {
				s.proxy.BroadcastReload()
			}
			return
		}
	}
}

// buildWorker compiles a worker binary
func (s *Supervisor) buildWorker(worker *Worker) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Construct worker paths from name
	workerRoot := filepath.Join(s.projectRoot, s.config.Workers.Directory, worker.Name)
	workerBinDir := filepath.Join(workerRoot, "bin")

	// Create bin directory if it doesn't exist
	if err := os.MkdirAll(workerBinDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Get worker config to determine type
	workerConfig := s.getWorkerConfig(worker.Name)
	workerType := "go" // default
	if workerConfig != nil && workerConfig.Config.Type != "" {
		workerType = workerConfig.Config.Type
	}

	var cmd *exec.Cmd
	var binaryPath string

	switch workerType {
	case "kotlin":
		// For Kotlin workers, use Gradle build
		log.Printf("Building Kotlin worker: %s", worker.Name)

		// Check if gradlew exists
		gradlewPath := filepath.Join(workerRoot, "gradlew")
		if _, err := os.Stat(gradlewPath); os.IsNotExist(err) {
			buildErr := fmt.Errorf("gradlew not found at %s", gradlewPath)
			if s.config.IsDevelopmentMode() {
				worker.SetBuildError(buildErr)
				log.Printf("Build error for %s (dev mode): %v", worker.Name, buildErr)
				return nil
			}
			return buildErr
		}

		// Make gradlew executable
		os.Chmod(gradlewPath, 0755)

		// Run gradle build
		cmd = exec.Command("./gradlew", "clean", "build", "-x", "test")
		cmd.Dir = workerRoot
		cmd.Env = os.Environ()

		// The binary path for Kotlin workers is the wrapper script in bin/
		binaryPath = filepath.Join(workerBinDir, worker.Name)

	default:
		// For Go workers, use go build
		workerSrcDir := filepath.Join(workerRoot, "src")

		// Generate binary name
		binaryName := fmt.Sprintf("tqworker_%s_%d",
			worker.Name,
			time.Now().Unix())
		binaryPath = filepath.Join(workerBinDir, binaryName)

		log.Printf("Building Go worker: %s -> %s", worker.Name, binaryPath)

		cmd = exec.Command("go", "build", "-o", binaryPath)
		cmd.Dir = workerSrcDir
		cmd.Env = os.Environ()
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		buildErr := fmt.Errorf("build failed: %w\n%s", err, output)

		// In development mode, store the error instead of failing
		if s.config.IsDevelopmentMode() {
			worker.SetBuildError(buildErr)
			log.Printf("Build error for %s (dev mode - will serve error page): %v", worker.Name, buildErr)
			return nil // Don't return error in dev mode
		}

		return buildErr
	}

	// Clear any previous build errors
	worker.SetBuildError(nil)

	// Update worker binary path
	worker.Binary = binaryPath

	log.Printf("✅ Built: %s", binaryPath)
	return nil
}

// startWorker starts a worker process
func (s *Supervisor) startWorker(worker *Worker) error {
	s.mu.Lock()
	port := s.nextPort
	s.nextPort++
	// Check if we've exceeded the port range
	if s.nextPort > s.config.Workers.PortRangeEnd {
		log.Printf("Warning: Exceeded worker port range, wrapping back to start")
		s.nextPort = s.config.Workers.PortRangeStart
	}
	s.mu.Unlock()

	worker.Port = port

	log.Printf("Starting worker for %s on port %d", worker.Route, port)

	// Set working directory to worker root (parent of src/)
	// This allows workers to access views/, config/, data/ folders using relative paths
	workerRoot := filepath.Join(s.projectRoot, s.config.Workers.Directory, worker.Name)

	// Get worker-specific configuration
	workerConfig := s.getWorkerConfig(worker.Name)

	// Build environment variables
	envVars := []string{
		fmt.Sprintf("WORKER_PORT=%d", port),
		fmt.Sprintf("WORKER_NAME=%s", worker.Name),
		fmt.Sprintf("WORKER_ROUTE=%s", worker.Route),
		fmt.Sprintf("WORKER_MODE=%s", s.config.Mode),
	}

	// Add timeout settings from worker config if available
	if workerConfig != nil {
		if workerConfig.Config.Timeouts.ReadTimeoutSeconds > 0 {
			envVars = append(envVars, fmt.Sprintf("WORKER_READ_TIMEOUT_SECONDS=%d", workerConfig.Config.Timeouts.ReadTimeoutSeconds))
		}
		if workerConfig.Config.Timeouts.WriteTimeoutSeconds > 0 {
			envVars = append(envVars, fmt.Sprintf("WORKER_WRITE_TIMEOUT_SECONDS=%d", workerConfig.Config.Timeouts.WriteTimeoutSeconds))
		}
		if workerConfig.Config.Timeouts.IdleTimeoutSeconds > 0 {
			envVars = append(envVars, fmt.Sprintf("WORKER_IDLE_TIMEOUT_SECONDS=%d", workerConfig.Config.Timeouts.IdleTimeoutSeconds))
		}
		if workerConfig.Config.Runtime.GOMAXPROCS > 0 {
			envVars = append(envVars, fmt.Sprintf("GOMAXPROCS=%d", workerConfig.Config.Runtime.GOMAXPROCS))
		}
		if workerConfig.Config.Runtime.GOMEMLIMIT != "" {
			envVars = append(envVars, fmt.Sprintf("GOMEMLIMIT=%s", workerConfig.Config.Runtime.GOMEMLIMIT))
		}
	}

	// Start the worker process
	cmd := exec.Command(worker.Binary)
	cmd.Dir = workerRoot // Set working directory to worker root (e.g., workers/index/)
	cmd.Env = append(os.Environ(), envVars...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start worker: %w", err)
	}

	worker.Process = cmd.Process
	worker.StartTime = time.Now()
	worker.SetHealthy(true)

	log.Printf("✅ Worker started for %s on port %d (PID: %d)",
		worker.Route, port, cmd.Process.Pid)

	// Monitor the process
	go func() {
		procPid := cmd.Process.Pid
		err := cmd.Wait()
		if err != nil {
			log.Printf("Worker for %s (PID: %d) exited: %v", worker.Route, procPid, err)
		} else {
			log.Printf("Worker for %s (PID: %d) exited cleanly", worker.Route, procPid)
		}

		// Only mark as unhealthy if this is still the current process
		// (prevents old process exits from affecting new processes after restart)
		if worker.Process != nil && worker.Process.Pid == procPid {
			worker.SetHealthy(false)
		}
	}()

	// Give it a moment to start
	time.Sleep(s.config.GetStartupDelay())

	return nil
}

// stopWorker stops a worker process
func (s *Supervisor) stopWorker(worker *Worker) error {
	if worker.Process == nil {
		return nil
	}

	log.Printf("Stopping worker for %s (PID: %d)", worker.Route, worker.Process.Pid)

	if err := worker.Process.Signal(os.Interrupt); err != nil {
		// If graceful shutdown fails, kill it
		worker.Process.Kill()
	}

	worker.SetHealthy(false)
	worker.Process = nil

	return nil
}

// startPHPWorker starts a PHP worker pool with FastCGI server
func (s *Supervisor) startPHPWorker(worker *Worker, workerMeta *WorkerConfigWithMeta) error {
	if workerMeta.Config.PHP == nil {
		return fmt.Errorf("PHP configuration not found for worker %s", worker.Name)
	}

	log.Printf("Starting PHP worker pool for %s (dynamic manager)", worker.Name)

	// Determine document root
	workerRoot := filepath.Join(s.projectRoot, s.config.Workers.Directory, worker.Name)
	documentRoot := filepath.Join(workerRoot, "public")

	// Detect PHP binary
	binary, err := php.DetectBinary(workerMeta.Config.PHP.Binary)
	if err != nil {
		return fmt.Errorf("failed to detect PHP binary: %w", err)
	}

	log.Printf("Using PHP binary: %s (version %s)", binary.Path, binary.Version)

	// Build PHP configuration
	phpConfig := &php.Config{
		Binary:       binary.Path,
		ConfigFile:   workerMeta.Config.PHP.ConfigFile,
		Settings:     workerMeta.Config.PHP.Settings,
		DocumentRoot: documentRoot,
		Pool: php.PoolConfig{
			Manager:        workerMeta.Config.PHP.Pool.Manager,
			MinWorkers:     workerMeta.Config.PHP.Pool.MinWorkers,
			MaxWorkers:     workerMeta.Config.PHP.Pool.MaxWorkers,
			StartWorkers:   workerMeta.Config.PHP.Pool.StartWorkers,
			MaxRequests:    workerMeta.Config.PHP.Pool.MaxRequests,
			RequestTimeout: time.Duration(workerMeta.Config.PHP.Pool.RequestTimeout) * time.Second,
			IdleTimeout:    time.Duration(workerMeta.Config.PHP.Pool.IdleTimeout) * time.Second,
			ListenAddr:     workerMeta.Config.FastCGI.Listen,
		},
	}

	// Create PHP manager
	manager, err := php.NewManager(binary, phpConfig)
	if err != nil {
		return fmt.Errorf("failed to create PHP manager: %w", err)
	}

	// Start the pool
	if err := manager.Start(); err != nil {
		return fmt.Errorf("failed to start PHP pool: %w", err)
	}

	// Store manager
	s.mu.Lock()
	s.phpManagers[worker.Name] = manager
	s.mu.Unlock()

	// Create FastCGI handler
	handler := php.NewFastCGIHandler(manager)

	// Create and start FastCGI server
	fcgiServer := fastcgi.NewServer(workerMeta.Config.FastCGI.Listen, handler)
	go func() {
		if err := fcgiServer.ListenAndServe(); err != nil {
			log.Printf("FastCGI server for %s stopped: %v", worker.Name, err)
		}
	}()

	// Store server
	s.mu.Lock()
	s.fastcgiServers[worker.Name] = fcgiServer
	s.mu.Unlock()

	// Mark worker as healthy (PHP workers don't have a Process)
	worker.SetHealthy(true)

	log.Printf("✅ PHP worker pool started for %s on %s (%s mode: %d-%d workers)",
		worker.Name,
		workerMeta.Config.FastCGI.Listen,
		workerMeta.Config.PHP.Pool.Manager,
		workerMeta.Config.PHP.Pool.MinWorkers,
		workerMeta.Config.PHP.Pool.MaxWorkers,
	)

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
					// Worker is unhealthy, attempt to restart it
					log.Printf("Detected unhealthy worker for %s, attempting restart...", worker.Route)
					if err := s.restartWorker(worker); err != nil {
						log.Printf("Failed to restart unhealthy worker for %s: %v", worker.Route, err)
					} else {
						log.Printf("✅ Successfully restarted unhealthy worker for %s", worker.Route)
					}
					continue
				}

				// Check max_requests limit from worker config
				workerConfig := s.getWorkerConfig(worker.Name)
				if workerConfig != nil && workerConfig.Config.Runtime.MaxRequests > 0 {
					requestCount := worker.GetRequestCount()
					if requestCount >= workerConfig.Config.Runtime.MaxRequests {
						log.Printf("Worker %s reached max_requests limit (%d/%d), restarting...",
							worker.Name, requestCount, workerConfig.Config.Runtime.MaxRequests)
						if err := s.restartWorker(worker); err != nil {
							log.Printf("Failed to restart worker %s due to max_requests: %v", worker.Name, err)
						} else {
							log.Printf("✅ Worker %s restarted after reaching max_requests", worker.Name)
						}
					}
				}
			}
		}
	}
}

// restartWorker performs a graceful restart of a worker
func (s *Supervisor) restartWorker(worker *Worker) error {
	oldProcess := worker.Process
	oldPort := worker.Port

	// Reset request count before restarting
	worker.mu.Lock()
	worker.RequestCount = 0
	worker.mu.Unlock()

	// Check if binary exists; if not, rebuild
	if worker.Binary == "" || !fileExists(worker.Binary) {
		log.Printf("Worker binary missing for %s, rebuilding...", worker.Route)
		if err := s.buildWorker(worker); err != nil {
			return fmt.Errorf("failed to rebuild worker: %w", err)
		}
	}

	// Start new worker on new port
	if err := s.startWorker(worker); err != nil {
		return fmt.Errorf("failed to start new worker: %w", err)
	}

	// Stop old worker after a brief delay
	time.Sleep(s.config.GetRestartDelay())
	if oldProcess != nil {
		log.Printf("Stopping old worker on port %d", oldPort)
		oldProcess.Signal(os.Interrupt)

		// Wait a bit for graceful shutdown
		time.Sleep(s.config.GetShutdownGracePeriod())
		// Force kill if still running
		oldProcess.Kill()
	}

	return nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

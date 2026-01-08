package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mevdschee/tqserver/pkg/config/php"
	"github.com/mevdschee/tqserver/pkg/phpfpm"
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
	// php-fpm supervised instances + clients (single-port per worker)
	phpLaunchers map[string]*phpfpm.Launcher
	phpClients   map[string]*phpfpm.Client
}

// getFreePort returns the next available port for a worker instance
func (s *Supervisor) getFreePort() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	port := s.nextPort
	s.nextPort++
	if s.nextPort > s.config.Workers.PortRangeEnd {
		s.nextPort = s.config.Workers.PortRangeStart
	}
	return port
}

// NewSupervisor creates a new supervisor
func NewSupervisor(config *Config, projectRoot string, router *Router, workerConfigs []*WorkerConfigWithMeta) *Supervisor {
	return &Supervisor{
		config:        config,
		projectRoot:   projectRoot,
		router:        router,
		workerConfigs: workerConfigs,
		nextPort:      config.Workers.PortRangeStart,
		stopChan:      make(chan struct{}),
		phpLaunchers:  make(map[string]*phpfpm.Launcher),
		phpClients:    make(map[string]*phpfpm.Client),
	}
}

// SetProxy sets the proxy reference for broadcasting reload events
func (s *Supervisor) SetProxy(proxy *Proxy) {
	s.proxy = proxy
}

// Start starts the supervisor
func (s *Supervisor) Start() error {
	// Discover all routes
	if err := s.router.DiscoverRoutes(); err != nil {
		return fmt.Errorf("failed to discover routes: %w", err)
	}

	// Initialize and start all workers
	for _, workerMeta := range s.workerConfigs {
		if !workerMeta.Config.Enabled {
			log.Printf("Worker %s is disabled, skipping", workerMeta.Name)
			continue
		}

		worker := &Worker{
			Name:           workerMeta.Name,
			Route:          workerMeta.Config.Path,
			Type:           workerMeta.Config.Type,
			Instances:      make([]*WorkerInstance, 0),
			Queue:          make(chan *WorkerRequest, 1000), // Default buffer
			MinWorkers:     1,
			MaxWorkers:     5,
			QueueThreshold: 10,
			ScaleDownDelay: 60,
		}

		// Apply scaling config
		if workerMeta.Config.Scaling != nil {
			worker.MinWorkers = workerMeta.Config.Scaling.MinWorkers
			worker.MaxWorkers = workerMeta.Config.Scaling.MaxWorkers
			worker.QueueThreshold = workerMeta.Config.Scaling.QueueThreshold
			worker.ScaleDownDelay = workerMeta.Config.Scaling.ScaleDownDelay
		}
		if worker.MinWorkers < 1 {
			worker.MinWorkers = 1
		}
		if worker.MaxWorkers < worker.MinWorkers {
			worker.MaxWorkers = worker.MinWorkers
		}

		s.router.RegisterWorker(worker)

		if worker.Type == "php" {
			// PHP uses its own manager (php-fpm)
			if err := s.startPHPWorker(worker, workerMeta); err != nil {
				log.Printf("Failed to start PHP worker %s: %v", workerMeta.Name, err)
			}
		} else {
			// Start Service (Bun/Go)
			if err := s.buildWorker(worker); err != nil {
				log.Printf("Failed to build worker %s: %v", worker.Name, err)
				worker.SetBuildError(err)
				// Continue to start dispatcher anyway so we can serve error pages
			}

			// Start the dispatcher loop for scaling and request handling
			s.wg.Add(1)
			go s.runWorkerDispatcher(worker)
		}
	}

	// Setup file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	s.watcher = watcher

	workersPath := filepath.Join(s.projectRoot, s.config.Workers.Directory)
	if err := s.watchDirectory(workersPath); err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	s.wg.Add(1)
	go s.watchForChanges()

	return nil
}

// Stop stops the supervisor
func (s *Supervisor) Stop() {
	close(s.stopChan)
	if s.watcher != nil {
		s.watcher.Close()
	}

	// Stop all workers
	workers := s.router.GetAllWorkers()
	for _, worker := range workers {
		s.stopWorker(worker)
	}

	// Stop PHP
	s.mu.Lock()
	shutdownTimeout := time.Duration(s.config.Workers.ShutdownGracePeriodMs) * time.Millisecond
	for _, launcher := range s.phpLaunchers {
		if launcher != nil {
			launcher.Stop(shutdownTimeout)
		}
	}
	s.mu.Unlock()

	s.wg.Wait()
}

// runWorkerDispatcher manages the worker pool, request distribution, and scaling
func (s *Supervisor) runWorkerDispatcher(w *Worker) {
	defer s.wg.Done()

	ticker := time.NewTicker(2 * time.Second) // Scaling check interval
	defer ticker.Stop()

	// Initial scale up to min workers
	for len(w.Instances) < w.MinWorkers {
		if _, err := s.scaleUp(w); err != nil {
			log.Printf("Failed to start initial worker for %s: %v", w.Name, err)
			time.Sleep(1 * time.Second)
		}
	}

	for {
		select {
		case <-s.stopChan:
			return

		case req := <-w.Queue:
			// Handle request
			// Simple Load Balancer: Round Robin
			// Note: This logic runs in the dispatcher goroutine, serialized.
			// It ensures we don't pick a dead instance, but doesn't block heavily.

			// If no instances, try to start one frantically
			if len(w.Instances) == 0 {
				log.Printf("No instances for %s! Attempting emergency scale up.", w.Name)
				if _, err := s.scaleUp(w); err != nil {
					log.Printf("Emergency scale up failed: %v", err)
					req.ResponseChan <- nil // Return nil to signal failure
					continue
				}
			}

			// Round Robin
			w.mu.Lock()
			idx := w.NextInstance % len(w.Instances)
			instance := w.Instances[idx]
			w.NextInstance++

			// Update stats
			instance.LastRequest = time.Now()
			w.mu.Unlock()

			req.ResponseChan <- instance

		case <-ticker.C:
			// Auto-scaling logic
			queueDepth := len(w.Queue)

			w.mu.Lock()
			numWorkers := len(w.Instances)
			w.mu.Unlock()

			// Scale UP
			if queueDepth > w.QueueThreshold && numWorkers < w.MaxWorkers {
				log.Printf("[Scaling] %s: Queue depth %d > %d. Scaling up.", w.Name, queueDepth, w.QueueThreshold)
				go s.scaleUp(w) // prevent blocking dispatcher
			}

			// Scale DOWN
			// If queue is empty and workers are idle
			if queueDepth == 0 && numWorkers > w.MinWorkers {
				s.scaleDown(w)
			}
		}
	}
}

// scaleUp starts a new worker instance
func (s *Supervisor) scaleUp(w *Worker) (*WorkerInstance, error) {
	// Build worker if needed (should be done already, but verify?)
	// Proceed to spawn
	return s.spawnWorkerInstance(w)
}

// scaleDown stops idle worker instances
func (s *Supervisor) scaleDown(w *Worker) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Find idle instances
	now := time.Now()
	activeInstances := make([]*WorkerInstance, 0)

	for _, inst := range w.Instances {
		// Keep instance if:
		// 1. We are at or below min workers (safety)
		// 2. Instance is active (LastRequest < ScaleDownDelay)
		if len(activeInstances) < w.MinWorkers {
			activeInstances = append(activeInstances, inst)
			continue
		}

		idleDuration := now.Sub(inst.LastRequest)
		if idleDuration.Seconds() < float64(w.ScaleDownDelay) {
			activeInstances = append(activeInstances, inst)
		} else {
			// Terminate
			log.Printf("[Scaling] %s: Scaling down instance %s (Idle %.0fs)", w.Name, inst.ID, idleDuration.Seconds())
			go s.terminateInstance(inst)
		}
	}
	w.Instances = activeInstances
}

// spawnWorkerInstance starts a single process for the worker
func (s *Supervisor) spawnWorkerInstance(w *Worker) (*WorkerInstance, error) {
	s.mu.Lock()
	port := s.getFreePort() // Lock safe (internal lock)
	s.mu.Unlock()

	workerRoot := filepath.Join(s.projectRoot, s.config.Workers.Directory, w.Name)
	workerMeta := s.getWorkerConfig(w.Name)

	// Prepare command
	var cmd *exec.Cmd

	if w.Type == "bun" {
		entrypoint := "index.ts" // Default
		if workerMeta != nil && workerMeta.Config.Bun != nil && workerMeta.Config.Bun.Entrypoint != "" {
			entrypoint = workerMeta.Config.Bun.Entrypoint
		}
		// Find bun binary
		bunPath, err := s.findBunBinary()
		if err != nil {
			return nil, err
		}
		cmd = exec.Command(bunPath, "run", entrypoint)
	} else {
		// "go" default
		binaryPath := filepath.Join(workerRoot, "bin", w.Name)
		cmd = exec.Command(binaryPath)
	}

	cmd.Dir = workerRoot

	// Environment
	env := os.Environ()
	env = append(env, fmt.Sprintf("WORKER_PORT=%d", port))
	env = append(env, fmt.Sprintf("WORKER_NAME=%s", w.Name))
	env = append(env, fmt.Sprintf("WORKER_ROUTE=%s", w.Route))
	env = append(env, fmt.Sprintf("PORT=%d", port)) // Standard for many libs

	if w.Type == "bun" && workerMeta != nil && workerMeta.Config.Bun != nil {
		for k, v := range workerMeta.Config.Bun.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	cmd.Env = env

	// Create logs directory
	logFile, err := os.Create(filepath.Join(s.projectRoot, "logs", fmt.Sprintf("%s_%d.log", w.Name, port)))
	if err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr // Fallback
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	inst := &WorkerInstance{
		ID:          fmt.Sprintf("%s-%d-%d", w.Name, port, time.Now().UnixNano()), // Manual ID using time
		Port:        port,
		Process:     cmd.Process,
		StartTime:   time.Now(),
		LastRequest: time.Now(),
		Healthy:     true,
	}

	w.mu.Lock()
	w.Instances = append(w.Instances, inst)
	w.mu.Unlock()

	log.Printf("Spawned worker instance %s for %s on port %d", inst.ID, w.Name, port)

	// Monitor process exit
	go func() {
		_ = cmd.Wait()
		log.Printf("Worker instance %s exited", inst.ID)

		w.mu.Lock()
		defer w.mu.Unlock()
		// Remove from instances list
		newInstances := make([]*WorkerInstance, 0)
		for _, i := range w.Instances {
			if i.ID != inst.ID {
				newInstances = append(newInstances, i)
			}
		}
		w.Instances = newInstances
	}()

	// Wait for port to be open
	s.waitForPort(port)

	return inst, nil
}

func (s *Supervisor) waitForPort(port int) {
	timeout := time.After(s.config.GetPortWaitTimeout())
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	addr := fmt.Sprintf("localhost:%d", port)
	for {
		select {
		case <-timeout:
			return
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
			if err == nil {
				conn.Close()
				return
			}
		}
	}
}

// terminateInstance stops a worker process
func (s *Supervisor) terminateInstance(inst *WorkerInstance) {
	if inst.Process != nil {
		inst.Process.Signal(os.Interrupt)
		time.Sleep(100 * time.Millisecond)
		inst.Process.Kill()
	}
}

// stopWorker stops all instances of a worker
func (s *Supervisor) stopWorker(w *Worker) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, inst := range w.Instances {
		s.terminateInstance(inst)
	}
	w.Instances = nil
}

// buildWorker builds the worker
func (s *Supervisor) buildWorker(worker *Worker) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	workerRoot := filepath.Join(s.projectRoot, s.config.Workers.Directory, worker.Name)

	if worker.Type == "bun" {
		// Install dependencies
		if _, err := os.Stat(filepath.Join(workerRoot, "package.json")); err == nil {
			// Find bun binary
			bunPath, err := s.findBunBinary()
			if err != nil {
				return err
			}
			cmd := exec.Command(bunPath, "install")
			cmd.Dir = workerRoot
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("bun install failed: %w", err)
			}
		}
		return nil
	} else if worker.Type == "go" {
		// Go build
		binDir := filepath.Join(workerRoot, "bin")
		os.MkdirAll(binDir, 0755)
		binPath := filepath.Join(binDir, worker.Name)

		cmd := exec.Command("go", "build", "-o", binPath, "./src")
		cmd.Dir = workerRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("go build failed: %s", out)
		}
		return nil
	}

	return nil
}

// Helper: getWorkerConfig
func (s *Supervisor) getWorkerConfig(name string) *WorkerConfigWithMeta {
	for _, wc := range s.workerConfigs {
		if wc.Name == name {
			return wc
		}
	}
	return nil
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
			if strings.HasPrefix(filepath.Base(path), ".") {
				return filepath.SkipDir
			}
			s.watcher.Add(path)
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
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				s.handleFileEvent(event.Name)
			}
		case <-s.watcher.Errors:
			return
		}
	}
}

// handleFileEvent
func (s *Supervisor) handleFileEvent(path string) {
	// Identify worker
	// If code changed -> rebuild and restart all instances (rolling restart?)
	// For now: restart all.

	// Find worker
	workers := s.router.GetAllWorkers()
	for _, w := range workers {
		workerDir := filepath.Join(s.projectRoot, s.config.Workers.Directory, w.Name)
		if strings.HasPrefix(path, workerDir) {
			log.Printf("Change detected in %s, reloading worker %s", path, w.Name)

			// Rebuild
			if err := s.buildWorker(w); err != nil {
				w.SetBuildError(err)
				log.Printf("Build failed: %v", err)
				if s.proxy != nil {
					s.proxy.BroadcastReload()
				}
				return
			}
			w.SetBuildError(nil)

			// Rolling Restart:
			// For each instance, kill it. Logic in dispatcher will respawn it if needed.
			// Or we can just call stopWorker and let dispatcher respawn.
			// But dispatcher is running.
			// Let's just kill all instances. Dispatcher loop will see MinWorkers > len(Instances) and spawn new ones.
			s.stopWorker(w)

			if s.proxy != nil {
				s.proxy.BroadcastReload()
			}
			return
		}
	}
}

// startPHPWorker (legacy PHP support, keeping minimal)
func (s *Supervisor) startPHPWorker(worker *Worker, workerMeta *WorkerConfigWithMeta) error {
	// Simplified PHP starter
	port := s.getFreePort()
	// In new structs, Worker doesn't have Port. We need to create an Instance.
	// But PHP is special because it manages its own pool.
	// We can treat the PHP-FPM Listener as the "Instance".

	// Build listen address: prefer configured listen_address, fall back to localhost
	host := workerMeta.Config.PHP.Pool.ListenAddress
	if host == "" {
		host = "127.0.0.1"
	}
	fcgiServerAddr := fmt.Sprintf("%s:%d", host, port)

	// If the chosen port is already bound by another process (e.g., system php-fpm),
	// probe and pick the next free port. This avoids falsely succeeding when
	// `net.Dial` connects to an unrelated service on the same port.
	maxAttempts := s.config.Workers.PortRangeEnd - s.config.Workers.PortRangeStart + 1
	tried := 0
	for tried < maxAttempts {
		// try to listen briefly to check availability
		ln, err := net.Listen("tcp", fcgiServerAddr)
		if err == nil {
			_ = ln.Close()
			break
		}
		// port in use, pick next
		port = s.getFreePort()
		fcgiServerAddr = fmt.Sprintf("%s:%d", host, port)
		tried++
	}
	if tried >= maxAttempts {
		return fmt.Errorf("no free port available in range %d-%d", s.config.Workers.PortRangeStart, s.config.Workers.PortRangeEnd)
	}

	log.Printf("Starting PHP worker pool for %s (dynamic manager)", worker.Name)

	// Determine document root
	workerRoot := filepath.Join(s.projectRoot, s.config.Workers.Directory, worker.Name)
	documentRoot := filepath.Join(workerRoot, "public")

	// Determine php-fpm binary to execute. Prefer worker-specified binary,
	// otherwise try common names (php-fpm, php-cgi, php) and scan PATH for
	// php-fpm* executables.
	findPHPBinary := func(preferred string) (string, error) {
		if preferred != "" {
			if p, err := exec.LookPath(preferred); err == nil {
				return p, nil
			}
			// try as provided path
			if _, err := os.Stat(preferred); err == nil {
				return preferred, nil
			}
		}

		candidates := []string{"php-fpm", "php-fpm8.3", "php-fpm8.2", "php-fpm8.1", "php-fpm8.0", "php-fpm7.4", "php-cgi", "php"}
		for _, c := range candidates {
			if p, err := exec.LookPath(c); err == nil {
				return p, nil
			}
		}

		// Scan PATH directories for files starting with php-fpm
		pathEnv := os.Getenv("PATH")
		for _, dir := range strings.Split(pathEnv, ":") {
			if dir == "" {
				continue
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				name := e.Name()
				if strings.HasPrefix(name, "php-fpm") {
					full := filepath.Join(dir, name)
					if st, err := os.Stat(full); err == nil {
						if st.Mode().Perm()&0111 != 0 {
							return full, nil
						}
					}
				}
			}
		}

		// Also check common sbin directories where system packages may install php-fpm
		sbinDirs := []string{"/usr/sbin", "/sbin", "/usr/local/sbin"}
		for _, dir := range sbinDirs {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				name := e.Name()
				if strings.HasPrefix(name, "php-fpm") || strings.HasPrefix(name, "php") {
					full := filepath.Join(dir, name)
					if st, err := os.Stat(full); err == nil {
						if st.Mode().Perm()&0111 != 0 {
							return full, nil
						}
					}
				}
			}
		}

		return "", fmt.Errorf("php-fpm binary not found in PATH; install php-fpm or set php.binary in worker config")
	}

	binaryPath, err := findPHPBinary(workerMeta.Config.PHP.Binary)
	if err != nil {
		return err
	}

	// Prepare environment variables for PHP worker
	envVars := map[string]string{
		"WORKER_SERVER_MODE": s.config.Mode,
		"WORKER_NAME":        worker.Name,
		"WORKER_ROUTE":       worker.Route,
		"WORKER_PORT":        fmt.Sprintf("%d", port),
		"WORKER_TYPE":        worker.Type,
		"WORKER_PATH":        worker.Route,
	}

	cfg := &php.Config{
		PHPFPMBinary: binaryPath,
		PHPIni:       workerMeta.Config.PHP.ConfigFile,
		DocumentRoot: documentRoot,
		Settings:     workerMeta.Config.PHP.Settings,
		PHPFPM: php.PHPFPMConfig{
			Enabled:            true,
			Listen:             fcgiServerAddr,
			Transport:          "tcp",
			GeneratedConfigDir: filepath.Join(os.TempDir(), "tqserver-phpfpm", worker.Name),
			NoDaemonize:        true,
			Env:                envVars,
		},
	}

	// Map pool fields
	cfg.PHPFPM.Pool = php.PoolConfig{
		Name:                    worker.Name,
		PM:                      workerMeta.Config.PHP.Pool.Manager,
		MaxChildren:             workerMeta.Config.PHP.Pool.MaxWorkers,
		StartServers:            workerMeta.Config.PHP.Pool.StartWorkers,
		MinSpareServers:         workerMeta.Config.PHP.Pool.MinWorkers,
		MaxSpareServers:         workerMeta.Config.PHP.Pool.MaxWorkers,
		MaxRequests:             workerMeta.Config.PHP.Pool.MaxRequests,
		RequestTerminateTimeout: time.Duration(workerMeta.Config.PHP.Pool.RequestTimeout) * time.Second,
		ProcessIdleTimeout:      time.Duration(workerMeta.Config.PHP.Pool.IdleTimeout) * time.Second,
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid php-fpm config: %w", err)
	}

	// Start php-fpm via launcher
	launcher := phpfpm.NewLauncher(cfg)

	if err := launcher.Start(); err != nil {
		return fmt.Errorf("failed to start php-fpm: %w", err)
	}

	// Wait for php-fpm
	ready := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", cfg.PHPFPM.Listen, 250*time.Millisecond)
		if err == nil {
			conn.Close()
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		_ = launcher.Stop(1 * time.Second)
		return fmt.Errorf("php-fpm did not become ready on %s", cfg.PHPFPM.Listen)
	}

	// Create client
	poolSize := cfg.PHPFPM.Pool.MaxChildren
	if poolSize <= 0 {
		poolSize = 2
	}
	client := phpfpm.NewClient(cfg.PHPFPM.Listen, cfg.PHPFPM.Transport, poolSize, 5*time.Second, cfg.PHPFPM.Pool.RequestTerminateTimeout)

	s.mu.Lock()
	s.phpLaunchers[worker.Name] = launcher
	s.phpClients[worker.Name] = client
	s.mu.Unlock()

	// Register pseudo-instance for proxy to find
	inst := &WorkerInstance{
		ID:        "php-master",
		Port:      port,
		Healthy:   true,
		StartTime: time.Now(),
	}
	worker.Instances = append(worker.Instances, inst)

	log.Printf("✅ PHP Worker pool started for %s on %s", worker.Route, fcgiServerAddr)
	return nil
}

// findBunBinary attempts to locate the Bun binary
func (s *Supervisor) findBunBinary() (string, error) {
	// 1. Try PATH
	if p, err := exec.LookPath("bun"); err == nil {
		return p, nil
	}

	// 2. Try common locations
	homeDir, err := os.UserHomeDir()
	if err == nil {
		bunPath := filepath.Join(homeDir, ".bun", "bin", "bun")
		if _, err := os.Stat(bunPath); err == nil {
			return bunPath, nil
		}
	}

	return "", fmt.Errorf("bun executable not found in PATH or ~/.bun/bin/bun")
}

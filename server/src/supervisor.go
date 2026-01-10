package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
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

	// Hot reload support
	reloadTimers map[string]*time.Timer
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
		reloadTimers:  make(map[string]*time.Timer),
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
		if !workerMeta.Config.IsEnabled(s.config.Mode) {
			log.Printf("Worker %s is disabled, skipping", workerMeta.Name)
			continue
		}

		worker := &Worker{
			Name:           workerMeta.Name,
			Path:           workerMeta.Config.Path,
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
				GetMetrics().RecordBuildError(worker.Name)
				// Continue to start dispatcher anyway so we can serve error pages
			}

			// Initial startup: start workers sequentially to avoid load spikes
			// We try to start up to MinWorkers here. If any fail, the dispatcher will handle retries.
			for i := 0; i < worker.MinWorkers; i++ {
				if _, err := s.scaleUp(worker); err != nil {
					log.Printf("Failed to start initial worker instance for %s: %v", worker.Name, err)
					break // Stop synchronous startup on error, let dispatcher retry
				}
				// Add a small delay between starts to spread the load
				if i < worker.MinWorkers-1 {
					time.Sleep(s.config.GetStartupDelay())
				}
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

	// Start monitoring worker health
	s.wg.Add(1)
	go s.monitorWorkerHealth()

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
			// Check for instances safely
			w.mu.Lock()
			if len(w.Instances) == 0 {
				w.mu.Unlock()
				log.Printf("No instances for %s! Attempting emergency scale up.", w.Name)
				if _, err := s.scaleUp(w); err != nil {
					log.Printf("Emergency scale up failed: %v", err)
					req.ResponseChan <- nil // Return nil to signal failure
					continue
				}
				w.mu.Lock()
			}

			// Re-check inside lock
			if len(w.Instances) == 0 {
				w.mu.Unlock()
				req.ResponseChan <- nil
				continue
			}

			// Round Robin
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

			// Maintain MinWorkers (Healing)
			if numWorkers < w.MinWorkers {
				log.Printf("[Scaling] %s: Workers %d < Min %d. Scaling up (healing).", w.Name, numWorkers, w.MinWorkers)
				go s.scaleUp(w)
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
	// s.mu.Lock() - Removed to avoid deadlock as getFreePort locks internally
	port := s.getFreePort()
	// s.mu.Unlock()

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
	env = append(env, fmt.Sprintf("WORKER_PATH=%s", w.Path))
	env = append(env, fmt.Sprintf("WORKER_TYPE=%s", w.Type))
	env = append(env, fmt.Sprintf("WORKER_MODE=%s", s.config.Mode))
	env = append(env, fmt.Sprintf("PORT=%d", port)) // Standard for many libs

	if w.Type == "bun" && workerMeta != nil && workerMeta.Config.Bun != nil {
		for k, v := range workerMeta.Config.Bun.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// SOCKS5 proxy environment variables
	if s.config.Socks5.Enabled {
		env = append(env, fmt.Sprintf("SOCKS5_PROXY=socks5://127.0.0.1:%d", s.config.Socks5.Port))
		env = append(env, fmt.Sprintf("ALL_PROXY=socks5://127.0.0.1:%d", s.config.Socks5.Port))
		env = append(env, fmt.Sprintf("TQSERVER_WORKER_UA=TQServer/%s", w.Name))
		if s.config.Socks5.HTTPSInspection != nil && s.config.Socks5.HTTPSInspection.Enabled {
			caCert := s.config.Socks5.HTTPSInspection.CACert
			if !filepath.IsAbs(caCert) {
				caCert = filepath.Join(s.projectRoot, caCert)
			}
			env = append(env, fmt.Sprintf("SSL_CERT_FILE=%s", caCert))
			env = append(env, fmt.Sprintf("NODE_EXTRA_CA_CERTS=%s", caCert))
		}
	}

	cmd.Env = env

	// Determine log file path
	logPath := fmt.Sprintf("logs/%s_%d.log", w.Name, port) // Default
	if workerMeta != nil {
		if workerMeta.Config.Logging.LogFile != "" {
			logPath = workerMeta.Config.Logging.LogFile
		} else if workerMeta.Config.LogFile != "" {
			logPath = workerMeta.Config.LogFile
		}
	}

	// Replace placeholders
	logPath = strings.ReplaceAll(logPath, "{name}", w.Name)
	logPath = strings.ReplaceAll(logPath, "{port}", fmt.Sprintf("%d", port))
	logPath = strings.ReplaceAll(logPath, "{date}", time.Now().Format("2006-01-02"))

	// Resolve absolute path
	if !filepath.IsAbs(logPath) {
		logPath = filepath.Join(s.projectRoot, logPath)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		log.Printf("Failed to create log directory: %v", err)
	}

	// Create/Open log file
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	} else {
		log.Printf("Failed to open log file %s: %v", logPath, err)
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

	log.Printf("Spawned worker instance %s for %s on port %d, waiting for health...", inst.ID, w.Name, port)

	// Wait for health check to pass
	if err := s.waitForHealth(port); err != nil {
		log.Printf("Worker %s failed health check: %v", inst.ID, err)
		// Cleanup failed process
		cmd.Process.Kill()
		return nil, fmt.Errorf("worker failed health check: %w", err)
	}

	w.mu.Lock()
	w.Instances = append(w.Instances, inst)
	w.mu.Unlock()

	log.Printf("Worker instance %s is ready and added to pool", inst.ID)

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

	return inst, nil
}

func (s *Supervisor) waitForHealth(port int) error {
	timeoutDuration := s.config.GetHealthCheckWaitTimeout()
	timeout := time.After(timeoutDuration)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	url := fmt.Sprintf("http://localhost:%d/health", port)
	client := http.Client{
		Timeout: 500 * time.Millisecond,
	}

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for port %d after %v", port, timeoutDuration)
		case <-ticker.C:
			resp, err := client.Get(url)
			if err == nil {
				if resp.StatusCode == http.StatusOK {
					resp.Body.Close()
					return nil
				}
				resp.Body.Close()
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

// scheduleReload debounces the reload trigger
func (s *Supervisor) scheduleReload(w *Worker) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if t, ok := s.reloadTimers[w.Name]; ok {
		t.Stop()
	}

	// Debounce: wait for no events for 500ms before reloading
	s.reloadTimers[w.Name] = time.AfterFunc(500*time.Millisecond, func() {
		s.reloadWorker(w)
	})
}

// reloadWorker rebuilds and restarts the worker
func (s *Supervisor) reloadWorker(w *Worker) {
	log.Printf("Reloading worker %s (change detected)", w.Name)

	// Rebuild
	if err := s.buildWorker(w); err != nil {
		w.SetBuildError(err)
		GetMetrics().RecordBuildError(w.Name)
		log.Printf("Build failed for worker %s: %v", w.Name, err)
		if s.proxy != nil {
			s.proxy.BroadcastReload()
		}
		return
	}
	w.SetBuildError(nil)

	// Record restart metric
	GetMetrics().RecordWorkerRestart(w.Name)

	// Rolling Restart:
	// For each instance, kill it. Logic in dispatcher will respawn it if needed.
	// Or we can just call stopWorker and let dispatcher respawn.
	// But dispatcher is running.
	// Let's just kill all instances. Dispatcher loop will see MinWorkers > len(Instances) and spawn new ones.
	s.stopWorker(w)

	if s.proxy != nil {
		s.proxy.BroadcastReload()
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
			base := filepath.Base(path)
			// Skip hidden dirs, bin, and node_modules
			if strings.HasPrefix(base, ".") || base == "bin" || base == "node_modules" {
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
			// Build artifacts might still trigger events if the parent dir is watched,
			// or if the ignore logic above missed something.
			// Double check in handleFileEvent.
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
	// Ignore changes in bin or node_modules (extra safety)
	if strings.Contains(path, "/bin/") || strings.Contains(path, "/node_modules/") {
		return
	}

	// Find matching worker
	workers := s.router.GetAllWorkers()
	for _, w := range workers {
		workerDir := filepath.Join(s.projectRoot, s.config.Workers.Directory, w.Name)
		if strings.HasPrefix(path, workerDir) {
			log.Printf("Change detected in %s, reloading worker %s", path, w.Name)

			// Rebuild
			if err := s.buildWorker(w); err != nil {
				w.SetBuildError(err)
				GetMetrics().RecordBuildError(w.Name)
				log.Printf("Build failed: %v", err)
				if s.proxy != nil {
					s.proxy.BroadcastReload()
				}
				return
			}
			w.SetBuildError(nil)

			// Record restart metric
			GetMetrics().RecordWorkerRestart(w.Name)

			// Rolling Restart:
			// For each instance, kill it. Logic in dispatcher will respawn it if needed.
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
		"WORKER_PATH":        worker.Path,
		"WORKER_PORT":        fmt.Sprintf("%d", port),
		"WORKER_TYPE":        worker.Type,
	}

	// SOCKS5 proxy environment variables for PHP
	if s.config.Socks5.Enabled {
		envVars["SOCKS5_PROXY"] = fmt.Sprintf("socks5://127.0.0.1:%d", s.config.Socks5.Port)
		envVars["ALL_PROXY"] = fmt.Sprintf("socks5://127.0.0.1:%d", s.config.Socks5.Port)
		envVars["TQSERVER_WORKER_UA"] = fmt.Sprintf("TQServer/%s", worker.Name)
		if s.config.Socks5.HTTPSInspection != nil && s.config.Socks5.HTTPSInspection.Enabled {
			caCert := s.config.Socks5.HTTPSInspection.CACert
			if !filepath.IsAbs(caCert) {
				caCert = filepath.Join(s.projectRoot, caCert)
			}
			envVars["SSL_CERT_FILE"] = caCert
		}
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

	log.Printf("✅ PHP Worker pool started for %s on %s", worker.Path, fcgiServerAddr)
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

// Reload reloads configuration and restarts workers
func (s *Supervisor) Reload(newConfig *Config, newWorkerConfigs []*WorkerConfigWithMeta) {
	log.Println("Reloading supervisor configuration...")
	s.mu.Lock()
	s.config = newConfig
	s.workerConfigs = newWorkerConfigs
	s.mu.Unlock()

	// Trigger rolling restart for all active workers
	workers := s.router.GetAllWorkers()
	for _, w := range workers {
		// Find config for this worker
		found := false
		for _, wc := range newWorkerConfigs {
			if wc.Name == w.Name {
				found = true
				break
			}
		}

		if found {
			go s.rollingRestart(w)
		} else {
			log.Printf("Worker %s removed from config, stopping...", w.Name)
			s.stopWorker(w)
		}
	}
}

// rollingRestart performs a zero-downtime restart of a worker
func (s *Supervisor) rollingRestart(w *Worker) {
	if w.Type == "php" {
		log.Printf("Rolling restart not fully implemented for PHP worker %s", w.Name)
		return
	}

	log.Printf("Rolling restart for worker %s", w.Name)

	w.mu.Lock()
	currentCount := len(w.Instances)
	if currentCount < w.MinWorkers {
		currentCount = w.MinWorkers
	}
	w.mu.Unlock()

	newInstances := make([]*WorkerInstance, 0)
	for i := 0; i < currentCount; i++ {
		inst, err := s.spawnWorkerInstance(w)
		if err != nil {
			log.Printf("Failed to spawn new instance for %s during reload: %v", w.Name, err)
		} else {
			newInstances = append(newInstances, inst)
		}
	}

	if len(newInstances) > 0 {
		log.Printf("Started %d new instances for %s. Stopping old instances...", len(newInstances), w.Name)

		w.mu.Lock()
		allInstances := w.Instances
		w.mu.Unlock()

		for _, inst := range allInstances {
			isNew := false
			for _, newInst := range newInstances {
				if inst.ID == newInst.ID {
					isNew = true
					break
				}
			}

			if !isNew {
				log.Printf("Stopping old instance %s", inst.ID)
				go s.terminateInstance(inst)
			}
		}

		// Broadcast reload in dev mode so browser updates
		if s.config.Mode == "dev" && s.proxy != nil {
			s.proxy.BroadcastReload()
		}
	}
}

// monitorWorkerHealth periodically checks if workers are healthy
func (s *Supervisor) monitorWorkerHealth() {
	defer s.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			// Check all workers
			workers := s.router.GetAllWorkers()
			for _, worker := range workers {
				// For PHP workers, perform active health check via TCP
				if worker.Type == "php" {
					if !s.checkPHPHealth(worker) {
						log.Printf("PHP worker active health check failed for %s, restarting...", worker.Name)
						// Restart the worker
						// Note: This is a bit aggressive, but consistent with previous behavior
						// We run this in a goroutine to not block the monitor loop
						go func(w *Worker) {
							s.stopWorker(w)
							// Get config to restart correctly
							workerMeta := s.getWorkerConfig(w.Name)
							if workerMeta != nil {
								if err := s.startPHPWorker(w, workerMeta); err != nil {
									log.Printf("Failed to restart PHP worker %s: %v", w.Name, err)
								}
							}
						}(worker)
					}
				} else {
					// For Bun/Go workers, check HTTP health endpoint
					if !s.checkHTTPHealth(worker) {
						log.Printf("Worker active health check failed for %s, restarting...", worker.Name)
						go func(w *Worker) {
							// Rolling restart: stop all instances and let dispatcher respawn them
							// Or we could restart specific instances if we tracked which one failed.
							// For simplicity, we restart the whole worker group for now.
							s.stopWorker(w)
							// Dispatcher loop will notice 0 instances and scale up.
						}(worker)
					}
				}
			}
		}
	}
}

// checkHTTPHealth performs an active HTTP GET to /health on worker instances
func (s *Supervisor) checkHTTPHealth(worker *Worker) bool {
	worker.mu.Lock()
	instances := make([]*WorkerInstance, len(worker.Instances))
	copy(instances, worker.Instances)
	worker.mu.Unlock()

	if len(instances) == 0 {
		return false
	}

	// We check all instances. If ANY is unhealthy, we return false for the worker?
	// Or we probably want to try to keep the worker alive but maybe kill the bad instance?
	// The current logic in monitorWorkerHealth restarts the WHOLE worker if this returns false.
	// So let's return false only if ALL instances fail, or if a majority fail?
	// Or maybe we should just return true if at least one is healthy?
	// Given the restart logic above (stops all instances), let's be strict:
	// If any instance is explicitly BAD (connection refused / 500), we probably want to fix it.
	// But restarting everything for one bad instance is harsh.
	// Let's iterate and if we find a bad one, we terminate THAT instance individually here?
	// But the function signature returns `bool` for the whole worker.
	// Let's change the pattern: checkHTTPHealth cleans up bad instances and returns true if at least one remains healthy.

	healthyCount := 0
	client := http.Client{
		Timeout: 500 * time.Millisecond,
	}
	metrics := GetMetrics()

	for _, inst := range instances {
		start := time.Now()
		url := fmt.Sprintf("http://localhost:%d/health", inst.Port)
		resp, err := client.Get(url)
		duration := time.Since(start)
		isHealthy := false
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				isHealthy = true
			}
			resp.Body.Close()
		}

		// Record health check metrics
		metrics.RecordHealthCheck(worker.Name, duration, isHealthy)

		if isHealthy {
			healthyCount++
		} else {
			log.Printf("Broadcasting health check failure for instance %s (%s)", inst.ID, url)
			// We can proactively terminate this specific bad instance
			// and let the dispatcher replace it.
			go s.terminateInstance(inst)
		}
	}

	// Update worker gauge metrics
	metrics.UpdateWorkerMetrics(worker.Name, len(instances), healthyCount, len(worker.Queue), healthyCount > 0)

	// If we have at least one healthy instance, or if we had 0 instances to start with (caught above),
	// we say the worker "group" is fine (the bad ones are being killed).
	// If healthyCount == 0, then we might need the big restart hammer from the caller.
	return healthyCount > 0
}

// checkPHPHealth performs an active TCP probe to check if the PHP worker is reachable
func (s *Supervisor) checkPHPHealth(worker *Worker) bool {
	// For now, checks the first instance (PHP pool master)
	// In the future we might want to check all instances if we have multiple PHP pools?
	// But startPHPWorker creates one 'php-master' instance with the port.
	if len(worker.Instances) == 0 {
		return false
	}

	// Find the php-master instance or just use the first one
	port := worker.Instances[0].Port
	if port == 0 {
		return false
	}

	workerMeta := s.getWorkerConfig(worker.Name)
	if workerMeta == nil || workerMeta.Config.PHP == nil {
		return false
	}

	host := workerMeta.Config.PHP.Pool.ListenAddress
	if host == "" {
		host = "127.0.0.1"
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
	duration := time.Since(start)
	isHealthy := err == nil

	// Record health check metrics
	metrics := GetMetrics()
	metrics.RecordHealthCheck(worker.Name, duration, isHealthy)
	metrics.UpdateWorkerMetrics(worker.Name, len(worker.Instances), 1, 0, isHealthy)

	if !isHealthy {
		// Only log verbose if we want to debug, otherwise it spams if down
		// log.Printf("Health check failed for PHP worker %s (%s): %v", worker.Name, addr, err)
		return false
	}
	conn.Close()
	return true
}

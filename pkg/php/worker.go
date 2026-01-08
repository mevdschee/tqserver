package php

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// WorkerState represents the current state of a PHP worker
type WorkerState int

const (
	WorkerStateIdle WorkerState = iota
	WorkerStateActive
	WorkerStateTerminating
	WorkerStateCrashed
)

func (s WorkerState) String() string {
	switch s {
	case WorkerStateIdle:
		return "idle"
	case WorkerStateActive:
		return "active"
	case WorkerStateTerminating:
		return "terminating"
	case WorkerStateCrashed:
		return "crashed"
	default:
		return "unknown"
	}
}

// Worker represents a single php-cgi process
type Worker struct {
	ID         int
	cmd        *exec.Cmd
	binary     *Binary
	config     *Config
	socketPath string // Internal FastCGI socket for this worker

	state        atomic.Value // WorkerState
	requestCount atomic.Int64
	startTime    time.Time
	lastUsed     time.Time
	mu           sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc

	// Channels for lifecycle management
	done   chan struct{}
	errors chan error
}

// NewWorker creates a new PHP worker
func NewWorker(id int, binary *Binary, config *Config, socketPath string) *Worker {
	ctx, cancel := context.WithCancel(context.Background())

	w := &Worker{
		ID:         id,
		binary:     binary,
		config:     config,
		socketPath: socketPath,
		startTime:  time.Now(),
		lastUsed:   time.Now(),
		ctx:        ctx,
		cancel:     cancel,
		done:       make(chan struct{}),
		errors:     make(chan error, 1),
	}

	w.setState(WorkerStateIdle)
	return w
}

// Start spawns the php-cgi process
func (w *Worker) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cmd != nil {
		return fmt.Errorf("worker already started")
	}

	// Build command arguments (with -b flag for FastCGI mode)
	args := w.binary.BuildArgs(w.config, w.socketPath)

	// Create command with context for cancellation
	w.cmd = exec.CommandContext(w.ctx, w.binary.Path, args...)

	// Set environment variables for php-cgi
	w.cmd.Env = append(os.Environ(),
		fmt.Sprintf("PHP_FCGI_MAX_REQUESTS=%d", w.config.Pool.MaxRequests),
	)

	// Set working directory to current directory
	// php-cgi will use SCRIPT_FILENAME (absolute path) to find files
	cwd, _ := os.Getwd()
	w.cmd.Dir = cwd
	log.Printf("[Worker %d] Working directory: %s", w.ID, cwd)

	// Capture stdout/stderr for logging
	stdout, err := w.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := w.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := w.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start php-cgi: %w", err)
	}

	log.Printf("[Worker %d] Starting php-cgi (PID: %d) on %s", w.ID, w.cmd.Process.Pid, w.socketPath)

	// Start goroutines to handle output and process monitoring
	go w.handleOutput(stdout, "stdout")
	go w.handleOutput(stderr, "stderr")
	go w.monitor()

	// Give php-cgi time to bind to the socket and verify it didn't crash
	time.Sleep(100 * time.Millisecond)

	// Check if process is still running
	if w.cmd.ProcessState != nil && w.cmd.ProcessState.Exited() {
		return fmt.Errorf("php-cgi exited immediately after start - check port %s is available", w.socketPath)
	}

	log.Printf("[Worker %d] php-cgi bound successfully to %s", w.ID, w.socketPath)

	// Verify we can actually connect to the worker
	testConn, err := net.DialTimeout("tcp", w.socketPath, 1*time.Second)
	if err != nil {
		log.Printf("[Worker %d] WARNING: Cannot connect to worker socket: %v", w.ID, err)
	} else {
		testConn.Close()
		log.Printf("[Worker %d] Verified socket is accepting connections", w.ID)
	}

	return nil
}

// Stop gracefully stops the worker
func (w *Worker) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cmd == nil || w.cmd.Process == nil {
		return nil
	}

	w.setState(WorkerStateTerminating)
	log.Printf("[Worker %d] Stopping (PID: %d)", w.ID, w.cmd.Process.Pid)

	// Send SIGTERM to php-cgi process
	if err := w.cmd.Process.Signal(os.Interrupt); err != nil {
		log.Printf("[Worker %d] Failed to send SIGTERM, will force kill: %v", w.ID, err)
	}

	// Cancel context to signal shutdown
	w.cancel()

	// Wait for process to exit (with timeout)
	done := make(chan error, 1)
	go func() {
		done <- w.cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil && err.Error() != "signal: killed" {
			log.Printf("[Worker %d] Exited with error: %v", w.ID, err)
		}
		close(w.done)
		return err
	case <-time.After(5 * time.Second):
		// Force kill if not stopped gracefully
		if err := w.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
		close(w.done)
		return fmt.Errorf("worker killed after timeout")
	}
}

// monitor watches the process and handles crashes
func (w *Worker) monitor() {
	err := w.cmd.Wait()

	w.mu.Lock()
	currentState := w.getState()
	w.mu.Unlock()

	// Only mark as crashed if not intentionally terminating
	if currentState != WorkerStateTerminating {
		w.setState(WorkerStateCrashed)
		if err != nil {
			log.Printf("[Worker %d] Crashed: %v", w.ID, err)
			w.errors <- fmt.Errorf("worker crashed: %w", err)
		} else {
			log.Printf("[Worker %d] Exited unexpectedly", w.ID)
			w.errors <- fmt.Errorf("worker exited unexpectedly")
		}
	}

	close(w.done)
}

// handleOutput reads and logs output from the process
func (w *Worker) handleOutput(reader io.Reader, streamName string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if streamName == "stderr" {
			// Make stderr very visible
			log.Printf("[Worker %d] ⚠️  STDERR: %s", w.ID, line)
		} else {
			log.Printf("[Worker %d] [%s] %s", w.ID, streamName, line)
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		log.Printf("[Worker %d] Error reading %s: %v", w.ID, streamName, err)
	}
}

// MarkActive marks the worker as actively processing a request
func (w *Worker) MarkActive() {
	w.setState(WorkerStateActive)
	w.requestCount.Add(1)
	w.mu.Lock()
	w.lastUsed = time.Now()
	w.mu.Unlock()
}

// MarkIdle marks the worker as idle
func (w *Worker) MarkIdle() {
	w.setState(WorkerStateIdle)
	w.mu.Lock()
	w.lastUsed = time.Now()
	w.mu.Unlock()
}

// GetState returns the current worker state
func (w *Worker) GetState() WorkerState {
	return w.getState()
}

func (w *Worker) getState() WorkerState {
	if state := w.state.Load(); state != nil {
		return state.(WorkerState)
	}
	return WorkerStateIdle
}

func (w *Worker) setState(state WorkerState) {
	w.state.Store(state)
}

// GetRequestCount returns the total number of requests handled
func (w *Worker) GetRequestCount() int64 {
	return w.requestCount.Load()
}

// GetUptime returns how long the worker has been running
func (w *Worker) GetUptime() time.Duration {
	return time.Since(w.startTime)
}

// GetIdleTime returns how long the worker has been idle
func (w *Worker) GetIdleTime() time.Duration {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.getState() == WorkerStateIdle {
		return time.Since(w.lastUsed)
	}
	return 0
}

// ShouldRestart checks if the worker should be restarted
func (w *Worker) ShouldRestart() bool {
	// Check if max requests limit is reached
	if w.config.Pool.MaxRequests > 0 && w.GetRequestCount() >= int64(w.config.Pool.MaxRequests) {
		return true
	}

	// Check if worker has crashed
	if w.getState() == WorkerStateCrashed {
		return true
	}

	return false
}

// Wait waits for the worker to exit
func (w *Worker) Wait() error {
	<-w.done
	return nil
}

// Errors returns a channel for worker errors
func (w *Worker) Errors() <-chan error {
	return w.errors
}

// IsHealthy checks if the worker is in a healthy state
func (w *Worker) IsHealthy() bool {
	state := w.getState()
	return state == WorkerStateIdle || state == WorkerStateActive
}

// GetPID returns the process ID, or 0 if not running
func (w *Worker) GetPID() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.cmd != nil && w.cmd.Process != nil {
		return w.cmd.Process.Pid
	}
	return 0
}

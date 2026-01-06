package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mevdschee/tqserver/pkg/coordinator"
	"github.com/mevdschee/tqserver/pkg/modecontroller"
)

// MockWorkerManager is a simple implementation for demonstration.
type MockWorkerManager struct {
	workers map[string]string
}

func NewMockWorkerManager() *MockWorkerManager {
	return &MockWorkerManager{
		workers: make(map[string]string),
	}
}

func (m *MockWorkerManager) StopWorker(name string) error {
	log.Printf("MockWorkerManager: Stopping worker %s", name)
	m.workers[name] = "stopped"
	return nil
}

func (m *MockWorkerManager) StartWorker(name string) error {
	log.Printf("MockWorkerManager: Starting worker %s", name)
	m.workers[name] = "starting"
	// Simulate startup delay
	time.Sleep(200 * time.Millisecond)
	m.workers[name] = "healthy"
	return nil
}

func (m *MockWorkerManager) GetWorkerStatus(name string) (string, error) {
	status, ok := m.workers[name]
	if !ok {
		return "unknown", nil
	}
	return status, nil
}

func (m *MockWorkerManager) RestartWorker(name string) error {
	coord := coordinator.New(m)
	return coord.RestartWorker(name)
}

func (m *MockWorkerManager) RestartServer() error {
	log.Println("MockWorkerManager: Server restart requested")
	return nil
}

func main() {
	// Create mock worker manager
	manager := NewMockWorkerManager()

	// Get mode from environment (defaults to dev)
	mode := modecontroller.GetModeFromEnv()
	log.Printf("Running in %s mode", mode)

	// Create mode controller
	controller, err := modecontroller.New(modecontroller.Config{
		Mode:            mode,
		WorkersDir:      "workers",
		ServerDir:       "server",
		ConfigDir:       "config",
		ServerBinPath:   "server/bin/tqserver",
		DebounceMs:      100,
		WorkerRestarter: manager,
	})
	if err != nil {
		log.Fatalf("Failed to create controller: %v", err)
	}

	// Start the controller
	if err := controller.Start(); err != nil {
		log.Fatalf("Failed to start controller: %v", err)
	}
	defer controller.Stop()

	log.Println("Controller started. Press Ctrl+C to stop.")
	log.Println("In dev mode: Edit files in workers/*/src/ to trigger rebuild")
	log.Println("In prod mode: Send SIGHUP signal to check for changes")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
}

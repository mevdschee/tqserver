package modecontroller

import (
	"fmt"
	"log"
	"os"

	"github.com/mevdschee/tqserver/pkg/devmode"
	"github.com/mevdschee/tqserver/pkg/prodmode"
	"github.com/mevdschee/tqserver/pkg/supervisor"
)

// Mode represents the deployment mode.
type Mode string

const (
	Development Mode = "dev"
	Production  Mode = "prod"
)

// WorkerRestarter defines the interface for restarting workers.
type WorkerRestarter interface {
	RestartWorker(workerName string) error
	RestartServer() error
}

// Controller manages the deployment mode (dev or prod).
type Controller struct {
	mode            Mode
	devMode         *devmode.DevMode
	prodMode        *prodmode.ProdMode
	workerRestarter WorkerRestarter
}

// Config holds controller configuration.
type Config struct {
	Mode            Mode
	WorkersDir      string
	ServerDir       string
	ConfigDir       string
	ServerBinPath   string
	DebounceMs      int
	WorkerRestarter WorkerRestarter
}

// New creates a new mode controller.
func New(cfg Config) (*Controller, error) {
	c := &Controller{
		mode:            cfg.Mode,
		workerRestarter: cfg.WorkerRestarter,
	}

	switch cfg.Mode {
	case Development:
		dm, err := devmode.New(devmode.Config{
			WorkersDir: cfg.WorkersDir,
			ServerDir:  cfg.ServerDir,
			ConfigDir:  cfg.ConfigDir,
			DebounceMs: cfg.DebounceMs,
			RestartHandler: func(workerName string) {
				if err := c.workerRestarter.RestartWorker(workerName); err != nil {
					log.Printf("Failed to restart worker %s: %v", workerName, err)
				}
			},
			ServerRestart: func() {
				log.Println("Server restart triggered by file change")
				// In dev mode, log but don't actually restart
				// (would require external process management)
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create dev mode: %w", err)
		}
		c.devMode = dm

	case Production:
		pm := prodmode.New(prodmode.Config{
			ServerBinPath: cfg.ServerBinPath,
			RestartHandler: func(workerName string, changeType string) {
				log.Printf("Prod mode change: worker=%s, type=%s", workerName, changeType)

				switch changeType {
				case "binary":
					// Full restart required
					if err := c.workerRestarter.RestartWorker(workerName); err != nil {
						log.Printf("Failed to restart worker %s: %v", workerName, err)
					}
				case "assets":
					// Asset-only change - can reload without full restart
					// For now, still do full restart
					if err := c.workerRestarter.RestartWorker(workerName); err != nil {
						log.Printf("Failed to restart worker %s: %v", workerName, err)
					}
				case "both":
					// Both changed - full restart
					if err := c.workerRestarter.RestartWorker(workerName); err != nil {
						log.Printf("Failed to restart worker %s: %v", workerName, err)
					}
				}
			},
		})
		c.prodMode = pm

	default:
		return nil, fmt.Errorf("unknown mode: %s", cfg.Mode)
	}

	return c, nil
}

// Start starts the mode controller.
func (c *Controller) Start() error {
	switch c.mode {
	case Development:
		log.Println("Starting in DEVELOPMENT mode")
		return c.devMode.Start()
	case Production:
		log.Println("Starting in PRODUCTION mode")
		return c.prodMode.Start()
	default:
		return fmt.Errorf("unknown mode: %s", c.mode)
	}
}

// Stop stops the mode controller.
func (c *Controller) Stop() {
	switch c.mode {
	case Development:
		if c.devMode != nil {
			c.devMode.Stop()
		}
	case Production:
		if c.prodMode != nil {
			c.prodMode.Stop()
		}
	}
}

// BuildAll builds all workers and server (dev mode only).
func (c *Controller) BuildAll() error {
	if c.mode == Development && c.devMode != nil {
		return c.devMode.BuildAll()
	}
	return nil
}

// RegisterWorker registers a worker for tracking (prod mode only).
func (c *Controller) RegisterWorker(worker *supervisor.WorkerInstance) {
	if c.mode == Production && c.prodMode != nil {
		c.prodMode.RegisterWorker(worker)
	}
}

// UnregisterWorker removes a worker from tracking (prod mode only).
func (c *Controller) UnregisterWorker(workerName string) {
	if c.mode == Production && c.prodMode != nil {
		c.prodMode.UnregisterWorker(workerName)
	}
}

// GetMode returns the current deployment mode.
func (c *Controller) GetMode() Mode {
	return c.mode
}

// GetModeFromEnv reads the deployment mode from environment.
func GetModeFromEnv() Mode {
	mode := os.Getenv("TQ_MODE")
	if mode == "" {
		mode = os.Getenv("DEPLOYMENT_MODE")
	}
	if mode == "" {
		mode = "dev" // Default to dev
	}

	switch mode {
	case "prod", "production":
		return Production
	default:
		return Development
	}
}

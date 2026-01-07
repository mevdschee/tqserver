package php

import (
	"fmt"
	"time"
)

// Config represents PHP worker configuration
type Config struct {
	// Binary path to php-cgi executable
	Binary string

	// ConfigFile is the optional base php.ini file path
	ConfigFile string

	// Settings are individual PHP configuration directives
	// These override values from ConfigFile
	Settings map[string]string

	// DocumentRoot is the document root directory
	DocumentRoot string

	// Pool configuration
	Pool PoolConfig
}

// PoolConfig represents process pool configuration
type PoolConfig struct {
	// Manager type: "static", "dynamic", or "ondemand"
	Manager string

	// MinWorkers is the minimum number of workers (dynamic/ondemand)
	MinWorkers int

	// MaxWorkers is the maximum number of workers (dynamic/static)
	MaxWorkers int

	// StartWorkers is the initial number of workers (dynamic)
	StartWorkers int

	// MaxRequests is the maximum requests per worker before restart (0 = unlimited)
	MaxRequests int

	// RequestTimeout is the maximum time for a single request
	RequestTimeout time.Duration

	// IdleTimeout is the time before an idle worker is killed (ondemand)
	IdleTimeout time.Duration

	// ListenAddr is the FastCGI listen address (e.g., "127.0.0.1:9000")
	ListenAddr string

	// UnixSocket is the optional Unix socket path
	UnixSocket string
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Binary == "" {
		return fmt.Errorf("php binary path is required")
	}

	if c.DocumentRoot == "" {
		return fmt.Errorf("document root is required")
	}

	// Validate pool configuration
	if err := c.Pool.Validate(); err != nil {
		return fmt.Errorf("pool config: %w", err)
	}

	return nil
}

// Validate checks if the pool configuration is valid
func (p *PoolConfig) Validate() error {
	switch p.Manager {
	case "static", "dynamic", "ondemand":
		// Valid
	default:
		return fmt.Errorf("invalid pool manager: %s (must be static, dynamic, or ondemand)", p.Manager)
	}

	if p.Manager == "static" && p.MaxWorkers < 1 {
		return fmt.Errorf("static pool requires max_workers >= 1")
	}

	if p.Manager == "dynamic" {
		if p.MinWorkers < 1 {
			return fmt.Errorf("dynamic pool requires min_workers >= 1")
		}
		if p.MaxWorkers < p.MinWorkers {
			return fmt.Errorf("max_workers must be >= min_workers")
		}
		if p.StartWorkers < p.MinWorkers || p.StartWorkers > p.MaxWorkers {
			return fmt.Errorf("start_workers must be between min_workers and max_workers")
		}
	}

	if p.Manager == "ondemand" && p.MaxWorkers < 1 {
		return fmt.Errorf("ondemand pool requires max_workers >= 1")
	}

	if p.RequestTimeout <= 0 {
		p.RequestTimeout = 30 * time.Second
	}

	if p.IdleTimeout <= 0 {
		p.IdleTimeout = 10 * time.Second
	}

	return nil
}

// GetWorkerCount returns the number of workers to start initially
func (p *PoolConfig) GetWorkerCount() int {
	switch p.Manager {
	case "static":
		return p.MaxWorkers
	case "dynamic":
		return p.StartWorkers
	case "ondemand":
		return 0 // Start with no workers
	default:
		return 1
	}
}

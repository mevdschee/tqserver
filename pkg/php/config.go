package php

import (
	"fmt"
	"time"
)

// Config represents PHP / php-fpm configuration used by the runtime.
// This refactor uses a php-fpm-first configuration model (no backwards compatibility).
type Config struct {
	// Path to the php-fpm binary to execute in dev/supervised mode.
	PHPFPMBinary string

	// Optional base php.ini file to pass to php-fpm (via -c or env as needed).
	PHPIni string

	// Global PHP-FPM options and pool configuration.
	PHPFPM PHPFPMConfig

	// DocumentRoot is the document root directory used for chdir in pool config.
	DocumentRoot string

	// Settings are individual PHP configuration directives injected as env entries
	// into the generated php-fpm pool (e.g. PHP_VALUE, env[] entries).
	Settings map[string]string
}

// PHPFPMConfig controls how php-fpm is launched and configured.
type PHPFPMConfig struct {
	// Enabled toggles using php-fpm. When false, no php-fpm is launched.
	Enabled bool

	// Listen is the address php-fpm should listen on (TCP host:port or unix socket path).
	Listen string

	// Transport indicates the listen transport: "tcp" or "unix".
	Transport string

	// Pool defines the pool-specific settings used to generate a php-fpm pool file.
	Pool PoolConfig

	// Env are extra environment variables injected into the php-fpm process.
	Env map[string]string

	// GeneratedConfigDir is the directory where generated php-fpm configs will be written.
	// If empty, a secure temp directory will be used.
	GeneratedConfigDir string

	// NoDaemonize indicates the runtime should start php-fpm with -F (no-daemonize).
	NoDaemonize bool
}

// PoolConfig represents an explicit php-fpm pool configuration that maps cleanly
// to php-fpm directives.
type PoolConfig struct {
	// Name is the pool name used for the pool file (e.g. "tqserver").
	Name string

	// PM is the process manager: "static", "dynamic" or "ondemand".
	PM string

	// MaxChildren maps to pm.max_children
	MaxChildren int

	// StartServers maps to pm.start_servers (dynamic)
	StartServers int

	// MinSpareServers maps to pm.min_spare_servers (dynamic)
	MinSpareServers int

	// MaxSpareServers maps to pm.max_spare_servers (dynamic)
	MaxSpareServers int

	// MaxRequests maps to pm.max_requests
	MaxRequests int

	// RequestTerminateTimeout maps to request_terminate_timeout (e.g. "30s").
	RequestTerminateTimeout time.Duration

	// ProcessIdleTimeout maps to process_idle_timeout (ondemand) (e.g. "10s").
	ProcessIdleTimeout time.Duration
}

// Validate checks if the configuration is valid for php-fpm generation and launch.
func (c *Config) Validate() error {
	if !c.PHPFPM.Enabled {
		return nil
	}
	if c.PHPFPM.Pool.Name == "" {
		return fmt.Errorf("phpfpm pool name is required")
	}
	if c.PHPFPM.Listen == "" {
		return fmt.Errorf("phpfpm listen address is required")
	}
	if c.DocumentRoot == "" {
		return fmt.Errorf("document root is required")
	}
	if err := c.PHPFPM.Pool.Validate(); err != nil {
		return fmt.Errorf("pool validation: %w", err)
	}
	return nil
}

// Validate checks pool constraints and fills sane defaults where applicable.
func (p *PoolConfig) Validate() error {
	switch p.PM {
	case "static", "dynamic", "ondemand":
		// ok
	default:
		p.PM = "dynamic"
	}

	if p.MaxChildren <= 0 {
		p.MaxChildren = 5
	}

	if p.PM == "dynamic" {
		if p.StartServers <= 0 {
			p.StartServers = 2
		}
		if p.MinSpareServers <= 0 {
			p.MinSpareServers = 1
		}
		if p.MaxSpareServers <= 0 {
			p.MaxSpareServers = p.MaxChildren
		}
	}

	if p.MaxRequests < 0 {
		p.MaxRequests = 0
	}

	if p.RequestTerminateTimeout <= 0 {
		p.RequestTerminateTimeout = 30 * time.Second
	}

	if p.ProcessIdleTimeout <= 0 {
		p.ProcessIdleTimeout = 10 * time.Second
	}

	return nil
}

// GetInitialWorkerCount returns the number of workers the pool will start with.
func (p *PoolConfig) GetInitialWorkerCount() int {
	switch p.PM {
	case "static":
		return p.MaxChildren
	case "dynamic":
		return p.StartServers
	case "ondemand":
		return 0
	default:
		return p.MaxChildren
	}
}

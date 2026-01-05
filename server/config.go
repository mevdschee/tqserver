package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the server configuration
type Config struct {
	Server struct {
		Port                int `yaml:"port"`
		ReadTimeoutSeconds  int `yaml:"read_timeout_seconds"`
		WriteTimeoutSeconds int `yaml:"write_timeout_seconds"`
		IdleTimeoutSeconds  int `yaml:"idle_timeout_seconds"`
	} `yaml:"server"`

	Workers struct {
		PortRangeStart        int `yaml:"port_range_start"`
		PortRangeEnd          int `yaml:"port_range_end"`
		StartupDelayMs        int `yaml:"startup_delay_ms"`
		RestartDelayMs        int `yaml:"restart_delay_ms"`
		ShutdownGracePeriodMs int `yaml:"shutdown_grace_period_ms"`
	} `yaml:"workers"`

	FileWatcher struct {
		DebounceMs int `yaml:"debounce_ms"`
	} `yaml:"file_watcher"`

	Pages struct {
		Directory string `yaml:"directory"`
	} `yaml:"pages"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(configPath string) (*Config, error) {
	// Set defaults
	config := &Config{}
	config.Server.Port = 8080
	config.Server.ReadTimeoutSeconds = 30
	config.Server.WriteTimeoutSeconds = 30
	config.Server.IdleTimeoutSeconds = 120
	config.Workers.PortRangeStart = 9000
	config.Workers.PortRangeEnd = 9999
	config.Workers.StartupDelayMs = 100
	config.Workers.RestartDelayMs = 100
	config.Workers.ShutdownGracePeriodMs = 500
	config.FileWatcher.DebounceMs = 50
	config.Pages.Directory = "pages"

	// If config file exists, load it
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	return config, nil
}

// GetReadTimeout returns the read timeout as a time.Duration
func (c *Config) GetReadTimeout() time.Duration {
	return time.Duration(c.Server.ReadTimeoutSeconds) * time.Second
}

// GetWriteTimeout returns the write timeout as a time.Duration
func (c *Config) GetWriteTimeout() time.Duration {
	return time.Duration(c.Server.WriteTimeoutSeconds) * time.Second
}

// GetIdleTimeout returns the idle timeout as a time.Duration
func (c *Config) GetIdleTimeout() time.Duration {
	return time.Duration(c.Server.IdleTimeoutSeconds) * time.Second
}

// GetStartupDelay returns the startup delay as a time.Duration
func (c *Config) GetStartupDelay() time.Duration {
	return time.Duration(c.Workers.StartupDelayMs) * time.Millisecond
}

// GetRestartDelay returns the restart delay as a time.Duration
func (c *Config) GetRestartDelay() time.Duration {
	return time.Duration(c.Workers.RestartDelayMs) * time.Millisecond
}

// GetShutdownGracePeriod returns the shutdown grace period as a time.Duration
func (c *Config) GetShutdownGracePeriod() time.Duration {
	return time.Duration(c.Workers.ShutdownGracePeriodMs) * time.Millisecond
}

// GetDebounceDelay returns the debounce delay as a time.Duration
func (c *Config) GetDebounceDelay() time.Duration {
	return time.Duration(c.FileWatcher.DebounceMs) * time.Millisecond
}

// GetPagesPath returns the absolute path to the pages directory
func (c *Config) GetPagesPath(projectRoot string) string {
	if filepath.IsAbs(c.Pages.Directory) {
		return c.Pages.Directory
	}
	return filepath.Join(projectRoot, c.Pages.Directory)
}

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// WorkerSettings represents per-worker configuration
type WorkerSettings struct {
	GoMaxProcs          int    `yaml:"go_max_procs"`
	MaxRequests         int    `yaml:"max_requests"`
	ReadTimeoutSeconds  int    `yaml:"read_timeout_seconds"`
	WriteTimeoutSeconds int    `yaml:"write_timeout_seconds"`
	IdleTimeoutSeconds  int    `yaml:"idle_timeout_seconds"`
	GoMemLimit          string `yaml:"go_mem_limit"`
	LogFile             string `yaml:"log_file"`
}

// Config represents the server configuration
type Config struct {
	Server struct {
		Port                int    `yaml:"port"`
		ReadTimeoutSeconds  int    `yaml:"read_timeout_seconds"`
		WriteTimeoutSeconds int    `yaml:"write_timeout_seconds"`
		IdleTimeoutSeconds  int    `yaml:"idle_timeout_seconds"`
		LogFile             string `yaml:"log_file"`
	} `yaml:"server"`

	Workers struct {
		BinDir                string                    `yaml:"bin_dir"`
		PortRangeStart        int                       `yaml:"port_range_start"`
		PortRangeEnd          int                       `yaml:"port_range_end"`
		StartupDelayMs        int                       `yaml:"startup_delay_ms"`
		RestartDelayMs        int                       `yaml:"restart_delay_ms"`
		ShutdownGracePeriodMs int                       `yaml:"shutdown_grace_period_ms"`
		Default               WorkerSettings            `yaml:"default"`
		Paths                 map[string]WorkerSettings `yaml:"paths"`
	} `yaml:"workers"`

	FileWatcher struct {
		DebounceMs int `yaml:"debounce_ms"`
	} `yaml:"file_watcher"`

	Pages struct {
		Directory string `yaml:"directory"`
	} `yaml:"pages"`
}

// setDefaults sets default values for the configuration
func setDefaults(config *Config) {
	config.Server.Port = 8080
	config.Server.ReadTimeoutSeconds = 30
	config.Server.WriteTimeoutSeconds = 30
	config.Server.IdleTimeoutSeconds = 120
	config.Server.LogFile = "logs/server_{date}.log"
	config.Workers.BinDir = "bin"
	config.Workers.PortRangeStart = 9000
	config.Workers.PortRangeEnd = 9999
	config.Workers.StartupDelayMs = 100
	config.Workers.RestartDelayMs = 100
	config.Workers.ShutdownGracePeriodMs = 500
	config.Workers.Default.GoMaxProcs = 1
	config.Workers.Default.MaxRequests = 0 // 0 = unlimited
	config.Workers.Default.ReadTimeoutSeconds = 30
	config.Workers.Default.WriteTimeoutSeconds = 30
	config.Workers.Default.IdleTimeoutSeconds = 120
	config.Workers.Default.GoMemLimit = "" // empty = unlimited
	config.Workers.Default.LogFile = "logs/{path}/worker_{date}.log"
	config.Workers.Paths = make(map[string]WorkerSettings)
	config.FileWatcher.DebounceMs = 50
	config.Pages.Directory = "pages"
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(configPath string) (*Config, error) {
	// Set defaults
	config := &Config{}
	setDefaults(config)

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

// Reload reloads the configuration from the same file path
func (c *Config) Reload(configPath string) error {
	// Load new config
	newConfig, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}

	// Copy all values from newConfig to c
	*c = *newConfig
	return nil
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

// GetWorkerIdleTimeout returns the worker idle timeout as a time.Duration
func (c *Config) GetWorkerIdleTimeout() time.Duration {
	return time.Duration(c.Workers.Default.IdleTimeoutSeconds) * time.Second
}

// GetReadTimeout returns the read timeout for worker settings
func (ws *WorkerSettings) GetReadTimeout() int {
	if ws.ReadTimeoutSeconds > 0 {
		return ws.ReadTimeoutSeconds
	}
	return 30 // Default
}

// GetWriteTimeout returns the write timeout for worker settings
func (ws *WorkerSettings) GetWriteTimeout() int {
	if ws.WriteTimeoutSeconds > 0 {
		return ws.WriteTimeoutSeconds
	}
	return 30 // Default
}

// GetWorkerSettings returns the worker settings for a given path.
// It checks for exact matches first, then prefix matches, and falls back to default settings.
func (c *Config) GetWorkerSettings(path string) WorkerSettings {
	// Check for exact match
	if settings, ok := c.Workers.Paths[path]; ok {
		return settings
	}

	// Check for prefix matches (e.g., /api matches /api/users)
	bestMatch := ""
	for configPath := range c.Workers.Paths {
		if strings.HasPrefix(path, configPath) && len(configPath) > len(bestMatch) {
			bestMatch = configPath
		}
	}

	if bestMatch != "" {
		return c.Workers.Paths[bestMatch]
	}

	// Fall back to default settings
	return c.Workers.Default
}

// GetPagesPath returns the absolute path to the pages directory
func (c *Config) GetPagesPath(projectRoot string) string {
	if filepath.IsAbs(c.Pages.Directory) {
		return c.Pages.Directory
	}
	return filepath.Join(projectRoot, c.Pages.Directory)
}

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// WorkerConfig represents a worker's configuration from worker.yaml
type WorkerConfig struct {
	Path    string `yaml:"path"`
	Runtime struct {
		GOMAXPROCS  int    `yaml:"go_max_procs"`
		GOMEMLIMIT  string `yaml:"go_mem_limit"`
		MaxRequests int    `yaml:"max_requests"`
	} `yaml:"runtime"`
	Timeouts struct {
		ReadTimeoutSeconds  int `yaml:"read_timeout_seconds"`
		WriteTimeoutSeconds int `yaml:"write_timeout_seconds"`
		IdleTimeoutSeconds  int `yaml:"idle_timeout_seconds"`
	} `yaml:"timeouts"`
	Logging struct {
		LogFile string `yaml:"log_file"`
	} `yaml:"logging"`
}

// WorkerConfigWithMeta includes config and metadata
type WorkerConfigWithMeta struct {
	Name       string
	ConfigPath string
	Config     WorkerConfig
	ModTime    time.Time
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
		Directory             string `yaml:"directory"`
		PortRangeStart        int    `yaml:"port_range_start"`
		PortRangeEnd          int    `yaml:"port_range_end"`
		StartupDelayMs        int    `yaml:"startup_delay_ms"`
		RestartDelayMs        int    `yaml:"restart_delay_ms"`
		ShutdownGracePeriodMs int    `yaml:"shutdown_grace_period_ms"`
	} `yaml:"workers"`

	FileWatcher struct {
		DebounceMs int `yaml:"debounce_ms"`
	} `yaml:"file_watcher"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(configPath string) (*Config, error) {
	// Set defaults
	config := &Config{}
	config.Server.Port = 8080
	config.Server.ReadTimeoutSeconds = 30
	config.Server.WriteTimeoutSeconds = 30
	config.Server.IdleTimeoutSeconds = 120
	config.Server.LogFile = "logs/tqserver_{date}.log"
	config.Workers.Directory = "workers"
	config.Workers.PortRangeStart = 9000
	config.Workers.PortRangeEnd = 9999
	config.Workers.StartupDelayMs = 100
	config.Workers.RestartDelayMs = 100
	config.Workers.ShutdownGracePeriodMs = 500
	config.FileWatcher.DebounceMs = 50

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

// LoadWorkerConfigs scans the workers directory and loads all worker configs
func LoadWorkerConfigs(workersDir string) ([]*WorkerConfigWithMeta, error) {
	var configs []*WorkerConfigWithMeta

	entries, err := os.ReadDir(workersDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read workers directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		workerName := entry.Name()
		configPath := filepath.Join(workersDir, workerName, "config", "worker.yaml")

		// Check if worker.yaml exists
		stat, err := os.Stat(configPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("Warning: Worker '%s' has no config/worker.yaml, skipping", workerName)
				continue
			}
			return nil, fmt.Errorf("failed to stat config for worker '%s': %w", workerName, err)
		}

		// Load worker config
		workerConfig, err := LoadWorkerConfig(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config for worker '%s': %w", workerName, err)
		}

		// Validate path is set
		if workerConfig.Path == "" {
			return nil, fmt.Errorf("worker '%s' has no path configured", workerName)
		}

		configs = append(configs, &WorkerConfigWithMeta{
			Name:       workerName,
			ConfigPath: configPath,
			Config:     *workerConfig,
			ModTime:    stat.ModTime(),
		})

		log.Printf("Loaded worker '%s' at path '%s'", workerName, workerConfig.Path)
	}

	return configs, nil
}

// loadWorkerConfig loads a single worker config file
func LoadWorkerConfig(configPath string) (*WorkerConfig, error) {
	// Set defaults
	config := &WorkerConfig{}
	config.Runtime.GOMAXPROCS = 2
	config.Runtime.GOMEMLIMIT = ""
	config.Runtime.MaxRequests = 0
	config.Timeouts.ReadTimeoutSeconds = 30
	config.Timeouts.WriteTimeoutSeconds = 30
	config.Timeouts.IdleTimeoutSeconds = 120
	config.Logging.LogFile = "logs/worker_{name}_{date}.log"

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// CheckWorkerConfigChanges checks if any worker configs have been modified
func CheckWorkerConfigChanges(configs []*WorkerConfigWithMeta) ([]string, error) {
	var changed []string

	for _, meta := range configs {
		stat, err := os.Stat(meta.ConfigPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("Warning: Config file removed for worker '%s'", meta.Name)
				continue
			}
			return nil, fmt.Errorf("failed to stat config for worker '%s': %w", meta.Name, err)
		}

		if stat.ModTime().After(meta.ModTime) {
			changed = append(changed, meta.Name)
			meta.ModTime = stat.ModTime()

			// Reload the config
			newConfig, err := LoadWorkerConfig(meta.ConfigPath)
			if err != nil {
				log.Printf("Error reloading config for worker '%s': %v", meta.Name, err)
				continue
			}
			meta.Config = *newConfig
			log.Printf("Reloaded config for worker '%s'", meta.Name)
		}
	}

	return changed, nil
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

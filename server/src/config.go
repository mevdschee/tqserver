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
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`     // "go", "bun" or "php"
	Enabled string `yaml:"enabled"`  // "true", "false", or "development"
	LogFile string `yaml:"log_file"` // Deprecated: use Logging.LogFile

	Logging struct {
		LogFile string `yaml:"log_file"`
	} `yaml:"logging"`

	// Go runtime configuration
	Go *struct {
		GOMAXPROCS          int    `yaml:"go_max_procs"`
		GOMEMLIMIT          string `yaml:"go_mem_limit"`
		ReadTimeoutSeconds  int    `yaml:"read_timeout_seconds"`
		WriteTimeoutSeconds int    `yaml:"write_timeout_seconds"`
		IdleTimeoutSeconds  int    `yaml:"idle_timeout_seconds"`
		MaxRequests         int    `yaml:"max_requests"`
	} `yaml:"go"`
	// Bun runtime configuration
	Bun *struct {
		Entrypoint string            `yaml:"entrypoint"` // Main file (e.g., "index.ts")
		Env        map[string]string `yaml:"env"`
	} `yaml:"bun"`

	// Scaling configuration (for Go and Bun workers)
	Scaling *struct {
		MinWorkers     int `yaml:"min_workers"`      // Minimum operational workers
		MaxWorkers     int `yaml:"max_workers"`      // Maximum operational workers
		QueueThreshold int `yaml:"queue_threshold"`  // Queue depth to trigger scale up
		ScaleDownDelay int `yaml:"scale_down_delay"` // Seconds idle before scaling down
	} `yaml:"scaling"`

	// PHP-specific configuration
	PHP *struct {
		Binary     string            `yaml:"binary"`
		ConfigFile string            `yaml:"config_file"`
		Settings   map[string]string `yaml:"settings"`
		Pool       struct {
			Manager        string `yaml:"manager"`
			MinWorkers     int    `yaml:"min_workers"`
			MaxWorkers     int    `yaml:"max_workers"`
			StartWorkers   int    `yaml:"start_workers"`
			MaxRequests    int    `yaml:"max_requests"`
			RequestTimeout int    `yaml:"request_timeout"`
			IdleTimeout    int    `yaml:"idle_timeout"`
			ListenAddress  string `yaml:"listen_address"`
		} `yaml:"pool"`
	} `yaml:"php"`
}

// IsEnabled returns true if the worker is enabled based on the server mode.
// Possible values for Enabled are: "true", "false", "development".
// - "true" or "" (empty): always enabled
// - "false": always disabled
// - "development": only enabled when server is in dev mode
func (wc *WorkerConfig) IsEnabled(serverMode string) bool {
	switch wc.Enabled {
	case "false":
		return false
	case "development":
		return serverMode == "dev" || serverMode == "development"
	default:
		// "true" or empty string means enabled
		return true
	}
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
	Mode string // "dev" or "prod" - not from YAML, set via flag or env

	Server struct {
		Port                int    `yaml:"port"`
		ReadTimeoutSeconds  int    `yaml:"read_timeout_seconds"`
		WriteTimeoutSeconds int    `yaml:"write_timeout_seconds"`
		IdleTimeoutSeconds  int    `yaml:"idle_timeout_seconds"`
		LogFile             string `yaml:"log_file"`
	} `yaml:"server"`

	Workers struct {
		Directory                string `yaml:"directory"`
		PortRangeStart           int    `yaml:"port_range_start"`
		PortRangeEnd             int    `yaml:"port_range_end"`
		StartupDelayMs           int    `yaml:"startup_delay_ms"`
		RestartDelayMs           int    `yaml:"restart_delay_ms"`
		ShutdownGracePeriodMs    int    `yaml:"shutdown_grace_period_ms"`
		HealthCheckWaitTimeoutMs int    `yaml:"health_check_wait_timeout_ms"`
		HealthCheckTimeoutMs     int    `yaml:"health_check_timeout_ms"`
	} `yaml:"workers"`

	FileWatcher struct {
		DebounceMs int `yaml:"debounce_ms"`
	} `yaml:"file_watcher"`

	Socks5 Socks5Config `yaml:"socks5"`

	Metrics struct {
		Enabled bool   `yaml:"enabled"` // Default: true
		Path    string `yaml:"path"`    // Default: "/metrics"
	} `yaml:"metrics"`
}

// Socks5Config represents the SOCKS5 proxy configuration
type Socks5Config struct {
	Enabled         bool                   `yaml:"enabled"`
	Port            int                    `yaml:"port"`
	LogFile         string                 `yaml:"log_file"`
	LogFormat       string                 `yaml:"log_format"` // "json" | "text"
	HTTPSInspection *HTTPSInspectionConfig `yaml:"https_inspection"`
}

// HTTPSInspectionConfig represents HTTPS MITM inspection settings
type HTTPSInspectionConfig struct {
	Enabled      bool   `yaml:"enabled"`
	CACert       string `yaml:"ca_cert"`
	CAKey        string `yaml:"ca_key"`
	AutoGenerate bool   `yaml:"auto_generate"`
	LogBody      bool   `yaml:"log_body"`
	MaxBodySize  int    `yaml:"max_body_size"`
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
	config.Workers.ShutdownGracePeriodMs = 5000    // Default 5s
	config.Workers.HealthCheckWaitTimeoutMs = 5000 // Default 5s
	config.Workers.HealthCheckTimeoutMs = 250      // Default 250ms
	config.FileWatcher.DebounceMs = 50

	// SOCKS5 proxy defaults
	config.Socks5.Enabled = false
	config.Socks5.Port = 1080
	config.Socks5.LogFile = "logs/socks5_{date}.log"
	config.Socks5.LogFormat = "json"

	// Metrics defaults
	config.Metrics.Enabled = true
	config.Metrics.Path = "/metrics"

	// Set mode from environment variable (defaults to "dev")
	config.Mode = os.Getenv("TQSERVER_MODE")
	if config.Mode == "" {
		config.Mode = "dev"
	}

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
	// Set defaults and pre-initialize nested structs so unmarshalling
	// only overrides supplied fields (avoids nil deref when `go` section
	// is omitted from worker.yaml).
	config := &WorkerConfig{}
	config.LogFile = "logs/worker_{name}_{date}.log"

	// Pre-create the Go runtime config with sensible defaults.
	config.Go = &struct {
		GOMAXPROCS          int    `yaml:"go_max_procs"`
		GOMEMLIMIT          string `yaml:"go_mem_limit"`
		ReadTimeoutSeconds  int    `yaml:"read_timeout_seconds"`
		WriteTimeoutSeconds int    `yaml:"write_timeout_seconds"`
		IdleTimeoutSeconds  int    `yaml:"idle_timeout_seconds"`
		MaxRequests         int    `yaml:"max_requests"`
	}{
		GOMAXPROCS:          2,
		GOMEMLIMIT:          "",
		ReadTimeoutSeconds:  30,
		WriteTimeoutSeconds: 30,
		IdleTimeoutSeconds:  120,
		MaxRequests:         0,
	}

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

// GetHealthCheckWaitTimeout returns the health check wait timeout as a time.Duration
func (c *Config) GetHealthCheckWaitTimeout() time.Duration {
	return time.Duration(c.Workers.HealthCheckWaitTimeoutMs) * time.Millisecond
}

// GetHealthCheckTimeout returns the health check request timeout as a time.Duration
func (c *Config) GetHealthCheckTimeout() time.Duration {
	return time.Duration(c.Workers.HealthCheckTimeoutMs) * time.Millisecond
}

// IsDevelopmentMode returns true if the server is running in development mode
func (c *Config) IsDevelopmentMode() bool {
	return c.Mode == "dev" || c.Mode == "development"
}

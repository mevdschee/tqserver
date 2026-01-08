package phpfpm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/mevdschee/tqserver/pkg/config/php"
)

// TestGenerateFromWorkerYAML verifies that generating php-fpm config using
// values taken directly from a worker's YAML (`workers/blog/config/worker.yaml`)
// produces pool config with the expected `php_admin_flag[...]` entries and pool directives.
func TestGenerateFromWorkerYAML(t *testing.T) {
	// read worker.yaml
	yamlPath := filepath.Join("..", "..", "workers", "blog", "config", "worker.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("read worker.yaml: %v", err)
	}

	// minimal structure to extract php section
	var raw struct {
		PHP struct {
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

	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal worker.yaml: %v", err)
	}

	tmp, err := os.MkdirTemp("", "tq-phpfpm-yaml-test-")
	if err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}
	defer os.RemoveAll(tmp)

	cfg := &php.Config{
		PHPFPMBinary: "",
		PHPIni:       raw.PHP.ConfigFile,
		DocumentRoot: tmp,
		Settings:     raw.PHP.Settings,
	}
	cfg.PHPFPM.Enabled = true
	cfg.PHPFPM.Listen = "127.0.0.1:9010"
	cfg.PHPFPM.Transport = "tcp"
	cfg.PHPFPM.GeneratedConfigDir = tmp
	cfg.PHPFPM.NoDaemonize = true
	cfg.PHPFPM.Pool = php.PoolConfig{
		Name:                    "tqtestyaml",
		PM:                      raw.PHP.Pool.Manager,
		MaxChildren:             raw.PHP.Pool.MaxWorkers,
		StartServers:            raw.PHP.Pool.StartWorkers,
		MinSpareServers:         raw.PHP.Pool.MinWorkers,
		MaxSpareServers:         raw.PHP.Pool.MaxWorkers,
		MaxRequests:             raw.PHP.Pool.MaxRequests,
		RequestTerminateTimeout: time.Duration(raw.PHP.Pool.RequestTimeout) * time.Second,
		ProcessIdleTimeout:      time.Duration(raw.PHP.Pool.IdleTimeout) * time.Second,
	}

	_, poolPath, err := GeneratePHPFPMConfig(cfg, tmp)
	if err != nil {
		t.Fatalf("GeneratePHPFPMConfig failed: %v", err)
	}

	poolData, err := os.ReadFile(poolPath)
	if err != nil {
		t.Fatalf("read pool conf: %v", err)
	}
	poolStr := string(poolData)

	// assert some settings from the YAML were rendered as php_admin_flag
	if !strings.Contains(poolStr, "php_admin_flag[max_execution_time] = 30") {
		t.Fatalf("pool config did not contain max_execution_time setting as php_admin_flag; got:\n%s", poolStr)
	}
	if !strings.Contains(poolStr, "pm = dynamic") {
		t.Fatalf("expected pm=dynamic in pool conf, got:\n%s", poolStr)
	}
	if !strings.Contains(poolStr, "pm.max_children = 10") {
		t.Fatalf("expected pm.max_children = 10 in pool conf, got:\n%s", poolStr)
	}
}

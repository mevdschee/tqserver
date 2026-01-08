package phpfpm

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mevdschee/tqserver/pkg/config/php"
)

// TestGenerateConfigAndLauncher verifies config generation and that the
// Launcher can start and stop a foreground php-fpm-like process.
func TestGenerateConfigAndLauncher(t *testing.T) {
	tmp, err := os.MkdirTemp("", "tq-phpfpm-test-")
	if err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}
	defer os.RemoveAll(tmp)

	// Prepare a minimal php.Config
	cfg := &php.Config{
		PHPFPMBinary: "",
		PHPIni:       "",
		DocumentRoot: tmp,
		Settings:     map[string]string{"APP_ENV": "test"},
	}
	cfg.PHPFPM.Enabled = true
	cfg.PHPFPM.Listen = "127.0.0.1:9001"
	cfg.PHPFPM.Transport = "tcp"
	cfg.PHPFPM.GeneratedConfigDir = tmp
	cfg.PHPFPM.NoDaemonize = true
	cfg.PHPFPM.Env = map[string]string{"TEST_ENV": "1"}
	cfg.PHPFPM.Pool = php.PoolConfig{
		Name:                    "tqtest",
		PM:                      "dynamic",
		MaxChildren:             2,
		StartServers:            1,
		MinSpareServers:         1,
		MaxSpareServers:         2,
		MaxRequests:             100,
		RequestTerminateTimeout: 5 * time.Second,
		ProcessIdleTimeout:      2 * time.Second,
	}

	// Generate configs
	main, err := GeneratePHPFPMConfig(cfg, tmp)
	if err != nil {
		t.Fatalf("GeneratePHPFPMConfig failed: %v", err)
	}
	if _, err := os.Stat(main); err != nil {
		t.Fatalf("main conf not written: %v", err)
	}

	// Create a tiny shim script that acts like a foreground php-fpm: prints and waits
	shim := filepath.Join(tmp, "php-fpm-shim.sh")
	script := `#!/bin/sh
echo "shim starting"
trap 'echo shim stopping; exit 0' INT TERM
while true; do
  echo "shim running"
  sleep 1
done
`
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatalf("write shim: %v", err)
	}

	cfg.PHPFPMBinary = shim

	launcher := NewLauncher(cfg)
	if err := launcher.Start(); err != nil {
		t.Fatalf("launcher start: %v", err)
	}

	// let it run briefly
	time.Sleep(500 * time.Millisecond)

	if err := launcher.Stop(2 * time.Second); err != nil {
		t.Fatalf("launcher stop: %v", err)
	}
}

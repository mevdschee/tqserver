package php

import (
	"testing"
)

func TestConfigValidation(t *testing.T) {
	// Minimal valid php-fpm config
	cfg := Config{
		PHPFPM: PHPFPMConfig{
			Enabled: true,
			Listen:  "127.0.0.1:9000",
			Pool: PoolConfig{
				Name:        "tq",
				PM:          "dynamic",
				MaxChildren: 3,
			},
		},
		DocumentRoot: "/var/www",
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}

	// Missing pool name should error
	bad := cfg
	bad.PHPFPM.Pool.Name = ""
	if err := bad.Validate(); err == nil {
		t.Fatalf("expected error for missing pool name")
	}

	// Missing listen should error
	bad = cfg
	bad.PHPFPM.Listen = ""
	if err := bad.Validate(); err == nil {
		t.Fatalf("expected error for missing listen")
	}

	// Missing document root should error
	bad = cfg
	bad.DocumentRoot = ""
	if err := bad.Validate(); err == nil {
		t.Fatalf("expected error for missing document root")
	}
}

func TestPoolConfigGetInitialWorkerCount(t *testing.T) {
	tests := []struct {
		name   string
		pool   PoolConfig
		expect int
	}{
		{
			name: "static pool",
			pool: PoolConfig{
				PM:          "static",
				MaxChildren: 5,
			},
			expect: 5,
		},
		{
			name: "dynamic pool",
			pool: PoolConfig{
				PM:           "dynamic",
				StartServers: 4,
			},
			expect: 4,
		},
		{
			name: "ondemand pool",
			pool: PoolConfig{
				PM:          "ondemand",
				MaxChildren: 5,
			},
			expect: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pool.GetInitialWorkerCount()
			if got != tt.expect {
				t.Errorf("GetInitialWorkerCount() = %d, want %d", got, tt.expect)
			}
		})
	}
}

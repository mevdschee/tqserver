package php

import (
	"testing"
	"time"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid static pool",
			config: Config{
				Binary:       "/usr/bin/php-cgi",
				DocumentRoot: "/var/www",
				Pool: PoolConfig{
					Manager:        "static",
					MaxWorkers:     4,
					RequestTimeout: 30 * time.Second,
					IdleTimeout:    10 * time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "valid dynamic pool",
			config: Config{
				Binary:       "/usr/bin/php-cgi",
				DocumentRoot: "/var/www",
				Pool: PoolConfig{
					Manager:        "dynamic",
					MinWorkers:     2,
					MaxWorkers:     10,
					StartWorkers:   5,
					RequestTimeout: 30 * time.Second,
					IdleTimeout:    10 * time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "missing binary",
			config: Config{
				DocumentRoot: "/var/www",
				Pool: PoolConfig{
					Manager:    "static",
					MaxWorkers: 4,
				},
			},
			wantErr: true,
		},
		{
			name: "missing document root",
			config: Config{
				Binary: "/usr/bin/php-cgi",
				Pool: PoolConfig{
					Manager:    "static",
					MaxWorkers: 4,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid pool manager",
			config: Config{
				Binary:       "/usr/bin/php-cgi",
				DocumentRoot: "/var/www",
				Pool: PoolConfig{
					Manager:    "invalid",
					MaxWorkers: 4,
				},
			},
			wantErr: true,
		},
		{
			name: "static pool with zero workers",
			config: Config{
				Binary:       "/usr/bin/php-cgi",
				DocumentRoot: "/var/www",
				Pool: PoolConfig{
					Manager:    "static",
					MaxWorkers: 0,
				},
			},
			wantErr: true,
		},
		{
			name: "dynamic pool with invalid worker counts",
			config: Config{
				Binary:       "/usr/bin/php-cgi",
				DocumentRoot: "/var/www",
				Pool: PoolConfig{
					Manager:      "dynamic",
					MinWorkers:   5,
					MaxWorkers:   2, // Less than min
					StartWorkers: 3,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPoolConfigGetWorkerCount(t *testing.T) {
	tests := []struct {
		name   string
		pool   PoolConfig
		expect int
	}{
		{
			name: "static pool",
			pool: PoolConfig{
				Manager:    "static",
				MaxWorkers: 5,
			},
			expect: 5,
		},
		{
			name: "dynamic pool",
			pool: PoolConfig{
				Manager:      "dynamic",
				MinWorkers:   2,
				MaxWorkers:   10,
				StartWorkers: 4,
			},
			expect: 4,
		},
		{
			name: "ondemand pool",
			pool: PoolConfig{
				Manager:    "ondemand",
				MaxWorkers: 5,
			},
			expect: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pool.GetWorkerCount()
			if got != tt.expect {
				t.Errorf("GetWorkerCount() = %d, want %d", got, tt.expect)
			}
		})
	}
}

func TestWorkerState(t *testing.T) {
	states := []WorkerState{
		WorkerStateIdle,
		WorkerStateActive,
		WorkerStateTerminating,
		WorkerStateCrashed,
	}

	expectedStrings := []string{
		"idle",
		"active",
		"terminating",
		"crashed",
	}

	for i, state := range states {
		if state.String() != expectedStrings[i] {
			t.Errorf("WorkerState.String() = %s, want %s", state.String(), expectedStrings[i])
		}
	}
}

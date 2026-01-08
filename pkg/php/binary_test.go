package php

import (
	"os/exec"
	"testing"
)

func TestDetectBinary(t *testing.T) {
	// Try to detect php-cgi in PATH
	binary, err := DetectBinary("")

	// Skip test if php-cgi is not installed
	if err != nil {
		t.Skipf("php-cgi not found in PATH: %v", err)
	}

	// Verify we got a valid binary
	if binary.Path == "" {
		t.Error("Binary path is empty")
	}

	if binary.Version == "" {
		t.Error("Version is empty")
	}

	if binary.Major < 5 {
		t.Errorf("Unexpected PHP major version: %d", binary.Major)
	}

	t.Logf("Detected PHP: %s", binary.String())
}

func TestBinaryBuildArgs(t *testing.T) {
	binary := &Binary{
		Path:  "/usr/bin/php-cgi",
		Major: 8,
		Minor: 2,
		Patch: 0,
	}

	config := &Config{
		PHPIni: "/etc/php/8.2/php.ini",
		Settings: map[string]string{
			"memory_limit":       "128M",
			"max_execution_time": "30",
			"display_errors":     "1",
		},
	}

	args := binary.BuildArgs(config, "127.0.0.1:9000")

	// Verify arguments
	expectedArgs := []string{
		"-c", "/etc/php/8.2/php.ini",
		"-b", "127.0.0.1:9000",
	}

	// Check required args
	if len(args) < len(expectedArgs) {
		t.Errorf("Expected at least %d args, got %d", len(expectedArgs), len(args))
	}

	for i, expected := range expectedArgs {
		if i >= len(args) || args[i] != expected {
			t.Errorf("Arg %d: expected %s, got %s", i, expected, args[i])
		}
	}

	// Verify -d flags are present
	foundSettings := 0
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-d" {
			foundSettings++
		}
	}

	if foundSettings != len(config.Settings) {
		t.Errorf("Expected %d -d flags, found %d", len(config.Settings), foundSettings)
	}
}

func TestBinarySupportsFeature(t *testing.T) {
	tests := []struct {
		name    string
		binary  Binary
		feature string
		expect  bool
	}{
		{
			name:    "PHP 7.4 opcache",
			binary:  Binary{Major: 7, Minor: 4, Patch: 0},
			feature: "opcache",
			expect:  true,
		},
		{
			name:    "PHP 5.4 opcache",
			binary:  Binary{Major: 5, Minor: 4, Patch: 0},
			feature: "opcache",
			expect:  false,
		},
		{
			name:    "PHP 8.0 jit",
			binary:  Binary{Major: 8, Minor: 0, Patch: 0},
			feature: "jit",
			expect:  true,
		},
		{
			name:    "PHP 7.4 jit",
			binary:  Binary{Major: 7, Minor: 4, Patch: 0},
			feature: "jit",
			expect:  false,
		},
		{
			name:    "PHP 8.1 fiber",
			binary:  Binary{Major: 8, Minor: 1, Patch: 0},
			feature: "fiber",
			expect:  true,
		},
		{
			name:    "PHP 8.0 fiber",
			binary:  Binary{Major: 8, Minor: 0, Patch: 0},
			feature: "fiber",
			expect:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.binary.SupportsFeature(tt.feature)
			if got != tt.expect {
				t.Errorf("SupportsFeature(%s) = %v, want %v", tt.feature, got, tt.expect)
			}
		})
	}
}

func TestDetectInvalidBinary(t *testing.T) {
	_, err := DetectBinary("/nonexistent/php-cgi")
	if err == nil {
		t.Error("Expected error for nonexistent binary")
	}
}

func TestBinaryVersion(t *testing.T) {
	// Skip if php-cgi is not available
	if _, err := exec.LookPath("php-cgi"); err != nil {
		t.Skip("php-cgi not available")
	}

	binary, err := DetectBinary("")
	if err != nil {
		t.Skipf("Could not detect php-cgi: %v", err)
	}

	// Test version detection
	if binary.Major == 0 {
		t.Error("Major version is 0")
	}

	// Version string should contain "PHP"
	if len(binary.Version) < 3 {
		t.Errorf("Invalid version string: %s", binary.Version)
	}

	t.Logf("PHP Version: %d.%d.%d", binary.Major, binary.Minor, binary.Patch)
}

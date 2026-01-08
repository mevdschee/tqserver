package php

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// Binary represents a php-cgi binary
type Binary struct {
	Path    string
	Version string
	Major   int
	Minor   int
	Patch   int
}

// DetectBinary finds and validates the php-cgi binary
func DetectBinary(path string) (*Binary, error) {
	// If no path specified, search in PATH
	if path == "" {
		var err error
		path, err = exec.LookPath("php-cgi")
		if err != nil {
			return nil, fmt.Errorf("php-cgi not found in PATH: %w", err)
		}
	}

	// Verify binary exists and is executable
	if _, err := exec.LookPath(path); err != nil {
		return nil, fmt.Errorf("php-cgi binary not found at %s: %w", path, err)
	}

	binary := &Binary{
		Path: path,
	}

	// Get version information
	if err := binary.detectVersion(); err != nil {
		return nil, err
	}

	return binary, nil
}

// detectVersion extracts the PHP version from php-cgi -v
func (b *Binary) detectVersion() error {
	cmd := exec.Command(b.Path, "-v")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get php-cgi version: %w", err)
	}

	// Parse version from output
	// Example: "PHP 8.2.15 (cgi-fcgi) (built: Jan  1 2024 12:00:00)"
	versionRegex := regexp.MustCompile(`PHP (\d+)\.(\d+)\.(\d+)`)
	matches := versionRegex.FindStringSubmatch(string(output))
	if len(matches) != 4 {
		return fmt.Errorf("failed to parse PHP version from: %s", string(output))
	}

	b.Version = matches[0]
	fmt.Sscanf(matches[1], "%d", &b.Major)
	fmt.Sscanf(matches[2], "%d", &b.Minor)
	fmt.Sscanf(matches[3], "%d", &b.Patch)

	return nil
}

// String returns a human-readable version string
func (b *Binary) String() string {
	return fmt.Sprintf("%s (PHP %d.%d.%d)", b.Path, b.Major, b.Minor, b.Patch)
}

// BuildArgs constructs the command-line arguments for php-cgi in standard CGI mode
func (b *Binary) BuildArgs(config *Config) []string {
	args := []string{}

	// Add base config file if specified
	if config.ConfigFile != "" {
		args = append(args, "-c", config.ConfigFile)
	}

	// Add individual settings as -d flags
	for key, value := range config.Settings {
		// Escape special characters if needed
		setting := fmt.Sprintf("%s=%s", key, value)
		args = append(args, "-d", setting)
	}

	return args
}

// SupportsFeature checks if the PHP version supports a specific feature
func (b *Binary) SupportsFeature(feature string) bool {
	switch strings.ToLower(feature) {
	case "opcache":
		// OPcache is available in PHP 5.5+
		return b.Major > 5 || (b.Major == 5 && b.Minor >= 5)
	case "jit":
		return b.Major >= 8
	case "fiber":
		return b.Major > 8 || (b.Major == 8 && b.Minor >= 1)
	default:
		return false
	}
}

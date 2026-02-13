package repoconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the repository-level build configuration
// This config file should be placed at the root of the repository as .build-app.yaml
type Config struct {
	// BuildArgs are build-time variables passed to Docker/BuildKit
	BuildArgs map[string]string `yaml:"buildArgs,omitempty"`

	// Env are environment variables set during the build process
	Env map[string]string `yaml:"env,omitempty"`

	// Dockerfile specifies a custom path to the Dockerfile (relative to repo root)
	Dockerfile string `yaml:"dockerfile,omitempty"`

	// Context specifies the build context path (relative to repo root)
	Context string `yaml:"context,omitempty"`

	// Target specifies the target stage in a multi-stage Dockerfile
	Target string `yaml:"target,omitempty"`

	// Platform specifies the target platform (e.g., "linux/amd64", "linux/arm64")
	Platform string `yaml:"platform,omitempty"`
}

const (
	// DefaultConfigFileName is the default name for the repository config file
	DefaultConfigFileName = ".build-app.yaml"
)

// Load reads and parses the repository configuration file
func Load(repoPath string) (*Config, error) {
	configPath := filepath.Join(repoPath, DefaultConfigFileName)

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Return empty config if file doesn't exist (not an error)
		return &Config{}, nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if cfg.Context == "" {
		cfg.Context = "."
	}
	if cfg.Dockerfile == "" {
		cfg.Dockerfile = "Dockerfile"
	}

	return &cfg, nil
}

// GetDockerfilePath returns the full path to the Dockerfile
func (c *Config) GetDockerfilePath(repoPath string) string {
	if c.Dockerfile == "" {
		return filepath.Join(repoPath, "Dockerfile")
	}
	return filepath.Join(repoPath, c.Dockerfile)
}

// GetContextPath returns the full path to the build context
func (c *Config) GetContextPath(repoPath string) string {
	if c.Context == "" {
		return repoPath
	}
	return filepath.Join(repoPath, c.Context)
}

// HasBuildArgs returns true if there are any build arguments configured
func (c *Config) HasBuildArgs() bool {
	return len(c.BuildArgs) > 0
}

// HasEnv returns true if there are any environment variables configured
func (c *Config) HasEnv() bool {
	return len(c.Env) > 0
}

package config

import (
	"fmt"
	"os"
)

// Config holds application configuration
type Config struct {
	Port              string
	WebhookSecret     string
	GitHubToken       string
	GitOpsRepo        string
	GitOpsToken       string
	KubeNamespace     string
	ContainerRegistry string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Port:              getEnv("PORT", "8080"),
		WebhookSecret:     os.Getenv("WEBHOOK_SECRET"),
		GitHubToken:       os.Getenv("GITHUB_TOKEN"),
		GitOpsRepo:        getEnv("GITOPS_REPO", ""),
		GitOpsToken:       getEnv("GITOPS_TOKEN", ""),
		KubeNamespace:     getEnv("KUBE_NAMESPACE", "default"),
		ContainerRegistry: getEnv("CONTAINER_REGISTRY", "ghcr.io"),
	}

	// Validate required fields
	if cfg.WebhookSecret == "" {
		return nil, fmt.Errorf("WEBHOOK_SECRET is required")
	}
	if cfg.GitHubToken == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

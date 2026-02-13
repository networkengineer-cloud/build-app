package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
	}{
		{
			name: "valid configuration",
			envVars: map[string]string{
				"WEBHOOK_SECRET": "test-secret",
				"GITHUB_TOKEN":   "test-token",
				"PORT":           "8080",
			},
			wantErr: false,
		},
		{
			name: "missing webhook secret",
			envVars: map[string]string{
				"GITHUB_TOKEN": "test-token",
			},
			wantErr: true,
		},
		{
			name: "missing github token",
			envVars: map[string]string{
				"WEBHOOK_SECRET": "test-secret",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			cfg, err := Load()

			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if cfg.WebhookSecret != tt.envVars["WEBHOOK_SECRET"] {
					t.Errorf("WebhookSecret = %v, want %v", cfg.WebhookSecret, tt.envVars["WEBHOOK_SECRET"])
				}
				if cfg.GitHubToken != tt.envVars["GITHUB_TOKEN"] {
					t.Errorf("GitHubToken = %v, want %v", cfg.GitHubToken, tt.envVars["GITHUB_TOKEN"])
				}
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		want         string
	}{
		{
			name:         "environment variable set",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "custom",
			want:         "custom",
		},
		{
			name:         "environment variable not set",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "",
			want:         "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
			}

			got := getEnv(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

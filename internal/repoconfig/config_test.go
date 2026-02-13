package repoconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name       string
		configYAML string
		wantConfig *Config
		wantErr    bool
	}{
		{
			name:       "no config file returns empty config",
			configYAML: "",
			wantConfig: &Config{},
			wantErr:    false,
		},
		{
			name: "valid config with build args",
			configYAML: `buildArgs:
  VERSION: "1.0.0"
  BUILD_DATE: "2024-01-01"
`,
			wantConfig: &Config{
				BuildArgs: map[string]string{
					"VERSION":    "1.0.0",
					"BUILD_DATE": "2024-01-01",
				},
				Context:    ".",
				Dockerfile: "Dockerfile",
			},
			wantErr: false,
		},
		{
			name: "valid config with env vars",
			configYAML: `env:
  NODE_ENV: "production"
  DEBUG: "false"
`,
			wantConfig: &Config{
				Env: map[string]string{
					"NODE_ENV": "production",
					"DEBUG":    "false",
				},
				Context:    ".",
				Dockerfile: "Dockerfile",
			},
			wantErr: false,
		},
		{
			name: "valid config with custom dockerfile path",
			configYAML: `dockerfile: docker/Dockerfile.prod
context: src
`,
			wantConfig: &Config{
				Dockerfile: "docker/Dockerfile.prod",
				Context:    "src",
			},
			wantErr: false,
		},
		{
			name: "valid config with target and platform",
			configYAML: `target: production
platform: linux/arm64
`,
			wantConfig: &Config{
				Target:     "production",
				Platform:   "linux/arm64",
				Context:    ".",
				Dockerfile: "Dockerfile",
			},
			wantErr: false,
		},
		{
			name: "full config",
			configYAML: `buildArgs:
  VERSION: "2.0.0"
  ENVIRONMENT: "staging"
env:
  LOG_LEVEL: "debug"
dockerfile: build/Dockerfile
context: app
target: builder
platform: linux/amd64
`,
			wantConfig: &Config{
				BuildArgs: map[string]string{
					"VERSION":     "2.0.0",
					"ENVIRONMENT": "staging",
				},
				Env: map[string]string{
					"LOG_LEVEL": "debug",
				},
				Dockerfile: "build/Dockerfile",
				Context:    "app",
				Target:     "builder",
				Platform:   "linux/amd64",
			},
			wantErr: false,
		},
		{
			name:       "invalid yaml",
			configYAML: "invalid: [yaml: content",
			wantConfig: nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir, err := os.MkdirTemp("", "test-config-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Create config file if yaml is provided
			if tt.configYAML != "" {
				configPath := filepath.Join(tmpDir, DefaultConfigFileName)
				if err := os.WriteFile(configPath, []byte(tt.configYAML), 0644); err != nil {
					t.Fatalf("Failed to write config file: %v", err)
				}
			}

			// Load config
			cfg, err := Load(tmpDir)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Compare results
			if cfg.Dockerfile != tt.wantConfig.Dockerfile {
				t.Errorf("Dockerfile = %v, want %v", cfg.Dockerfile, tt.wantConfig.Dockerfile)
			}
			if cfg.Context != tt.wantConfig.Context {
				t.Errorf("Context = %v, want %v", cfg.Context, tt.wantConfig.Context)
			}
			if cfg.Target != tt.wantConfig.Target {
				t.Errorf("Target = %v, want %v", cfg.Target, tt.wantConfig.Target)
			}
			if cfg.Platform != tt.wantConfig.Platform {
				t.Errorf("Platform = %v, want %v", cfg.Platform, tt.wantConfig.Platform)
			}

			// Compare maps
			if len(cfg.BuildArgs) != len(tt.wantConfig.BuildArgs) {
				t.Errorf("BuildArgs length = %d, want %d", len(cfg.BuildArgs), len(tt.wantConfig.BuildArgs))
			}
			for k, v := range tt.wantConfig.BuildArgs {
				if cfg.BuildArgs[k] != v {
					t.Errorf("BuildArgs[%s] = %v, want %v", k, cfg.BuildArgs[k], v)
				}
			}

			if len(cfg.Env) != len(tt.wantConfig.Env) {
				t.Errorf("Env length = %d, want %d", len(cfg.Env), len(tt.wantConfig.Env))
			}
			for k, v := range tt.wantConfig.Env {
				if cfg.Env[k] != v {
					t.Errorf("Env[%s] = %v, want %v", k, cfg.Env[k], v)
				}
			}
		})
	}
}

func TestGetDockerfilePath(t *testing.T) {
	tests := []struct {
		name       string
		config     *Config
		repoPath   string
		wantPath   string
	}{
		{
			name: "default dockerfile path",
			config: &Config{
				Dockerfile: "",
			},
			repoPath: "/repo",
			wantPath: "/repo/Dockerfile",
		},
		{
			name: "custom dockerfile path",
			config: &Config{
				Dockerfile: "docker/Dockerfile.prod",
			},
			repoPath: "/repo",
			wantPath: "/repo/docker/Dockerfile.prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetDockerfilePath(tt.repoPath)
			if got != tt.wantPath {
				t.Errorf("GetDockerfilePath() = %v, want %v", got, tt.wantPath)
			}
		})
	}
}

func TestGetContextPath(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		repoPath string
		wantPath string
	}{
		{
			name: "default context path",
			config: &Config{
				Context: "",
			},
			repoPath: "/repo",
			wantPath: "/repo",
		},
		{
			name: "custom context path",
			config: &Config{
				Context: "src",
			},
			repoPath: "/repo",
			wantPath: "/repo/src",
		},
		{
			name: "dot context path",
			config: &Config{
				Context: ".",
			},
			repoPath: "/repo",
			wantPath: "/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetContextPath(tt.repoPath)
			if got != tt.wantPath {
				t.Errorf("GetContextPath() = %v, want %v", got, tt.wantPath)
			}
		})
	}
}

func TestHasBuildArgs(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		want   bool
	}{
		{
			name:   "no build args",
			config: &Config{},
			want:   false,
		},
		{
			name: "has build args",
			config: &Config{
				BuildArgs: map[string]string{
					"VERSION": "1.0.0",
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.HasBuildArgs(); got != tt.want {
				t.Errorf("HasBuildArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasEnv(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		want   bool
	}{
		{
			name:   "no env vars",
			config: &Config{},
			want:   false,
		},
		{
			name: "has env vars",
			config: &Config{
				Env: map[string]string{
					"NODE_ENV": "production",
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.HasEnv(); got != tt.want {
				t.Errorf("HasEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

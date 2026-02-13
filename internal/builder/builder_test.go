package builder

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectBuildStrategy(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		expected BuildStrategy
	}{
		{
			name:     "dockerfile exists",
			files:    []string{"Dockerfile"},
			expected: StrategyDockerfile,
		},
		{
			name:     "go.mod exists",
			files:    []string{"go.mod", "main.go"},
			expected: StrategyGo,
		},
		{
			name:     "package.json exists",
			files:    []string{"package.json", "index.js"},
			expected: StrategyNodeJS,
		},
		{
			name:     "no build files",
			files:    []string{"README.md"},
			expected: StrategyUnknown,
		},
		{
			name:     "dockerfile takes precedence",
			files:    []string{"Dockerfile", "go.mod", "package.json"},
			expected: StrategyDockerfile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir, err := os.MkdirTemp("", "test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Create test files
			for _, file := range tt.files {
				path := filepath.Join(tmpDir, file)
				if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
					t.Fatalf("Failed to create file %s: %v", file, err)
				}
			}

			// Create builder (config doesn't matter for this test)
			builder := &Builder{}

			// Test detection
			strategy, err := builder.detectBuildStrategy(tmpDir)
			if err != nil {
				t.Errorf("detectBuildStrategy() error = %v", err)
				return
			}

			if strategy != tt.expected {
				t.Errorf("detectBuildStrategy() = %v, want %v", strategy, tt.expected)
			}
		})
	}
}

func TestEnsureDockerfile(t *testing.T) {
	tests := []struct {
		name          string
		strategy      BuildStrategy
		existingFiles []string
		shouldCreate  bool
		wantErr       bool
	}{
		{
			name:          "dockerfile already exists",
			strategy:      StrategyDockerfile,
			existingFiles: []string{"Dockerfile"},
			shouldCreate:  false,
			wantErr:       false,
		},
		{
			name:         "generate go dockerfile",
			strategy:     StrategyGo,
			shouldCreate: true,
			wantErr:      false,
		},
		{
			name:         "generate nodejs dockerfile",
			strategy:     StrategyNodeJS,
			shouldCreate: true,
			wantErr:      false,
		},
		{
			name:     "unknown strategy fails",
			strategy: StrategyUnknown,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir, err := os.MkdirTemp("", "test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Create existing files
			for _, file := range tt.existingFiles {
				path := filepath.Join(tmpDir, file)
				if err := os.WriteFile(path, []byte("existing"), 0644); err != nil {
					t.Fatalf("Failed to create file %s: %v", file, err)
				}
			}

			// Create builder
			builder := &Builder{}

			// Test ensure dockerfile
			err = builder.ensureDockerfile(tmpDir, tt.strategy)

			if (err != nil) != tt.wantErr {
				t.Errorf("ensureDockerfile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Check if Dockerfile exists
				dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
				if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
					t.Error("Dockerfile should exist but doesn't")
				}

				// If we created it, check content
				if tt.shouldCreate {
					content, err := os.ReadFile(dockerfilePath)
					if err != nil {
						t.Fatalf("Failed to read Dockerfile: %v", err)
					}
					if len(content) == 0 {
						t.Error("Generated Dockerfile is empty")
					}
				}
			}
		})
	}
}

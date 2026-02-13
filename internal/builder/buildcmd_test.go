package builder

import (
	"strings"
	"testing"

	"github.com/networkengineer-cloud/build-app/internal/config"
	"github.com/networkengineer-cloud/build-app/internal/repoconfig"
)

func TestBuildBuildctlCommand(t *testing.T) {
	tests := []struct {
		name       string
		imageName  string
		repoConfig *repoconfig.Config
		wantParts  []string // Parts that should be in the command
	}{
		{
			name:      "basic config with no customization",
			imageName: "ghcr.io/org/repo:tag",
			repoConfig: &repoconfig.Config{
				Dockerfile: "Dockerfile",
				Context:    ".",
			},
			wantParts: []string{
				"buildctl",
				"--frontend=dockerfile.v0",
				"--local context=/workspace",
				"--local dockerfile=/workspace",
				"--output type=image,name=ghcr.io/org/repo:tag",
			},
		},
		{
			name:      "config with build args",
			imageName: "ghcr.io/org/repo:tag",
			repoConfig: &repoconfig.Config{
				Dockerfile: "Dockerfile",
				Context:    ".",
				BuildArgs: map[string]string{
					"VERSION":    "1.0.0",
					"BUILD_DATE": "2024-01-01",
				},
			},
			wantParts: []string{
				"--opt build-arg:VERSION=1.0.0",
				"--opt build-arg:BUILD_DATE=2024-01-01",
			},
		},
		{
			name:      "config with custom context",
			imageName: "ghcr.io/org/repo:tag",
			repoConfig: &repoconfig.Config{
				Dockerfile: "Dockerfile",
				Context:    "services/api",
			},
			wantParts: []string{
				"--local context=/workspace/services/api",
			},
		},
		{
			name:      "config with custom dockerfile",
			imageName: "ghcr.io/org/repo:tag",
			repoConfig: &repoconfig.Config{
				Dockerfile: "docker/Dockerfile.prod",
				Context:    ".",
			},
			wantParts: []string{
				"--local dockerfile=/workspace/docker",
				"--opt filename=Dockerfile.prod",
			},
		},
		{
			name:      "config with target",
			imageName: "ghcr.io/org/repo:tag",
			repoConfig: &repoconfig.Config{
				Dockerfile: "Dockerfile",
				Context:    ".",
				Target:     "production",
			},
			wantParts: []string{
				"--opt target=production",
			},
		},
		{
			name:      "config with platform",
			imageName: "ghcr.io/org/repo:tag",
			repoConfig: &repoconfig.Config{
				Dockerfile: "Dockerfile",
				Context:    ".",
				Platform:   "linux/arm64",
			},
			wantParts: []string{
				"--opt platform=linux/arm64",
			},
		},
		{
			name:      "full config with all options",
			imageName: "ghcr.io/org/repo:tag",
			repoConfig: &repoconfig.Config{
				Dockerfile: "build/Dockerfile",
				Context:    "src",
				BuildArgs: map[string]string{
					"VERSION": "2.0.0",
				},
				Target:   "production",
				Platform: "linux/amd64",
			},
			wantParts: []string{
				"--local context=/workspace/src",
				"--local dockerfile=/workspace/build",
				"--opt filename=Dockerfile",
				"--opt build-arg:VERSION=2.0.0",
				"--opt target=production",
				"--opt platform=linux/amd64",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create builder
			cfg := &config.Config{
				KubeNamespace: "test",
			}
			b := &Builder{
				config: cfg,
			}

			// Build command
			cmd := b.buildBuildctlCommand(tt.imageName, tt.repoConfig)

			// Check that all expected parts are in the command
			for _, part := range tt.wantParts {
				if !strings.Contains(cmd, part) {
					t.Errorf("Expected command to contain %q, but it didn't.\nCommand: %s", part, cmd)
				}
			}

			// Basic validation
			if !strings.Contains(cmd, "buildkitd") {
				t.Error("Command should contain buildkitd")
			}
			if !strings.Contains(cmd, "buildctl") {
				t.Error("Command should contain buildctl")
			}
			if !strings.Contains(cmd, tt.imageName) {
				t.Errorf("Command should contain image name %s", tt.imageName)
			}
		})
	}
}

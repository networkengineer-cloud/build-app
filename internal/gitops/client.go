package gitops

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Client manages GitOps repository updates
type Client struct {
	repoURL string
	token   string
}

// NewClient creates a new GitOps client
func NewClient(repoURL, token string) *Client {
	return &Client{
		repoURL: repoURL,
		token:   token,
	}
}

// UpdateDeployment updates the deployment image in the GitOps repository
func (c *Client) UpdateDeployment(ctx context.Context, org, repo, imageName string) error {
	// Clone GitOps repo
	tmpDir, err := os.MkdirTemp("", "gitops-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	log.Printf("Cloning GitOps repository to %s", tmpDir)

	// Add token to URL for authentication
	cloneURL := c.repoURL
	if c.token != "" {
		// Insert token into URL (https://token@github.com/...)
		cloneURL = strings.Replace(cloneURL, "https://", fmt.Sprintf("https://%s@", c.token), 1)
	}

	cmd := exec.Command("git", "clone", cloneURL, tmpDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %w, output: %s", err, string(output))
	}

	// Path to deployment file
	deploymentPath := filepath.Join(tmpDir, "apps", org, repo, "deployment.yaml")

	// Ensure directory exists
	deploymentDir := filepath.Dir(deploymentPath)
	if err := os.MkdirAll(deploymentDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Read or create deployment file
	var deployment map[string]interface{}
	if _, err := os.Stat(deploymentPath); err == nil {
		// File exists, read it
		data, err := os.ReadFile(deploymentPath)
		if err != nil {
			return fmt.Errorf("failed to read deployment file: %w", err)
		}
		if err := yaml.Unmarshal(data, &deployment); err != nil {
			return fmt.Errorf("failed to parse deployment file: %w", err)
		}
	} else {
		// Create new deployment
		deployment = createDefaultDeployment(org, repo, imageName)
	}

	// Update image
	if err := updateImage(deployment, imageName); err != nil {
		return fmt.Errorf("failed to update image: %w", err)
	}

	// Write updated deployment
	data, err := yaml.Marshal(deployment)
	if err != nil {
		return fmt.Errorf("failed to marshal deployment: %w", err)
	}

	if err := os.WriteFile(deploymentPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write deployment file: %w", err)
	}

	// Commit and push changes
	cmd = exec.Command("git", "config", "user.email", "build-app@github.com")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git config email failed: %w, output: %s", err, string(output))
	}

	cmd = exec.Command("git", "config", "user.name", "Build App")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git config name failed: %w, output: %s", err, string(output))
	}

	cmd = exec.Command("git", "add", deploymentPath)
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %w, output: %s", err, string(output))
	}

	commitMsg := fmt.Sprintf("Update %s/%s to %s", org, repo, imageName)
	cmd = exec.Command("git", "commit", "-m", commitMsg)
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		// Check if there are no changes to commit
		if !strings.Contains(string(output), "nothing to commit") {
			return fmt.Errorf("git commit failed: %w, output: %s", err, string(output))
		}
		log.Printf("No changes to commit")
		return nil
	}

	cmd = exec.Command("git", "push")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git push failed: %w, output: %s", err, string(output))
	}

	log.Printf("Successfully updated GitOps repository")
	return nil
}

func createDefaultDeployment(org, repo, imageName string) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      repo,
			"namespace": "default",
			"labels": map[string]interface{}{
				"app": repo,
			},
		},
		"spec": map[string]interface{}{
			"replicas": 1,
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"app": repo,
				},
			},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app": repo,
					},
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  repo,
							"image": imageName,
							"ports": []interface{}{
								map[string]interface{}{
									"containerPort": 8080,
								},
							},
						},
					},
				},
			},
		},
	}
}

func updateImage(deployment map[string]interface{}, imageName string) error {
	spec, ok := deployment["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid deployment spec")
	}

	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid deployment template")
	}

	templateSpec, ok := template["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid template spec")
	}

	containers, ok := templateSpec["containers"].([]interface{})
	if !ok || len(containers) == 0 {
		return fmt.Errorf("invalid or empty containers")
	}

	container, ok := containers[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid container")
	}

	container["image"] = imageName
	return nil
}

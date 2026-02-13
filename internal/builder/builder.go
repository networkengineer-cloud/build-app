package builder

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/networkengineer-cloud/build-app/internal/config"
	"github.com/networkengineer-cloud/build-app/internal/gitops"
	"github.com/networkengineer-cloud/build-app/internal/repoconfig"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// BuildRequest represents a build request
type BuildRequest struct {
	Org       string
	Repo      string
	CloneURL  string
	SHA       string
	ImageTag  string
	Branch    string
	IsPR      bool
	PRNumber  int
	IsTag     bool
}

// Builder manages container builds
type Builder struct {
	config     *config.Config
	kubeClient *kubernetes.Clientset
	gitops     *gitops.Client
}

// NewBuilder creates a new builder
func NewBuilder(cfg *config.Config) *Builder {
	kubeClient, err := getKubernetesClient()
	if err != nil {
		log.Printf("Warning: failed to create Kubernetes client: %v", err)
	}

	var gitopsClient *gitops.Client
	if cfg.GitOpsRepo != "" {
		gitopsClient = gitops.NewClient(cfg.GitOpsRepo, cfg.GitOpsToken)
	}

	return &Builder{
		config:     cfg,
		kubeClient: kubeClient,
		gitops:     gitopsClient,
	}
}

// Build executes the build process
func (b *Builder) Build(ctx context.Context, req *BuildRequest) error {
	log.Printf("Starting build for %s/%s with tag %s", req.Org, req.Repo, req.ImageTag)

	// Clone repository
	tmpDir, err := b.cloneRepo(req.CloneURL, req.SHA)
	if err != nil {
		return fmt.Errorf("failed to clone repo: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Load repository configuration
	repoConfig, err := repoconfig.Load(tmpDir)
	if err != nil {
		return fmt.Errorf("failed to load repository config: %w", err)
	}
	log.Printf("Loaded repository config from %s", tmpDir)

	// Detect build strategy
	strategy, err := b.detectBuildStrategy(tmpDir)
	if err != nil {
		return fmt.Errorf("failed to detect build strategy: %w", err)
	}
	log.Printf("Detected build strategy: %s", strategy)

	// Ensure Dockerfile exists
	if err := b.ensureDockerfile(tmpDir, strategy); err != nil {
		return fmt.Errorf("failed to ensure Dockerfile: %w", err)
	}

	// Build image using BuildKit
	imageName := fmt.Sprintf("%s/%s/%s:%s", b.config.ContainerRegistry, req.Org, req.Repo, req.ImageTag)
	if err := b.buildWithBuildKit(ctx, req, imageName, tmpDir, repoConfig); err != nil {
		return fmt.Errorf("failed to build with BuildKit: %w", err)
	}

	log.Printf("Successfully built image: %s", imageName)

	// Update GitOps repository if configured
	if b.gitops != nil && !req.IsPR {
		if err := b.gitops.UpdateDeployment(ctx, req.Org, req.Repo, imageName); err != nil {
			log.Printf("Warning: failed to update GitOps repo: %v", err)
		} else {
			log.Printf("Successfully updated GitOps repository")
		}
	}

	return nil
}

func (b *Builder) cloneRepo(cloneURL, sha string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "build-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	log.Printf("Cloning repository to %s", tmpDir)

	// Clone the repository
	cmd := exec.Command("git", "clone", cloneURL, tmpDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("git clone failed: %w, output: %s", err, string(output))
	}

	// Checkout the specific commit
	cmd = exec.Command("git", "checkout", sha)
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("git checkout failed: %w, output: %s", err, string(output))
	}

	return tmpDir, nil
}

// BuildStrategy represents the detected build strategy
type BuildStrategy string

const (
	StrategyDockerfile BuildStrategy = "dockerfile"
	StrategyGo         BuildStrategy = "go"
	StrategyNodeJS     BuildStrategy = "nodejs"
	StrategyUnknown    BuildStrategy = "unknown"
)

func (b *Builder) detectBuildStrategy(repoDir string) (BuildStrategy, error) {
	// Check for Dockerfile
	dockerfilePath := filepath.Join(repoDir, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); err == nil {
		return StrategyDockerfile, nil
	}

	// Check for go.mod (Go project)
	goModPath := filepath.Join(repoDir, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		return StrategyGo, nil
	}

	// Check for package.json (Node.js project)
	packageJSONPath := filepath.Join(repoDir, "package.json")
	if _, err := os.Stat(packageJSONPath); err == nil {
		return StrategyNodeJS, nil
	}

	return StrategyUnknown, nil
}

func (b *Builder) ensureDockerfile(repoDir string, strategy BuildStrategy) error {
	dockerfilePath := filepath.Join(repoDir, "Dockerfile")

	// If Dockerfile already exists, use it
	if _, err := os.Stat(dockerfilePath); err == nil {
		return nil
	}

	// Generate Dockerfile based on strategy
	var dockerfileContent string
	switch strategy {
	case StrategyGo:
		dockerfileContent = `FROM golang:1.23 AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]
`
	case StrategyNodeJS:
		dockerfileContent = `FROM node:18-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .

FROM node:18-alpine
WORKDIR /app
COPY --from=builder /app .
EXPOSE 3000
CMD ["node", "index.js"]
`
	default:
		return fmt.Errorf("cannot generate Dockerfile for strategy: %s", strategy)
	}

	// Write generated Dockerfile
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
		return fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	log.Printf("Generated Dockerfile for strategy: %s", strategy)
	return nil
}

func (b *Builder) buildWithBuildKit(ctx context.Context, req *BuildRequest, imageName, repoDir string, repoConfig *repoconfig.Config) error {
	if b.kubeClient == nil {
		return fmt.Errorf("Kubernetes client not initialized")
	}

	// Create BuildKit Job
	job := b.createBuildKitJob(req, imageName, repoDir, repoConfig)

	log.Printf("Creating BuildKit job: %s", job.Name)

	// Create the job
	createdJob, err := b.kubeClient.BatchV1().Jobs(b.config.KubeNamespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	// Wait for job completion (with timeout)
	timeout := time.After(15 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("job timeout after 15 minutes")
		case <-ticker.C:
			currentJob, err := b.kubeClient.BatchV1().Jobs(b.config.KubeNamespace).Get(ctx, createdJob.Name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get job status: %w", err)
			}

			if currentJob.Status.Succeeded > 0 {
				log.Printf("Job completed successfully")
				return nil
			}

			if currentJob.Status.Failed > 0 {
				return fmt.Errorf("job failed")
			}
		}
	}
}

func (b *Builder) createBuildKitJob(req *BuildRequest, imageName, repoDir string, repoConfig *repoconfig.Config) *batchv1.Job {
	jobName := fmt.Sprintf("build-%s-%s-%s", req.Repo, req.ImageTag, time.Now().Format("20060102-150405"))
	jobName = strings.ReplaceAll(jobName, ".", "-")
	jobName = strings.ToLower(jobName)
	if len(jobName) > 63 {
		jobName = jobName[:63]
	}

	// Build the buildctl command with config options
	buildCmd := b.buildBuildctlCommand(imageName, repoConfig)

	// Prepare environment variables for buildkit container
	envVars := []corev1.EnvVar{
		{
			Name:  "DOCKER_CONFIG",
			Value: "/root/.docker",
		},
	}

	// Add custom environment variables from repo config
	if repoConfig.HasEnv() {
		for key, value := range repoConfig.Env {
			envVars = append(envVars, corev1.EnvVar{
				Name:  key,
				Value: value,
			})
		}
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: b.config.KubeNamespace,
			Labels: map[string]string{
				"app":       "build-app",
				"org":       req.Org,
				"repo":      req.Repo,
				"image-tag": req.ImageTag,
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: int32Ptr(3600), // Clean up after 1 hour
			BackoffLimit:            int32Ptr(2),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{
						{
							Name:  "clone",
							Image: "alpine/git:latest",
							Command: []string{
								"sh",
								"-c",
								fmt.Sprintf("git clone %s /workspace && cd /workspace && git checkout %s", req.CloneURL, req.SHA),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "workspace",
									MountPath: "/workspace",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "buildkit",
							Image: "moby/buildkit:latest",
							Command: []string{
								"sh",
								"-c",
								buildCmd,
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: boolPtr(true),
							},
							Env: envVars,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "workspace",
									MountPath: "/workspace",
								},
								{
									Name:      "docker-config",
									MountPath: "/root/.docker",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "workspace",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "docker-config",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "docker-config",
									Items: []corev1.KeyToPath{
										{
											Key:  "config.json",
											Path: "config.json",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// buildBuildctlCommand constructs the buildctl command with repository config options
func (b *Builder) buildBuildctlCommand(imageName string, repoConfig *repoconfig.Config) string {
	// Base command
	cmd := "buildkitd --addr unix:///run/buildkit/buildkitd.sock & sleep 2 && buildctl --addr unix:///run/buildkit/buildkitd.sock build"
	
	// Frontend
	cmd += " --frontend=dockerfile.v0"
	
	// Context path (default to /workspace if not specified or is ".")
	contextPath := "/workspace"
	if repoConfig.Context != "." {
		contextPath = "/workspace/" + repoConfig.Context
	}
	cmd += " --local context=" + contextPath
	
	// Dockerfile path
	if repoConfig.Dockerfile != "Dockerfile" {
		// If custom dockerfile, specify the directory containing it
		dockerfileDir := filepath.Dir("/workspace/" + repoConfig.Dockerfile)
		cmd += " --local dockerfile=" + dockerfileDir
		// Also specify the actual filename
		cmd += " --opt filename=" + filepath.Base(repoConfig.Dockerfile)
	} else {
		cmd += " --local dockerfile=/workspace"
	}
	
	// Build arguments
	if repoConfig.HasBuildArgs() {
		for key, value := range repoConfig.BuildArgs {
			cmd += fmt.Sprintf(" --opt build-arg:%s=%s", key, value)
		}
	}
	
	// Target stage (if specified)
	if repoConfig.Target != "" {
		cmd += " --opt target=" + repoConfig.Target
	}
	
	// Platform (if specified)
	if repoConfig.Platform != "" {
		cmd += " --opt platform=" + repoConfig.Platform
	}
	
	// Output (push to registry)
	cmd += " --output type=image,name=" + imageName + ",push=true,registry.insecure=false"
	
	return cmd
}

func boolPtr(b bool) *bool {
	return &b
}

func int32Ptr(i int32) *int32 {
	return &i
}

func getKubernetesClient() (*kubernetes.Clientset, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build config: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return clientset, nil
}

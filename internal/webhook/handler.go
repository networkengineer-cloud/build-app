package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/networkengineer-cloud/build-app/internal/builder"
	"github.com/networkengineer-cloud/build-app/internal/config"
	"github.com/networkengineer-cloud/build-app/internal/github"
)

// Handler manages webhook requests
type Handler struct {
	config  *config.Config
	builder *builder.Builder
	github  *github.Client
}

// NewHandler creates a new webhook handler
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{
		config:  cfg,
		builder: builder.NewBuilder(cfg),
		github:  github.NewClient(cfg.GitHubToken),
	}
}

// HandleWebhook processes incoming GitHub webhooks
func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		http.Error(w, "Error reading body", http.StatusBadRequest)
		return
	}
	defer func() { _ = r.Body.Close() }()

	// Validate signature
	signature := r.Header.Get("X-Hub-Signature-256")
	if !h.validateSignature(body, signature) {
		log.Printf("Invalid signature")
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Get event type
	event := r.Header.Get("X-GitHub-Event")
	log.Printf("Received webhook event: %s", event)

	// Process based on event type
	switch event {
	case "push":
		if err := h.handlePush(body); err != nil {
			log.Printf("Error handling push: %v", err)
			http.Error(w, "Error processing push", http.StatusInternalServerError)
			return
		}
	case "pull_request":
		if err := h.handlePullRequest(body); err != nil {
			log.Printf("Error handling pull request: %v", err)
			http.Error(w, "Error processing pull request", http.StatusInternalServerError)
			return
		}
	case "create":
		if err := h.handleCreate(body); err != nil {
			log.Printf("Error handling create: %v", err)
			http.Error(w, "Error processing create", http.StatusInternalServerError)
			return
		}
	default:
		log.Printf("Unsupported event type: %s", event)
		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "Webhook processed successfully")
}

func (h *Handler) validateSignature(body []byte, signature string) bool {
	if signature == "" {
		return false
	}

	// Remove "sha256=" prefix
	signature = strings.TrimPrefix(signature, "sha256=")

	// Calculate expected signature
	mac := hmac.New(sha256.New, []byte(h.config.WebhookSecret))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedMAC))
}

func (h *Handler) handlePush(body []byte) error {
	var payload struct {
		Ref        string `json:"ref"`
		After      string `json:"after"`
		Repository struct {
			FullName string `json:"full_name"`
			CloneURL string `json:"clone_url"`
		} `json:"repository"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("failed to parse push payload: %w", err)
	}

	// Extract branch name
	branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
	sha := payload.After
	parts := strings.Split(payload.Repository.FullName, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository name: %s", payload.Repository.FullName)
	}
	org := parts[0]
	repo := parts[1]

	log.Printf("Processing push: %s/%s branch=%s sha=%s", org, repo, branch, sha[:7])

	// Update GitHub status to pending
	ctx := context.Background()
	if err := h.github.UpdateCommitStatus(ctx, org, repo, sha, "pending", "Building container image..."); err != nil {
		log.Printf("Warning: failed to update commit status: %v", err)
	}

	// Generate image tag
	imageTag := fmt.Sprintf("%s-%s", branch, sha[:7])

	// Build and push image
	buildReq := &builder.BuildRequest{
		Org:       org,
		Repo:      repo,
		CloneURL:  payload.Repository.CloneURL,
		SHA:       sha,
		ImageTag:  imageTag,
		Branch:    branch,
		IsPR:      false,
	}

	if err := h.builder.Build(ctx, buildReq); err != nil {
		_ = h.github.UpdateCommitStatus(ctx, org, repo, sha, "failure", fmt.Sprintf("Build failed: %v", err))
		return fmt.Errorf("build failed: %w", err)
	}

	// Update GitHub status to success
	if err := h.github.UpdateCommitStatus(ctx, org, repo, sha, "success", "Container image built successfully"); err != nil {
		log.Printf("Warning: failed to update commit status: %v", err)
	}

	return nil
}

func (h *Handler) handlePullRequest(body []byte) error {
	var payload struct {
		Action      string `json:"action"`
		Number      int    `json:"number"`
		PullRequest struct {
			Head struct {
				SHA string `json:"sha"`
				Ref string `json:"ref"`
			} `json:"head"`
		} `json:"pull_request"`
		Repository struct {
			FullName string `json:"full_name"`
			CloneURL string `json:"clone_url"`
		} `json:"repository"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("failed to parse PR payload: %w", err)
	}

	// Only process opened, synchronize, and reopened actions
	if payload.Action != "opened" && payload.Action != "synchronize" && payload.Action != "reopened" {
		log.Printf("Ignoring PR action: %s", payload.Action)
		return nil
	}

	sha := payload.PullRequest.Head.SHA
	parts := strings.Split(payload.Repository.FullName, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository name: %s", payload.Repository.FullName)
	}
	org := parts[0]
	repo := parts[1]

	log.Printf("Processing PR: %s/%s pr=%d sha=%s", org, repo, payload.Number, sha[:7])

	// Update GitHub status to pending
	ctx := context.Background()
	if err := h.github.UpdateCommitStatus(ctx, org, repo, sha, "pending", "Building container image..."); err != nil {
		log.Printf("Warning: failed to update commit status: %v", err)
	}

	// Generate image tag for PR
	imageTag := fmt.Sprintf("pr-%d-%s", payload.Number, sha[:7])

	// Build and push image
	buildReq := &builder.BuildRequest{
		Org:       org,
		Repo:      repo,
		CloneURL:  payload.Repository.CloneURL,
		SHA:       sha,
		ImageTag:  imageTag,
		IsPR:      true,
		PRNumber:  payload.Number,
	}

	if err := h.builder.Build(ctx, buildReq); err != nil {
		_ = h.github.UpdateCommitStatus(ctx, org, repo, sha, "failure", fmt.Sprintf("Build failed: %v", err))
		return fmt.Errorf("build failed: %w", err)
	}

	// Update GitHub status to success
	if err := h.github.UpdateCommitStatus(ctx, org, repo, sha, "success", "Container image built successfully"); err != nil {
		log.Printf("Warning: failed to update commit status: %v", err)
	}

	return nil
}

func (h *Handler) handleCreate(body []byte) error {
	var payload struct {
		RefType string `json:"ref_type"`
		Ref     string `json:"ref"`
		Repository struct {
			FullName string `json:"full_name"`
			CloneURL string `json:"clone_url"`
		} `json:"repository"`
		MasterBranch string `json:"master_branch"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("failed to parse create payload: %w", err)
	}

	// Only process tag creation
	if payload.RefType != "tag" {
		log.Printf("Ignoring create event for ref_type: %s", payload.RefType)
		return nil
	}

	tag := payload.Ref
	parts := strings.Split(payload.Repository.FullName, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository name: %s", payload.Repository.FullName)
	}
	org := parts[0]
	repo := parts[1]

	log.Printf("Processing tag: %s/%s tag=%s", org, repo, tag)

	// For tags, we need to get the commit SHA
	ctx := context.Background()
	sha, err := h.github.GetTagSHA(ctx, org, repo, tag)
	if err != nil {
		return fmt.Errorf("failed to get tag SHA: %w", err)
	}

	// Update GitHub status to pending
	if err := h.github.UpdateCommitStatus(ctx, org, repo, sha, "pending", "Building container image..."); err != nil {
		log.Printf("Warning: failed to update commit status: %v", err)
	}

	// Use the tag as the image tag
	imageTag := tag

	// Build and push image
	buildReq := &builder.BuildRequest{
		Org:       org,
		Repo:      repo,
		CloneURL:  payload.Repository.CloneURL,
		SHA:       sha,
		ImageTag:  imageTag,
		IsTag:     true,
	}

	if err := h.builder.Build(ctx, buildReq); err != nil {
		_ = h.github.UpdateCommitStatus(ctx, org, repo, sha, "failure", fmt.Sprintf("Build failed: %v", err))
		return fmt.Errorf("build failed: %w", err)
	}

	// Update GitHub status to success
	if err := h.github.UpdateCommitStatus(ctx, org, repo, sha, "success", "Container image built successfully"); err != nil {
		log.Printf("Warning: failed to update commit status: %v", err)
	}

	return nil
}

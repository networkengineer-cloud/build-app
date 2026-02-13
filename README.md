# Build App - Zero-Config Webhook Build Service

A Kubernetes-native webhook service that automatically builds and deploys container images from GitHub repositories.

## Features

- **GitHub Webhook Integration**: Automatically triggered by push, pull request, and tag events
- **Zero Configuration**: Automatically detects build strategy (Dockerfile, Go, Node.js)
- **Secure**: GitHub webhook signature validation
- **BuildKit-based Builds**: Builds container images using Docker's modern BuildKit engine
- **GitOps Integration**: Automatically updates deployment manifests
- **GitHub Status Updates**: Real-time build status feedback
- **Image Tagging**: Smart tagging strategy (main-{sha}, pr-{num}-{sha}, v1.2.3)

## Architecture

```
GitHub Webhook → Build App → BuildKit Job → GHCR → GitOps Repo
                                ↓
                         GitHub Commit Status
```

## Core Flow

1. **Receive GitHub webhook** (push/PR/tag) → validate signature
2. **Clone repository** → detect build strategy (Dockerfile exists? go.mod? package.json?)
3. **Create ephemeral K8s Job** running BuildKit to build container
4. **Push to GHCR** as `ghcr.io/{org}/{repo}:{tag}` (tags: `main-{sha}`, `pr-{num}-{sha}`, `v1.2.3`)
5. **Update GitOps repo**: modify `apps/{org}/{repo}/deployment.yaml` image field
6. **Update GitHub commit status** (pending→success/failure)

## Build Strategy Detection

The service automatically detects the appropriate build strategy:

1. **Dockerfile exists**: Use existing Dockerfile
2. **go.mod exists**: Generate Go multi-stage Dockerfile
3. **package.json exists**: Generate Node.js multi-stage Dockerfile
4. **Otherwise**: Fail with error message

## Installation

### Prerequisites

- Kubernetes cluster (1.24+)
- kubectl configured
- GitHub personal access token with repo permissions
- GitHub Container Registry (GHCR) access token

### Deploy to Kubernetes

1. Create namespace:
   ```bash
   kubectl create namespace build-app
   ```

2. Create secrets:
   ```bash
   # GitHub webhook secret
   kubectl create secret generic build-app-secrets \
     --from-literal=WEBHOOK_SECRET=your-webhook-secret \
     --from-literal=GITHUB_TOKEN=your-github-token \
     --from-literal=GITOPS_TOKEN=your-gitops-token \
     -n build-app
   
   # Docker registry credentials for BuildKit
   kubectl create secret docker-registry docker-config \
     --docker-server=ghcr.io \
     --docker-username=your-github-username \
     --docker-password=your-github-token \
     -n build-app
   ```

3. Update ConfigMap in `deploy/kubernetes/deployment.yaml`:
   ```yaml
   data:
     GITOPS_REPO: "https://github.com/your-org/gitops-repo.git"
   ```

4. Deploy:
   ```bash
   kubectl apply -f deploy/kubernetes/deployment.yaml
   kubectl apply -f deploy/kubernetes/ingress.yaml
   ```

5. Configure GitHub webhook:
   - Go to repository Settings → Webhooks → Add webhook
   - Payload URL: `https://your-domain/webhook`
   - Content type: `application/json`
   - Secret: Use the same value as `WEBHOOK_SECRET`
   - Events: Push, Pull requests, Create (for tags)

## Configuration

Environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| PORT | No | 8080 | HTTP server port |
| WEBHOOK_SECRET | Yes | - | GitHub webhook secret for signature validation |
| GITHUB_TOKEN | Yes | - | GitHub personal access token |
| GITOPS_REPO | No | - | GitOps repository URL (e.g., https://github.com/org/gitops) |
| GITOPS_TOKEN | No | - | Token for GitOps repository (if different from GITHUB_TOKEN) |
| KUBE_NAMESPACE | No | default | Kubernetes namespace for build jobs |
| CONTAINER_REGISTRY | No | ghcr.io | Container registry URL |

## Image Tagging Strategy

- **Push to main/master**: `main-{short-sha}` (e.g., `main-abc1234`)
- **Push to other branches**: `{branch}-{short-sha}` (e.g., `develop-abc1234`)
- **Pull requests**: `pr-{number}-{short-sha}` (e.g., `pr-42-abc1234`)
- **Tags**: `{tag}` (e.g., `v1.2.3`)

## Development

### Build locally

```bash
go build -o build-app .
```

### Run locally

```bash
export WEBHOOK_SECRET=test-secret
export GITHUB_TOKEN=your-token
export PORT=8080
./build-app
```

### Build Docker image

```bash
docker build -t build-app:latest .
```

## Testing

Send a test webhook:

```bash
curl -X POST http://localhost:8080/webhook \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: push" \
  -H "X-Hub-Signature-256: sha256=$(echo -n '{}' | openssl dgst -sha256 -hmac 'test-secret' | cut -d' ' -f2)" \
  -d '{}'
```

## Security

- Webhook signatures are validated using HMAC-SHA256
- Kubernetes RBAC restricts service account permissions
- Secrets are stored in Kubernetes secrets
- BuildKit requires privileged access for container builds

## Troubleshooting

### Check service logs
```bash
kubectl logs -f deployment/build-app -n build-app
```

### Check build job status
```bash
kubectl get jobs -n build-app
kubectl logs job/build-<repo>-<tag>-<timestamp> -n build-app -c buildkit
```

### Common issues

1. **Webhook signature validation fails**: Ensure `WEBHOOK_SECRET` matches GitHub webhook secret
2. **Build job fails**: Check BuildKit logs and ensure Docker registry credentials are correct
3. **GitOps update fails**: Verify `GITOPS_TOKEN` has write access to GitOps repository

## License

MIT
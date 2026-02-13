# Build App - Zero-Config Webhook Build Service

A Kubernetes-native webhook service that automatically builds and deploys container images from GitHub repositories.

## Features

- **GitHub Webhook Integration**: Automatically triggered by push, pull request, and tag events
- **Zero Configuration**: Automatically detects build strategy (Dockerfile, Go, Node.js)
- **Repository Configuration**: Optional `.build-app.yaml` for build customization ([docs](docs/CONFIGURATION.md))
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

## Repository Configuration

You can customize the build process by creating a `.build-app.yaml` file at the root of your repository. This is optional - if not present, the service will use default behavior.

### Configuration Options

```yaml
# Build arguments passed to Docker/BuildKit
buildArgs:
  VERSION: "1.0.0"
  BUILD_DATE: "2024-01-01"

# Environment variables set during build
env:
  NODE_ENV: "production"
  DEBUG: "false"

# Custom Dockerfile path (relative to repo root)
# Default: Dockerfile
dockerfile: docker/Dockerfile.prod

# Build context path (relative to repo root)
# Default: . (repo root)
context: src

# Target stage in multi-stage Dockerfile
target: production

# Target platform (e.g., linux/amd64, linux/arm64)
platform: linux/amd64
```

### Example Configurations

See the [`examples/`](examples/) directory for complete configuration examples:
- **Basic**: Simple build args and environment variables
- **Advanced**: Custom paths, targets, and platforms
- **Monorepo**: Building from a subdirectory
- **Multi-stage**: Building specific stages

### Build Arguments in Dockerfile

To use build arguments, reference them in your Dockerfile:

```dockerfile
ARG VERSION
ARG BUILD_DATE

FROM node:18
LABEL version=${VERSION}
LABEL build_date=${BUILD_DATE}
# ... rest of your Dockerfile
```

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

### OpenTelemetry Configuration

The service supports OpenTelemetry for observability (metrics, traces, and logs):

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| OTEL_SERVICE_NAME | No | build-app | Service name for telemetry |
| SERVICE_VERSION | No | 1.0.0 | Service version |
| OTEL_EXPORTER_OTLP_ENDPOINT | No | - | OTLP endpoint URL (e.g., localhost:4318) |
| OTEL_TRACES_ENABLED | No | true | Enable/disable distributed tracing |
| OTEL_METRICS_ENABLED | No | true | Enable/disable metrics collection |

#### Metrics Collected

- `build.duration` - Histogram of container build durations (seconds)
- `build.count` - Counter of builds executed (by status: success/failure, type)
- `webhook.count` - Counter of webhooks processed (by event type, status)
- `http.server.duration` - HTTP request durations (via middleware)

#### Traces

Distributed traces are collected for:
- Webhook request handling
- Repository cloning
- Build execution
- GitHub API calls
- GitOps updates

#### Example Configuration

To send telemetry to an OTLP collector:

```yaml
env:
  - name: OTEL_EXPORTER_OTLP_ENDPOINT
    value: "otel-collector.monitoring.svc.cluster.local:4318"
  - name: OTEL_SERVICE_NAME
    value: "build-app"
  - name: SERVICE_VERSION
    value: "1.0.0"
  - name: OTEL_TRACES_ENABLED
    value: "true"
  - name: OTEL_METRICS_ENABLED
    value: "true"
```

When `OTEL_EXPORTER_OTLP_ENDPOINT` is not set, the service runs with no-op telemetry (no overhead).

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

## Documentation

- **[Configuration Guide](docs/CONFIGURATION.md)** - Customize builds with `.build-app.yaml`
- **[Setup Guide](SETUP.md)** - Detailed deployment instructions
- **[Contributing](CONTRIBUTING.md)** - Development guidelines

## License

MIT
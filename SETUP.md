# Quick Setup Guide

This guide will help you deploy the Build App webhook service to your Kubernetes cluster.

## Prerequisites

- Kubernetes cluster (1.24+)
- kubectl configured and connected to your cluster
- GitHub personal access token with `repo` and `write:packages` permissions
- A domain name for webhook endpoint (optional, for production)

## Step 1: Prepare Secrets

### 1.1 Generate Webhook Secret

```bash
WEBHOOK_SECRET=$(openssl rand -hex 32)
echo "Save this webhook secret: $WEBHOOK_SECRET"
```

### 1.2 Get GitHub Token

Create a GitHub Personal Access Token (PAT) with these permissions:
- `repo` (full control)
- `write:packages` (to push to GHCR)
- `read:org` (if building org repos)

Save it as `GITHUB_TOKEN`.

### 1.3 GitOps Token (Optional)

If using a separate GitOps repository, create another token with write access to that repo.

## Step 2: Deploy to Kubernetes

### 2.1 Create Namespace

```bash
kubectl create namespace build-app
```

### 2.2 Create Secrets

```bash
# Main application secrets
kubectl create secret generic build-app-secrets \
  --from-literal=WEBHOOK_SECRET="$WEBHOOK_SECRET" \
  --from-literal=GITHUB_TOKEN="$GITHUB_TOKEN" \
  --from-literal=GITOPS_TOKEN="$GITHUB_TOKEN" \
  -n build-app

# Docker registry credentials for BuildKit
kubectl create secret generic docker-config \
  --from-literal=config.json="{\"auths\":{\"ghcr.io\":{\"auth\":\"$(echo -n "USERNAME:$GITHUB_TOKEN" | base64)\"}}}" \
  -n build-app
```

Replace `USERNAME` with your GitHub username.

### 2.3 Update Configuration

Edit `deploy/kubernetes/deployment.yaml` and update the ConfigMap:

```yaml
data:
  GITOPS_REPO: "https://github.com/your-org/gitops-repo.git"
```

### 2.4 Deploy Application

```bash
kubectl apply -f deploy/kubernetes/deployment.yaml
```

### 2.5 Verify Deployment

```bash
kubectl get pods -n build-app
kubectl logs -f deployment/build-app -n build-app
```

## Step 3: Expose Webhook Endpoint

### Option A: Using Ingress (Recommended for Production)

1. Install nginx-ingress-controller if not already installed:
   ```bash
   kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/cloud/deploy.yaml
   ```

2. Update `deploy/kubernetes/ingress.yaml` with your domain:
   ```yaml
   spec:
     rules:
     - host: webhook.your-domain.com
   ```

3. Deploy ingress:
   ```bash
   kubectl apply -f deploy/kubernetes/ingress.yaml
   ```

### Option B: Using LoadBalancer (Quick Testing)

```bash
kubectl patch service build-app -n build-app -p '{"spec": {"type": "LoadBalancer"}}'
kubectl get service build-app -n build-app
```

### Option C: Port Forward (Local Testing)

```bash
kubectl port-forward service/build-app 8080:80 -n build-app
```

Your webhook will be available at `http://localhost:8080/webhook`

## Step 4: Configure GitHub Webhook

1. Go to your repository on GitHub
2. Navigate to **Settings** → **Webhooks** → **Add webhook**
3. Configure:
   - **Payload URL**: `https://webhook.your-domain.com/webhook` (or your endpoint)
   - **Content type**: `application/json`
   - **Secret**: Use the `WEBHOOK_SECRET` from Step 1.1
   - **Which events**: Select:
     - Push events
     - Pull requests
     - Branch or tag creation
4. Click **Add webhook**

## Step 5: Test the Setup

### 5.1 Trigger a Build

Push a commit to your repository:

```bash
git commit --allow-empty -m "Test webhook build"
git push
```

### 5.2 Check Build Status

```bash
# Check if job was created
kubectl get jobs -n build-app

# View job logs
kubectl logs job/<job-name> -n build-app -c buildkit

# Check application logs
kubectl logs -f deployment/build-app -n build-app
```

### 5.3 Verify Image in GHCR

Go to your repository on GitHub → **Packages** to see the built image.

## Troubleshooting

### Webhook not triggering

1. Check webhook delivery in GitHub:
   - Go to repository **Settings** → **Webhooks**
   - Click on your webhook
   - Check **Recent Deliveries** for errors

2. Verify signature validation:
   ```bash
   kubectl logs deployment/build-app -n build-app | grep signature
   ```

### Build job fails

1. Check job status:
   ```bash
   kubectl describe job/<job-name> -n build-app
   ```

2. Check BuildKit logs:
   ```bash
   kubectl logs job/<job-name> -n build-app -c buildkit
   ```

3. Common issues:
   - Missing docker-config secret
   - Invalid registry credentials
   - Network connectivity issues

### GitOps update fails

1. Verify token permissions:
   ```bash
   curl -H "Authorization: Bearer $GITOPS_TOKEN" https://api.github.com/user
   ```

2. Check logs:
   ```bash
   kubectl logs deployment/build-app -n build-app | grep GitOps
   ```

## Security Notes

1. **Keep secrets secure**: Never commit secrets to version control
2. **Rotate tokens regularly**: Update GitHub tokens periodically
3. **Use TLS**: Always use HTTPS for webhook endpoints in production
4. **Network policies**: Consider adding Kubernetes network policies to restrict traffic
5. **RBAC**: The service uses minimal RBAC permissions - review and adjust as needed

## Next Steps

- Set up monitoring and alerting for build failures
- Configure retention policies for build jobs
- Set up automatic cleanup of old images
- Consider adding webhook authentication logs to a central logging system
- Implement rate limiting if handling many webhooks

## Additional Resources

- [GitHub Webhooks Documentation](https://docs.github.com/en/webhooks)
- [Kubernetes Jobs Documentation](https://kubernetes.io/docs/concepts/workloads/controllers/job/)
- [BuildKit Documentation](https://github.com/moby/buildkit)
- [GHCR Documentation](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)

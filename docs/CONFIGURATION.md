# Repository Configuration Guide

This guide explains how to customize your builds using a `.build-app.yaml` configuration file in your repository.

## Overview

Build App supports repository-level configuration through a `.build-app.yaml` file placed at the root of your repository. This allows you to:

- Pass build arguments to your Dockerfile
- Set environment variables during the build
- Specify custom Dockerfile locations
- Define build context paths (useful for monorepos)
- Target specific stages in multi-stage builds
- Specify target platforms

## Quick Start

Create a `.build-app.yaml` file in the root of your repository:

```yaml
buildArgs:
  VERSION: "1.0.0"
env:
  NODE_ENV: "production"
```

That's it! The next time you push or create a PR, the build service will use these settings.

## Configuration Reference

### buildArgs

Build arguments are passed to Docker/BuildKit using `--build-arg` flag. These can be referenced in your Dockerfile using `ARG` instructions.

```yaml
buildArgs:
  VERSION: "1.0.0"
  BUILD_DATE: "2024-01-01"
  GO_VERSION: "1.23"
```

**In your Dockerfile:**
```dockerfile
ARG VERSION
ARG BUILD_DATE

FROM golang:1.23
LABEL version=${VERSION}
LABEL build_date=${BUILD_DATE}
```

### env

Environment variables are set in the BuildKit container during the build process.

```yaml
env:
  NODE_ENV: "production"
  DEBUG: "false"
  LOG_LEVEL: "info"
```

### dockerfile

Specify a custom path to your Dockerfile (relative to repository root).

```yaml
dockerfile: docker/Dockerfile.prod
```

**Default:** `Dockerfile`

### context

Specify the build context directory (relative to repository root). This is useful for monorepos where your application is in a subdirectory.

```yaml
context: services/api
```

**Default:** `.` (repository root)

### target

For multi-stage Dockerfiles, specify which stage to build.

```yaml
target: production
```

**Example multi-stage Dockerfile:**
```dockerfile
FROM node:18 AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:18 AS production
WORKDIR /app
COPY --from=builder /app/dist ./dist
CMD ["node", "dist/index.js"]
```

With `target: production`, only the production stage will be built.

### platform

Specify the target platform for the build.

```yaml
platform: linux/amd64
```

**Common values:**
- `linux/amd64` - Intel/AMD 64-bit
- `linux/arm64` - ARM 64-bit (Apple Silicon, AWS Graviton)
- `linux/arm/v7` - ARM 32-bit

## Complete Example

```yaml
# .build-app.yaml
buildArgs:
  VERSION: "2.1.0"
  ENVIRONMENT: "staging"
  NODE_VERSION: "18"

env:
  NODE_ENV: "production"
  LOG_LEVEL: "debug"
  API_ENDPOINT: "https://api.staging.example.com"

dockerfile: build/Dockerfile.optimized
context: src
target: production
platform: linux/amd64
```

## Use Cases

### 1. Version Stamping

Inject version information into your builds:

```yaml
buildArgs:
  VERSION: "1.2.3"
  BUILD_DATE: "2024-01-15"
  GIT_COMMIT: "abc123"
```

### 2. Monorepo Builds

Build from a specific service directory:

```yaml
context: services/api
dockerfile: services/api/Dockerfile
buildArgs:
  SERVICE_NAME: "api"
```

### 3. Multi-Environment Builds

Different configurations for different branches:

**On main branch (.build-app.yaml):**
```yaml
buildArgs:
  ENVIRONMENT: "production"
env:
  NODE_ENV: "production"
target: production
```

**On develop branch (.build-app.yaml):**
```yaml
buildArgs:
  ENVIRONMENT: "staging"
env:
  NODE_ENV: "development"
  DEBUG: "true"
target: development
```

### 4. Multi-Architecture Builds

Build for ARM architecture:

```yaml
platform: linux/arm64
buildArgs:
  TARGETARCH: "arm64"
```

### 5. Private Registry Authentication

Pass registry credentials as build args:

```yaml
buildArgs:
  NPM_TOKEN: "${NPM_TOKEN}"
  GITHUB_TOKEN: "${GITHUB_TOKEN}"
```

**Note:** Sensitive values should be handled carefully. Consider using GitHub Secrets or other secure methods.

## Best Practices

### 1. Keep Sensitive Data Out

Never commit secrets or sensitive data to `.build-app.yaml`. Instead, use:
- GitHub Secrets (passed via environment variables)
- Build-time secret mounting (BuildKit secrets)
- External secret management systems

### 2. Document Your Configuration

Add comments to explain non-obvious settings:

```yaml
# Version is automatically set by CI/CD pipeline
buildArgs:
  VERSION: "1.0.0"
  
# Target production stage for optimized build
target: production
```

### 3. Use Defaults Wisely

Only specify options that differ from defaults to keep configuration minimal:

```yaml
# Good - only non-default settings
buildArgs:
  VERSION: "1.0.0"

# Unnecessary - these are defaults
# dockerfile: Dockerfile
# context: .
```

### 4. Test Locally First

Test your configuration locally before committing:

```bash
# Simulate the build command
buildctl build \
  --frontend=dockerfile.v0 \
  --local context=. \
  --local dockerfile=. \
  --opt build-arg:VERSION=1.0.0 \
  --output type=image,name=myimage:latest
```

### 5. Version Your Configuration

Track changes to `.build-app.yaml` in version control to understand how builds evolved over time.

## Troubleshooting

### Build Args Not Working

**Problem:** Build arguments aren't being applied.

**Solution:** Ensure your Dockerfile declares the ARG:

```dockerfile
ARG VERSION
# Must come before using it
RUN echo "Building version ${VERSION}"
```

### Custom Dockerfile Not Found

**Problem:** Build fails with "dockerfile not found".

**Solution:** Verify the path is relative to repository root:

```yaml
# Correct
dockerfile: docker/Dockerfile

# Wrong
dockerfile: /docker/Dockerfile
```

### Context Issues

**Problem:** Files not found during build.

**Solution:** Ensure the context path is correct and contains all necessary files:

```yaml
context: src
dockerfile: src/Dockerfile  # Dockerfile must be in context
```

### Environment Variables Not Set

**Problem:** Environment variables aren't available in the build.

**Solution:** Environment variables in `.build-app.yaml` are set in the BuildKit container, not in the resulting image. To set variables in the image, use `ENV` in your Dockerfile or pass them as build args.

## Migration from No Configuration

If you're adding configuration to an existing project:

1. Start with minimal configuration (just build args if needed)
2. Test thoroughly
3. Gradually add more options as needed

**Before (no config):**
- Build uses default Dockerfile at root
- No build args
- Context is repository root

**After (with config):**
```yaml
buildArgs:
  VERSION: "1.0.0"
```

## FAQ

**Q: Is the configuration file required?**  
A: No, it's optional. Without it, builds use default behavior.

**Q: Can I have different configs per branch?**  
A: Yes! Different branches can have different `.build-app.yaml` files. The service uses the file from the branch being built.

**Q: What happens if the config file is invalid?**  
A: The build will fail with an error message explaining what's wrong with the configuration.

**Q: Can I use environment variables in the config?**  
A: Build args can reference environment variables using `${VAR_NAME}` syntax, but the environment variables must be available in the BuildKit container.

**Q: Does this work with auto-generated Dockerfiles?**  
A: Yes! Even if the service generates a Dockerfile (for Go or Node.js projects), build args and other settings still apply.

## Examples

See the [`examples/`](../examples/) directory for ready-to-use configuration examples:

- `.build-app.yaml.basic` - Simple configuration
- `.build-app.yaml.advanced` - All available options
- `.build-app.yaml.monorepo` - Monorepo setup
- `.build-app.yaml.multistage` - Multi-stage builds

## Support

For issues or questions:
1. Check this documentation
2. Review example configurations
3. Open an issue on GitHub with your `.build-app.yaml` and error logs

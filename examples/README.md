# Configuration Examples

This directory contains example `.build-app.yaml` configuration files demonstrating different use cases.

## Usage

Copy one of these examples to the root of your repository as `.build-app.yaml` and customize as needed.

## Examples

### Basic Configuration (`.build-app.yaml.basic`)

Simple build arguments and environment variables.

```yaml
buildArgs:
  VERSION: "1.0.0"
  BUILD_DATE: "2024-01-01"

env:
  NODE_ENV: "production"
  DEBUG: "false"
```

**Use Case:** Adding version information and environment configuration to your builds.

---

### Advanced Configuration (`.build-app.yaml.advanced`)

All available configuration options.

```yaml
buildArgs:
  VERSION: "2.0.0"
  ENVIRONMENT: "staging"
  GO_VERSION: "1.23"

env:
  LOG_LEVEL: "debug"
  API_ENDPOINT: "https://api.staging.example.com"

dockerfile: build/Dockerfile.prod
context: src
target: production
platform: linux/amd64
```

**Use Case:** Full customization with custom Dockerfile location, build context, target stage, and platform.

---

### Monorepo Configuration (`.build-app.yaml.monorepo`)

Building from a subdirectory in a monorepo.

```yaml
buildArgs:
  SERVICE_NAME: "api-service"
  VERSION: "1.0.0"

context: services/api
dockerfile: services/api/Dockerfile

env:
  SERVICE_ENV: "production"
```

**Use Case:** When your application code is in a subdirectory (common in monorepos).

---

### Multi-Stage Build (`.build-app.yaml.multistage`)

Targeting a specific stage in a multi-stage Dockerfile.

```yaml
buildArgs:
  NODE_VERSION: "18"
  NPM_TOKEN: "${NPM_TOKEN}"

target: builder

env:
  NODE_ENV: "production"
```

**Use Case:** Building only a specific stage from a multi-stage Dockerfile (e.g., just the builder stage for caching).

---

## Testing Your Configuration

Before committing, you can test your configuration locally by examining what command would be generated:

1. Place your `.build-app.yaml` in the repo root
2. The build service will:
   - Load the configuration
   - Generate appropriate BuildKit commands
   - Pass build args, set env vars, and use custom paths

## Configuration Reference

For complete documentation, see [../docs/CONFIGURATION.md](../docs/CONFIGURATION.md).

### Available Options

| Option | Type | Description | Default |
|--------|------|-------------|---------|
| `buildArgs` | map[string]string | Build arguments passed to Docker | - |
| `env` | map[string]string | Environment variables during build | - |
| `dockerfile` | string | Path to Dockerfile (relative to repo root) | `Dockerfile` |
| `context` | string | Build context path (relative to repo root) | `.` |
| `target` | string | Target stage in multi-stage build | - |
| `platform` | string | Target platform (e.g., `linux/amd64`) | - |

## Common Patterns

### Version Stamping

```yaml
buildArgs:
  VERSION: "1.2.3"
  GIT_COMMIT: "abc123"
  BUILD_TIME: "2024-01-15T10:30:00Z"
```

Reference in Dockerfile:
```dockerfile
ARG VERSION
ARG GIT_COMMIT
ARG BUILD_TIME

LABEL version=${VERSION}
LABEL git.commit=${GIT_COMMIT}
LABEL build.time=${BUILD_TIME}
```

### Environment-Specific Builds

Different configurations per branch/environment:

**main branch:**
```yaml
buildArgs:
  ENV: "production"
target: production
```

**develop branch:**
```yaml
buildArgs:
  ENV: "development"
  DEBUG: "true"
target: development
```

### Multi-Architecture

```yaml
platform: linux/arm64
buildArgs:
  TARGETARCH: "arm64"
```

## Tips

1. **Keep it minimal**: Only specify options that differ from defaults
2. **Document**: Add comments explaining non-obvious choices
3. **Version control**: Track changes to understand build evolution
4. **Test locally**: Validate your config before committing
5. **Secure secrets**: Never commit sensitive data; use secure methods

## Getting Help

- Full documentation: [../docs/CONFIGURATION.md](../docs/CONFIGURATION.md)
- Setup guide: [../SETUP.md](../SETUP.md)
- Main README: [../README.md](../README.md)

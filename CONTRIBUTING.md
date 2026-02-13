# Contributing to Build App

Thank you for your interest in contributing to Build App! This document provides guidelines and instructions for contributing.

## Code of Conduct

Be respectful, inclusive, and professional in all interactions.

## How to Contribute

### Reporting Bugs

If you find a bug, please open an issue with:
- Clear description of the problem
- Steps to reproduce
- Expected vs actual behavior
- Environment details (Kubernetes version, Go version, etc.)
- Relevant logs or error messages

### Suggesting Features

Feature requests are welcome! Please open an issue with:
- Clear description of the feature
- Use case and benefits
- Potential implementation approach (if you have ideas)

### Pull Requests

1. **Fork the repository** and create your branch from `main`
2. **Make your changes** following our coding standards
3. **Add tests** for any new functionality
4. **Update documentation** if needed
5. **Run tests** to ensure everything passes
6. **Submit a pull request**

## Development Setup

### Prerequisites

- Go 1.23 or later
- Docker (for building images)
- kubectl and access to a Kubernetes cluster (for testing)
- Git

### Local Development

1. Clone the repository:
   ```bash
   git clone https://github.com/networkengineer-cloud/build-app.git
   cd build-app
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Run tests:
   ```bash
   go test ./...
   ```

4. Build the application:
   ```bash
   go build -o build-app .
   ```

5. Run locally (requires environment variables):
   ```bash
   export WEBHOOK_SECRET=test-secret
   export GITHUB_TOKEN=your-token
   ./build-app
   ```

## Coding Standards

### Go Style

- Follow the official [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` to format code
- Use `golangci-lint` for linting
- Write clear, descriptive variable and function names
- Add comments for exported functions and types

### Testing

- Write unit tests for new functionality
- Aim for at least 80% code coverage
- Use table-driven tests where appropriate
- Mock external dependencies (Kubernetes API, GitHub API)

### Commits

- Write clear, descriptive commit messages
- Use present tense ("Add feature" not "Added feature")
- Reference issues in commits (e.g., "Fix #123")
- Keep commits focused and atomic

### Example Commit Message

```
Add support for custom Dockerfile paths

- Allow users to specify custom Dockerfile locations
- Update build strategy detection
- Add tests for custom paths
- Update documentation

Fixes #42
```

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run tests with race detector
go test -race ./...

# Run specific package tests
go test ./internal/webhook -v
```

### Writing Tests

- Place tests in `*_test.go` files
- Use descriptive test names
- Test both success and error cases
- Use subtests for related test cases

Example:

```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid input", "test", "result", false},
        {"invalid input", "", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := MyFunction(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("MyFunction() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("MyFunction() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Documentation

- Update README.md for user-facing changes
- Update SETUP.md for deployment-related changes
- Add inline comments for complex logic
- Update API documentation if adding/changing endpoints

## Release Process

Maintainers will handle releases following semantic versioning:
- Major version: Breaking changes
- Minor version: New features (backward compatible)
- Patch version: Bug fixes

## Questions?

Feel free to open an issue for any questions or clarifications needed.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

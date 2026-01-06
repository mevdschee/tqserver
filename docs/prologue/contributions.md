# Contribution Guide

- [Code of Conduct](#code-of-conduct)
- [How to Contribute](#how-to-contribute)
- [Development Setup](#development-setup)
- [Coding Standards](#coding-standards)
- [Git Workflow](#git-workflow)
- [Testing Requirements](#testing-requirements)
- [Documentation](#documentation)
- [Review Process](#review-process)

## Code of Conduct

### Our Pledge

We are committed to providing a welcoming and inclusive environment for everyone, regardless of gender, sexual orientation, disability, ethnicity, religion, or experience level.

### Our Standards

**Positive behavior includes**:
- Using welcoming and inclusive language
- Respecting differing viewpoints and experiences
- Gracefully accepting constructive criticism
- Focusing on what's best for the community
- Showing empathy towards other community members

**Unacceptable behavior includes**:
- Trolling, insulting, or derogatory comments
- Public or private harassment
- Publishing others' private information
- Other conduct inappropriate in a professional setting

### Enforcement

Violations can be reported to conduct@tqserver.dev. All complaints will be reviewed and investigated promptly and fairly.

## How to Contribute

There are many ways to contribute to TQServer:

### Reporting Bugs

Found a bug? Please create an issue with:

1. **Clear title** - Describe the problem concisely
2. **Environment** - TQServer version, Go version, OS
3. **Steps to reproduce** - Minimal reproduction case
4. **Expected behavior** - What should happen
5. **Actual behavior** - What actually happens
6. **Logs** - Relevant error messages or stack traces
7. **Screenshots** - If applicable

### Suggesting Features

Have an idea? Create a feature request with:

1. **Use case** - Why this feature is needed
2. **Proposed solution** - How it might work
3. **Alternatives** - Other approaches considered
4. **Additional context** - Examples, mockups, etc.

### Writing Code

Ready to contribute code? Great!

1. Check [existing issues](https://github.com/mevdschee/tqserver/issues)
2. Comment on an issue to claim it
3. Fork the repository
4. Create a feature branch
5. Write code and tests
6. Submit a pull request

### Improving Documentation

Documentation improvements are always welcome:

- Fix typos or unclear explanations
- Add examples
- Improve existing guides
- Translate documentation
- Create tutorials

### Helping Others

- Answer questions on GitHub Issues
- Help on Discord
- Write blog posts or tutorials
- Share your experience

## Development Setup

### Prerequisites

- **Go 1.24+** - Required for building
- **Git** - For version control
- **Make** - For build automation (optional)
- **Docker** - For testing (optional)

### Clone the Repository

```bash
# Fork the repository on GitHub first, then:
git clone https://github.com/YOUR_USERNAME/tqserver.git
cd tqserver

# Add upstream remote
git remote add upstream https://github.com/mevdschee/tqserver.git
```

### Install Dependencies

```bash
# Download Go dependencies
go mod download

# Verify dependencies
go mod verify
```

### Build from Source

```bash
# Build server
go build -o bin/tqserver ./cmd/tqserver

# Or use the build script
./scripts/build-dev.sh

# Run tests
go test ./...
```

### Running Locally

```bash
# Start the server
./bin/tqserver

# Or with custom config
./bin/tqserver -config=config/server.yaml

# In another terminal, test
curl http://localhost:8080/
```

### Development Tools

Recommended tools for development:

```bash
# Install golangci-lint for linting
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Install gopls for IDE support
go install golang.org/x/tools/gopls@latest

# Install gofumpt for formatting
go install mvdan.cc/gofumpt@latest

# Install staticcheck for static analysis
go install honnef.co/go/tools/cmd/staticcheck@latest
```

## Coding Standards

### Go Style Guide

Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments):

- Use `gofmt` (or `gofumpt`) for formatting
- Follow effective Go guidelines
- Write idiomatic Go code
- Keep functions small and focused
- Use meaningful variable names

### Code Organization

```go
// Package declaration and imports
package mypackage

import (
    "fmt"
    "log"
    
    "github.com/external/package"
    
    "github.com/mevdschee/tqserver/internal/config"
)

// Constants
const (
    DefaultTimeout = 30 * time.Second
)

// Types
type Server struct {
    config *config.Config
}

// Constructor
func NewServer(cfg *config.Config) *Server {
    return &Server{config: cfg}
}

// Methods
func (s *Server) Start() error {
    // Implementation
}
```

### Error Handling

```go
// Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to start server: %w", err)
}

// Check errors explicitly
result, err := doSomething()
if err != nil {
    return err
}

// Don't ignore errors
_ = file.Close() // Bad
if err := file.Close(); err != nil {
    log.Printf("failed to close file: %v", err)
}
```

### Logging

```go
// Use standard log package or structured logging
log.Printf("Starting server on %s:%d", host, port)

// Include context in error logs
log.Printf("Failed to start worker %s: %v", workerName, err)

// Don't log sensitive information
log.Printf("User authenticated: %s", userID) // OK
log.Printf("Password: %s", password) // NEVER
```

### Comments

```go
// Package comment describes the package
// Package supervisor manages worker processes.
package supervisor

// Exported types and functions need comments
// Worker represents a running worker process.
type Worker struct {
    Name string
    Port int
}

// NewWorker creates a new worker instance.
func NewWorker(name string) *Worker {
    return &Worker{Name: name}
}

// Internal comments explain non-obvious logic
func (w *Worker) restart() error {
    // We must stop the old worker before starting the new one
    // to avoid port conflicts during the transition period
    if err := w.stop(); err != nil {
        return err
    }
    return w.start()
}
```

### Testing

```go
// Test file: worker_test.go
package supervisor

import "testing"

func TestWorkerStart(t *testing.T) {
    // Arrange
    worker := NewWorker("test")
    
    // Act
    err := worker.Start()
    
    // Assert
    if err != nil {
        t.Errorf("Expected no error, got %v", err)
    }
}

// Table-driven tests for multiple cases
func TestWorkerValidation(t *testing.T) {
    tests := []struct {
        name    string
        worker  string
        wantErr bool
    }{
        {"valid", "api", false},
        {"empty", "", true},
        {"invalid chars", "api@123", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validateWorker(tt.worker)
            if (err != nil) != tt.wantErr {
                t.Errorf("wantErr %v, got %v", tt.wantErr, err)
            }
        })
    }
}
```

## Git Workflow

### Branching Strategy

- `main` - Stable release branch
- `develop` - Development branch
- `feature/*` - New features
- `fix/*` - Bug fixes
- `docs/*` - Documentation updates
- `refactor/*` - Code refactoring

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation
- `style`: Formatting
- `refactor`: Code restructuring
- `test`: Tests
- `chore`: Maintenance

**Examples**:
```
feat(supervisor): add worker restart timeout

Add configurable timeout for worker restarts to prevent
hanging during shutdown.

Closes #123
```

```
fix(proxy): handle connection errors gracefully

Previously, connection errors would crash the proxy.
Now they are logged and return 502 to the client.

Fixes #456
```

### Pull Request Process

1. **Create a feature branch**
   ```bash
   git checkout -b feature/my-awesome-feature
   ```

2. **Make your changes**
   ```bash
   # Write code
   # Write tests
   # Update documentation
   ```

3. **Commit with clear messages**
   ```bash
   git add .
   git commit -m "feat(router): add middleware support"
   ```

4. **Keep your branch updated**
   ```bash
   git fetch upstream
   git rebase upstream/develop
   ```

5. **Push to your fork**
   ```bash
   git push origin feature/my-awesome-feature
   ```

6. **Create pull request**
   - Go to GitHub
   - Click "New Pull Request"
   - Fill out the template
   - Link related issues

### Pull Request Template

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
- [ ] Tests added/updated
- [ ] All tests passing
- [ ] Manual testing done

## Checklist
- [ ] Code follows style guidelines
- [ ] Self-review completed
- [ ] Comments added for complex logic
- [ ] Documentation updated
- [ ] No new warnings
```

## Testing Requirements

### Unit Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Integration Tests

```bash
# Run integration tests
go test -tags=integration ./...

# Or use make
make test-integration
```

### Benchmarks

```bash
# Run benchmarks
go test -bench=. ./...

# Save baseline
go test -bench=. ./... > old.txt

# Compare after changes
go test -bench=. ./... > new.txt
benchcmp old.txt new.txt
```

### Test Coverage Requirements

- New features: 80% coverage minimum
- Bug fixes: Add test reproducing the bug
- Critical paths: 100% coverage
- Total project: Maintain current coverage

## Documentation

### Code Documentation

- All exported types need comments
- All exported functions need comments
- Complex algorithms need explanation
- Examples for public APIs

### User Documentation

When adding features:

1. Update relevant documentation pages
2. Add code examples
3. Update configuration reference
4. Add to changelog
5. Update migration guides if breaking

### Documentation Style

- Write in present tense
- Use active voice
- Be concise but complete
- Include practical examples
- Link to related docs

## Review Process

### What We Look For

- **Correctness**: Does it work as intended?
- **Testing**: Are there adequate tests?
- **Documentation**: Is it documented?
- **Style**: Does it follow conventions?
- **Performance**: Are there performance concerns?
- **Security**: Are there security implications?

### Review Timeline

- Initial review: Within 2 business days
- Follow-up reviews: Within 1 business day
- Approval required: At least 1 maintainer
- Merge: After approval and CI passes

### Addressing Review Feedback

```bash
# Make requested changes
git add .
git commit -m "address review feedback"

# Push changes
git push origin feature/my-awesome-feature
```

### After Merge

- Delete your feature branch
- Pull latest changes
- Close related issues
- Celebrate! 🎉

## Recognition

Contributors are recognized in:

- README.md contributors section
- Release notes
- Annual contributor highlights
- Project website

## Questions?

- **Discord**: Join our community
- **Email**: dev@tqserver.dev
- **GitHub Discussions**: For general questions

## License

By contributing, you agree that your contributions will be licensed under the same license as the project.

Thank you for contributing to TQServer! 🚀

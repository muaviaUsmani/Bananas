# Contributing to Bananas

Thank you for your interest in contributing to Bananas! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Development Workflow](#development-workflow)
- [Testing](#testing)
- [Code Style](#code-style)
- [Documentation](#documentation)
- [Pull Request Process](#pull-request-process)
- [Reporting Issues](#reporting-issues)
- [Feature Requests](#feature-requests)

---

## Code of Conduct

### Our Standards

- Be respectful and inclusive
- Welcome newcomers and help them learn
- Accept constructive criticism gracefully
- Focus on what is best for the community
- Show empathy towards other community members

### Unacceptable Behavior

- Harassment, trolling, or discriminatory comments
- Publishing others' private information
- Other unethical or unprofessional conduct

---

## Getting Started

### Prerequisites

- **Go 1.21+** ([installation guide](https://golang.org/doc/install))
- **Docker & Docker Compose** ([installation guide](https://docs.docker.com/get-docker/))
- **Git** ([installation guide](https://git-scm.com/downloads))
- **Redis** (via Docker or local installation)

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork:
```bash
git clone https://github.com/YOUR_USERNAME/Bananas.git
cd Bananas
```

3. Add upstream remote:
```bash
git remote add upstream https://github.com/muaviaUsmani/Bananas.git
```

4. Verify remotes:
```bash
git remote -v
# origin    https://github.com/YOUR_USERNAME/Bananas.git (fetch)
# origin    https://github.com/YOUR_USERNAME/Bananas.git (push)
# upstream  https://github.com/muaviaUsmani/Bananas.git (fetch)
# upstream  https://github.com/muaviaUsmani/Bananas.git (push)
```

---

## Development Setup

### Quick Start

```bash
# Start development environment with hot reload
make dev

# In another terminal, run tests
make test

# Check code style
make lint
```

### Manual Setup

1. **Install dependencies:**
```bash
go mod download
```

2. **Start Redis:**
```bash
docker run -d -p 6379:6379 redis:7-alpine
```

3. **Run tests:**
```bash
go test ./...
```

4. **Build binaries:**
```bash
make build
# or
go build -o bin/worker cmd/worker/main.go
go build -o bin/api cmd/api/main.go
go build -o bin/scheduler cmd/scheduler/main.go
```

### Project Structure

```
Bananas/
├── cmd/                  # Entry points (main packages)
│   ├── api/             # API server
│   ├── worker/          # Worker process
│   └── scheduler/       # Scheduler process
├── internal/            # Private application code
│   ├── config/          # Configuration management
│   ├── job/             # Job types and utilities
│   ├── queue/           # Queue implementation
│   ├── worker/          # Worker pool and execution
│   ├── scheduler/       # Scheduling logic
│   ├── result/          # Result backend
│   ├── metrics/         # Metrics collection
│   └── logger/          # Logging utilities
├── pkg/                 # Public SDK (client library)
│   └── client/          # Client for job submission
├── docs/                # Documentation
├── examples/            # Example applications
├── tests/               # Integration tests
└── proto/               # Protocol buffer definitions
```

---

## Development Workflow

### Branching Strategy

We use **GitHub Flow** (simplified Git Flow):

1. `main` branch is always deployable
2. Create feature branches from `main`
3. Merge via pull requests with review
4. Deploy from `main`

### Creating a Feature Branch

```bash
# Update main
git checkout main
git pull upstream main

# Create feature branch
git checkout -b feature/my-awesome-feature

# Or for bug fixes
git checkout -b fix/issue-123
```

### Making Changes

1. **Write code** following our [code style](#code-style)
2. **Add tests** for new functionality
3. **Update documentation** if needed
4. **Run tests** to ensure nothing breaks
5. **Commit changes** with clear messages

```bash
# Stage changes
git add .

# Commit with descriptive message
git commit -m "Add task routing for GPU jobs

- Implement routing key support in Job struct
- Add DequeueWithRouting method
- Update worker config for routing keys
- Add comprehensive tests

Closes #123"
```

### Commit Message Guidelines

**Format:**
```
<type>: <subject>

<body>

<footer>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Adding tests
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `chore`: Maintenance tasks

**Example:**
```
feat: Add support for periodic tasks with cron scheduling

Implement cron-based task scheduling with distributed locking
to prevent duplicate execution across multiple scheduler instances.

Features:
- Cron expression parsing (5-field format)
- Timezone support (IANA timezones)
- Distributed locking via Redis
- Schedule state persistence

Closes #45
```

### Syncing with Upstream

```bash
# Fetch upstream changes
git fetch upstream

# Rebase your feature branch
git rebase upstream/main

# If conflicts, resolve and continue
git rebase --continue

# Force push to your fork
git push origin feature/my-awesome-feature --force
```

---

## Testing

### Running Tests

```bash
# Run all tests
make test

# Run with verbose output
make test-verbose

# Run specific package tests
go test ./internal/queue/... -v

# Run specific test
go test ./internal/queue -run TestRedisQueue_Enqueue -v

# Run with coverage
make test-coverage

# Run integration tests only
go test ./tests/... -v
```

### Writing Tests

**Unit Test Example:**
```go
package job

import "testing"

func TestNewJob(t *testing.T) {
    payload := []byte(`{"key":"value"}`)
    j := NewJob("test_job", payload, PriorityNormal)

    if j == nil {
        t.Fatal("expected job to be created")
    }
    if j.Name != "test_job" {
        t.Errorf("expected name 'test_job', got '%s'", j.Name)
    }
    if j.Priority != PriorityNormal {
        t.Errorf("expected priority %s, got %s", PriorityNormal, j.Priority)
    }
}
```

**Table-Driven Test Example:**
```go
func TestValidateRoutingKey(t *testing.T) {
    tests := []struct {
        name      string
        key       string
        wantError bool
    }{
        {"valid simple", "gpu", false},
        {"valid with underscore", "high_memory", false},
        {"valid with hyphen", "us-east-1", false},
        {"invalid empty", "", true},
        {"invalid too long", strings.Repeat("a", 65), true},
        {"invalid special char", "gpu@worker", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateRoutingKey(tt.key)
            if (err != nil) != tt.wantError {
                t.Errorf("ValidateRoutingKey(%q) error = %v, wantError %v",
                    tt.key, err, tt.wantError)
            }
        })
    }
}
```

**Integration Test Example:**
```go
func TestTaskRouting_BasicRouting(t *testing.T) {
    // Setup
    q, client := setupTestQueue(t)
    defer q.Close()
    defer client.Close()

    ctx := context.Background()

    // Create and enqueue GPU job
    gpuJob := job.NewJob("process_image", []byte(`{}`), job.PriorityNormal)
    gpuJob.SetRoutingKey("gpu")
    q.Enqueue(ctx, gpuJob)

    // GPU worker should get GPU job
    dequeuedJob, err := q.DequeueWithRouting(ctx, []string{"gpu"})
    if err != nil {
        t.Fatalf("failed to dequeue: %v", err)
    }
    if dequeuedJob.RoutingKey != "gpu" {
        t.Errorf("expected routing key 'gpu', got '%s'", dequeuedJob.RoutingKey)
    }

    // Cleanup
    q.Complete(ctx, dequeuedJob.ID)
}
```

### Test Coverage Requirements

- **Target:** 90%+ overall coverage
- **Minimum:** 80% for new code
- **Critical paths:** 100% coverage (queue operations, job execution)

**Check coverage:**
```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

---

## Code Style

### Go Style Guidelines

We follow the [Effective Go](https://golang.org/doc/effective_go) guidelines and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).

**Key principles:**
- Use `gofmt` for formatting (automatically applied)
- Use `golangci-lint` for linting
- Write clear, self-documenting code
- Add comments for exported functions/types
- Keep functions small and focused
- Handle errors explicitly

### Formatting

```bash
# Format code
gofmt -s -w .

# Or use goimports (auto-adds/removes imports)
goimports -w .
```

### Linting

```bash
# Run linter
golangci-lint run

# Auto-fix issues
golangci-lint run --fix
```

### Code Examples

**Good:**
```go
// ProcessImage resizes and optimizes an image for web display.
// Returns the URL of the processed image or an error if processing fails.
func ProcessImage(ctx context.Context, imageURL string, width, height int) (string, error) {
    if imageURL == "" {
        return "", errors.New("image URL cannot be empty")
    }

    img, err := downloadImage(ctx, imageURL)
    if err != nil {
        return "", fmt.Errorf("download failed: %w", err)
    }

    resized := resize(img, width, height)
    optimized := optimize(resized)

    url, err := uploadImage(ctx, optimized)
    if err != nil {
        return "", fmt.Errorf("upload failed: %w", err)
    }

    return url, nil
}
```

**Bad:**
```go
// bad naming, no error wrapping, no documentation
func process(u string, w int, h int) (string, error) {
    img, _ := downloadImage(context.Background(), u) // ignoring error!
    return uploadImage(context.Background(), resize(img, w, h))
}
```

### Naming Conventions

**Variables:**
```go
// Good
userID := "123"
maxRetries := 3
isProcessing := true

// Bad
UserId := "123"        // unexported vars shouldn't be capitalized
max_retries := 3       // use camelCase, not snake_case
processing := true     // unclear boolean name
```

**Functions:**
```go
// Good
func GetJob(id string) (*Job, error)
func ValidateRoutingKey(key string) error
func NewRegistry() *Registry

// Bad
func get_job(id string) (*Job, error)      // use camelCase
func validate(key string) error             // not descriptive enough
func CreateNewRegistry() *Registry         // redundant "New"
```

**Interfaces:**
```go
// Good
type Reader interface { /* ... */ }
type Handler interface { /* ... */ }

// Bad
type IReader interface { /* ... */ }  // don't prefix with "I"
type ReaderInterface interface { /* ... */ }  // don't suffix with "Interface"
```

---

## Documentation

### Code Documentation

**Document all exported types and functions:**
```go
// Job represents a unit of work to be processed by the task queue.
//
// Jobs are created by clients and submitted to queues, where they are
// picked up by workers for execution. Each job has a unique ID, priority,
// and routing key that determines which worker pool processes it.
type Job struct {
    // ID is the unique identifier for the job (UUID format)
    ID string `json:"id"`

    // Name identifies the handler that will process this job
    Name string `json:"name"`

    // Priority determines the processing order (high > normal > low)
    Priority JobPriority `json:"priority"`

    // ...
}
```

**Document non-obvious behavior:**
```go
// Dequeue retrieves a job from the highest priority non-empty queue.
//
// This method blocks until a job is available or the context is cancelled.
// Jobs are returned in strict priority order: high, normal, then low.
//
// If the context has a deadline, it will be respected. The method returns
// nil (job) if the context is cancelled or deadline exceeded.
func (q *RedisQueue) Dequeue(ctx context.Context, priorities []JobPriority) (*Job, error) {
    // implementation
}
```

### Markdown Documentation

When adding or updating documentation:

1. Use clear, concise language
2. Include code examples
3. Add diagrams where helpful (ASCII art is fine)
4. Link to related documentation
5. Keep formatting consistent

**Example:**
```markdown
# Task Routing

Task routing allows jobs to be directed to specific worker pools based on routing keys.

## Quick Example

```go
// Submit GPU job
client.SubmitJobWithRoute("process_image", payload, job.PriorityHigh, "gpu")

// Configure GPU worker
WORKER_ROUTING_KEYS=gpu ./worker
```

## See Also

- [Architecture Overview](./ARCHITECTURE.md)
- [API Reference](./API_REFERENCE.md)
```

---

## Pull Request Process

### Before Submitting

- [ ] Code follows style guidelines
- [ ] Tests added for new functionality
- [ ] All tests pass (`make test`)
- [ ] Documentation updated
- [ ] Commit messages are clear
- [ ] Branch is up to date with main

### Creating a Pull Request

1. **Push to your fork:**
```bash
git push origin feature/my-awesome-feature
```

2. **Create PR on GitHub:**
   - Go to https://github.com/muaviaUsmani/Bananas
   - Click "New Pull Request"
   - Select your branch
   - Fill in the template

### PR Template

```markdown
## Description

Brief description of what this PR does.

## Type of Change

- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature (non-breaking change that adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update

## Changes Made

- Change 1
- Change 2
- Change 3

## Testing

Describe the tests you added and how to run them.

## Related Issues

Closes #123
Related to #456

## Checklist

- [ ] Code follows style guidelines
- [ ] Self-review completed
- [ ] Comments added for complex code
- [ ] Documentation updated
- [ ] Tests added
- [ ] All tests pass
- [ ] No new warnings
```

### Review Process

1. **Automated checks** run (tests, linting)
2. **Maintainer reviews** code
3. **Address feedback** if requested
4. **Approval** from maintainer
5. **Merge** to main

### After Merge

```bash
# Update your main branch
git checkout main
git pull upstream main

# Delete feature branch
git branch -d feature/my-awesome-feature
git push origin --delete feature/my-awesome-feature
```

---

## Reporting Issues

### Bug Reports

**Use the issue template:**

```markdown
## Bug Description

Clear description of the bug.

## Steps to Reproduce

1. Step 1
2. Step 2
3. Step 3

## Expected Behavior

What should happen.

## Actual Behavior

What actually happens.

## Environment

- OS: Ubuntu 22.04
- Go version: 1.21.0
- Bananas version: v0.1.0
- Redis version: 7.0.0

## Logs

```
Relevant log output
```

## Additional Context

Any other relevant information.
```

### Security Issues

**Do NOT open public issues for security vulnerabilities.**

Instead, email: security@example.com

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

---

## Feature Requests

### Proposing New Features

1. **Check existing issues** - Feature might already be proposed
2. **Open a discussion** - Describe your use case
3. **Get feedback** - Discuss implementation approach
4. **Create proposal** - Detailed design document
5. **Implement** - Follow development workflow

### Feature Proposal Template

```markdown
## Feature Description

Clear description of the proposed feature.

## Motivation

Why is this feature needed? What problem does it solve?

## Proposed Solution

How should this be implemented?

## Alternatives Considered

What other approaches did you consider?

## Additional Context

Any other relevant information.
```

---

## Development Tips

### Useful Commands

```bash
# Quick test specific package
make test-quick PACKAGE=./internal/queue

# Run with race detector
go test -race ./...

# Profile CPU usage
go test -cpuprofile=cpu.prof ./internal/queue
go tool pprof cpu.prof

# Check for goroutine leaks
go test -run TestName -v -count=1 -timeout=30s

# Generate mocks (if using mockgen)
mockgen -source=internal/queue/queue.go -destination=mocks/queue_mock.go
```

### Debugging

**Delve debugger:**
```bash
# Install
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug test
dlv test ./internal/queue -- -test.run TestDequeue

# Set breakpoint
(dlv) break queue.go:123
(dlv) continue
(dlv) print job
```

**Logging:**
```go
import "log"

log.Printf("Debug: job=%+v\n", job)
```

### Common Pitfalls

1. **Don't ignore errors:**
```go
// Bad
result, _ := someFunction()

// Good
result, err := someFunction()
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}
```

2. **Close resources:**
```go
// Good
client, err := redis.NewClient(opts)
if err != nil {
    return err
}
defer client.Close()
```

3. **Use context for cancellation:**
```go
// Good
func ProcessWithTimeout(ctx context.Context, data string) error {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    select {
    case <-ctx.Done():
        return ctx.Err()
    case result := <-process(data):
        return result
    }
}
```

---

## Getting Help

### Resources

- **Documentation:** [docs/](./docs/)
- **Examples:** [examples/](./examples/)
- **GitHub Issues:** https://github.com/muaviaUsmani/Bananas/issues
- **Discussions:** https://github.com/muaviaUsmani/Bananas/discussions

### Ask Questions

Don't hesitate to ask questions! We're here to help:

1. Check existing documentation
2. Search closed issues
3. Open a GitHub discussion
4. Ask in pull request comments

---

## License

By contributing to Bananas, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to Bananas! 🍌

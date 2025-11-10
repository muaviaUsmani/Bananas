# Bananas - Distributed Task Queue

A distributed task queue system built with Go, Redis, and Docker.

## Description

Bananas is a lightweight, scalable distributed task queue system designed to handle asynchronous job processing across multiple workers. It uses Redis as the message broker and provides a simple API for job submission and management.

## ✨ Features

- **Task Routing** - Direct jobs to specific worker pools (GPU, email, regions) for resource isolation and independent scaling
- **Priority Queues** - High, normal, and low priority processing with strict ordering
- **Scheduled Jobs** - Schedule jobs for future execution with exponential backoff retry
- **Job Retry** - Automatic retry with configurable exponential backoff for failed jobs
- **Dead Letter Queue** - Failed jobs are moved to DLQ for inspection and recovery
- **Worker Modes** - Thin, default, specialized, and job-specialized modes for different use cases
- **Result Backend** - Store and retrieve job results with configurable TTL
- **Graceful Shutdown** - Workers complete in-progress jobs before stopping
- **Metrics & Monitoring** - Built-in metrics for queue depth, job counts, and worker utilization
- **Hot Reload** - Development mode with instant service restart on code changes

## 📖 Documentation

- **[Integration Guide](./INTEGRATION_GUIDE.md)** - Complete guide for integrating Bananas into your projects
- **[Task Routing Guide](./docs/TASK_ROUTING_USAGE.md)** - Route jobs to specific worker pools (GPU, email, regions)
- **[Architecture & Components](./README.md#architecture)** - System design and component overview
- **[API Documentation](./cmd/README.md)** - Service entry points (API, Worker, Scheduler)
- **[Internal Packages](./internal/README.md)** - Core packages (Config, Job, Queue, Worker)
- **[Client SDK](./pkg/README.md)** - Public client library documentation
- **[Tests](./tests/README.md)** - Integration test documentation

**New to Bananas?** Start with the [Integration Guide](./INTEGRATION_GUIDE.md) to learn how to add Bananas to your existing infrastructure.

## Getting Started

### Prerequisites

- Docker and Docker Compose

### Quick Start with Docker

There are two ways to run the system with Docker:

1. **Development mode** (with hot reload) - for active development
2. **Production mode** (optimized builds) - for testing production builds

## Development Mode (Recommended for Development)

Use the development setup for fast iteration with hot reload. Code changes are detected automatically and services restart instantly without rebuilding containers.

### Start Development Environment

```bash
make dev
# Or directly:
docker compose -f docker-compose.dev.yml up
```

This will start all services with:
- **Hot reload enabled** - code changes trigger automatic restarts
- **Source code mounted** - edit files on your host, changes reflect immediately
- **Fast feedback loop** - no container rebuilds needed

When you change any `.go` file:
- Only the affected service restarts (not all containers)
- Restart happens in ~1 second
- No Docker image rebuild required

### Stop Development Environment

```bash
make dev-down
# Or directly:
docker compose -f docker-compose.dev.yml down
```

## Production Mode

Use production mode to test optimized, production-ready builds.

### Build and Start All Services

```bash
make prod-build
# Or directly:
docker compose up --build
```

This will start:
- Redis server
- API server (accessible at http://localhost:8080)
- 3 worker instances (optimized binaries)
- 1 scheduler instance (optimized binaries)

### Scaling Workers

You can easily scale the number of workers:

```bash
make scale-workers N=5
# Or directly:
docker compose up --scale worker=5
```

### View Service Logs

View logs for a specific service:

```bash
# View worker logs
make logs-worker

# View API logs
make logs-api

# View all logs
make dev-logs
```

### Stop All Services

```bash
make down
# Or directly:
docker compose down
```

### Rebuild Production Images

After code changes, rebuild the optimized production images:

```bash
make prod-build
# Or directly:
docker compose up --build
```

### Run Individual Services

You can run individual services in either mode:

```bash
# Development mode - with hot reload
docker compose -f docker-compose.dev.yml up api

# Production mode - optimized builds
docker compose up api
```

### Available Make Commands

Run `make help` or just `make` to see all available commands:

```bash
make help
```

Quick reference:
- `make dev` - Start development environment
- `make prod-build` - Build and start production
- `make test` - Run all tests
- `make test-coverage` - Run tests with coverage report
- `make logs-worker` - View worker logs
- `make scale-workers N=5` - Scale workers
- `make clean` - Clean up everything

## Testing

### Running Tests

The project includes comprehensive test coverage (93.3%) across all components.

**Recommended: Run tests in Docker (works with any local Go version):**

```bash
# Run all tests in Docker container with Go 1.23
make test

# Run tests with verbose output in Docker
make test-verbose

# Generate coverage report in Docker
make test-coverage
```

**Alternative: Run tests locally (requires Go 1.21+):**

```bash
# Run tests on your local machine
make test-local

# Run tests with verbose output locally
make test-local-verbose

# Generate coverage report locally
make test-local-coverage
```

**Note**: The Docker-based test commands are recommended because:
- No local Go installation required
- Always uses the correct Go version (1.23)
- Consistent test environment
- Works on any machine with Docker

### Test Structure

- **Unit Tests**: Test individual components in isolation
  - `internal/job/types_test.go` - Job model tests (100% coverage)
  - `internal/worker/executor_test.go` - Job execution tests
  - `internal/worker/handler_test.go` - Handler registry tests
  - `internal/queue/redis_test.go` - Redis queue operations
  - `pkg/client/client_test.go` - Client SDK tests (95.2% coverage)

- **Integration Tests**: End-to-end workflow tests
  - `tests/integration_test.go` - Full system integration tests

## Redis Queue Implementation

Bananas uses Redis as its message broker with the following queue structure:

### Queue Keys

- **Job Storage**: `bananas:job:{id}` - Job data stored as JSON
- **Priority Queues**: 
  - `bananas:queue:high` - High priority jobs
  - `bananas:queue:normal` - Normal priority jobs
  - `bananas:queue:low` - Low priority jobs
- **Processing Queue**: `bananas:queue:processing` - Currently processing jobs
- **Dead Letter Queue**: `bananas:queue:dead` - Failed jobs (max retries exceeded)
- **Scheduled Set**: `bananas:queue:scheduled` - Jobs scheduled for future execution

### Queue Operations

**Enqueue**: Atomically stores job data and adds job ID to priority queue
```go
queue.Enqueue(ctx, job)
```

**Dequeue**: Atomically moves job from priority queue to processing queue (RPOPLPUSH)
```go
job, err := queue.Dequeue(ctx, []job.JobPriority{
    job.PriorityHigh,
    job.PriorityNormal,
    job.PriorityLow,
})
```

**Complete**: Updates job status and removes from processing queue
```go
queue.Complete(ctx, jobID)
```

**Fail**: Handles failures with exponential backoff retry logic
- If attempts < maxRetries: Schedules retry with exponential backoff (2^attempts seconds)
- If attempts >= maxRetries: Moves to dead letter queue
```go
queue.Fail(ctx, job, "error message")
```

**MoveScheduledToReady**: Moves ready jobs from scheduled set to priority queues
- Should be called periodically by scheduler (e.g., every second)
- Moves jobs whose retry time has arrived back to their priority queues
```go
count, err := queue.MoveScheduledToReady(ctx)
```

### Retry Strategy with Exponential Backoff

Failed jobs are not immediately re-enqueued. Instead, they're scheduled with exponential backoff:

- **1st retry**: 2 seconds delay (2^1)
- **2nd retry**: 4 seconds delay (2^2)
- **3rd retry**: 8 seconds delay (2^3)
- **Nth retry**: 2^N seconds delay

This approach prevents:
- **Thundering herd problems** when external services fail
- **Overwhelming failing dependencies** with retry storms
- **Priority queue pollution** with repeatedly failing jobs

Jobs awaiting retry are stored in a Redis sorted set (`bananas:queue:scheduled`) with their retry timestamp as the score. A background scheduler process periodically calls `MoveScheduledToReady()` to move ready jobs back to their priority queues.

### Features

**Queue Management:**
- ✅ Priority-based job processing (High > Normal > Low)
- ✅ Automatic retry with exponential backoff
- ✅ Scheduled set for delayed retry execution
- ✅ Dead letter queue for permanently failed jobs
- ✅ Atomic operations to prevent job loss
- ✅ Thread-safe operations

**Worker Pool:**
- ✅ Configurable concurrency (WORKER_CONCURRENCY env var)
- ✅ Per-job timeout enforcement (JOB_TIMEOUT env var)
- ✅ Graceful shutdown (waits for running jobs)
- ✅ Automatic job completion/failure tracking
- ✅ Context cancellation support
- ✅ Worker-level error handling

**Observability:**
- ✅ Comprehensive logging for all operations
- ✅ Job execution timing
- ✅ Retry attempt tracking

## Architecture Overview

### Components

- **API Server** (`cmd/api`): HTTP API for job submission and status queries
- **Worker** (`cmd/worker`): Processes jobs from Redis queue with configurable concurrency and graceful shutdown
- **Scheduler** (`cmd/scheduler`): Moves scheduled jobs to ready queues when their retry time arrives (runs every second)

### Worker Pool Architecture

The worker service uses a **concurrent worker pool** pattern:

1. **Pool Manager** (`internal/worker/pool.go`):
   - Manages N worker goroutines (configurable via `WORKER_CONCURRENCY`)
   - Each worker continuously dequeues jobs from Redis
   - Distributes work across workers for parallel processing
   - Implements graceful shutdown (waits for in-flight jobs)

2. **Executor** (`internal/worker/executor.go`):
   - Executes individual jobs by looking up registered handlers
   - Integrates with Redis queue for completion/failure tracking
   - Calls `queue.Complete()` on success
   - Calls `queue.Fail()` on error (triggers exponential backoff)
   - Enforces per-job timeout from configuration

3. **Handler Registry** (`internal/worker/handler.go`):
   - Maps job names to handler functions
   - Allows dynamic handler registration
   - Type-safe handler interface

**Example Worker Flow:**
```
Worker 1 → Dequeue → Execute → Complete/Fail → Dequeue → ...
Worker 2 → Dequeue → Execute → Complete/Fail → Dequeue → ...
Worker N → Dequeue → Execute → Complete/Fail → Dequeue → ...
```

### Internal Packages

- `internal/queue`: Queue management and Redis operations
- `internal/job`: Job definition and state management
- `internal/config`: Configuration management
- `internal/worker`: Worker pool, executor, and task execution logic

### Client Package

- `pkg/client`: Go client library for interacting with the Bananas API

## Project Structure

```
.
├── cmd/
│   ├── api/              # API server entry point
│   ├── worker/           # Worker entry point
│   └── scheduler/        # Scheduler entry point
├── internal/             # Private application code
│   ├── queue/            # Queue implementation
│   ├── job/              # Job types and management
│   ├── config/           # Configuration
│   └── worker/           # Worker logic
├── pkg/                  # Public libraries
│   └── client/           # Client SDK
├── docker/               # Docker configuration
│   ├── Dockerfile.api        # Production build for API
│   ├── Dockerfile.worker     # Production build for worker
│   ├── Dockerfile.scheduler  # Production build for scheduler
│   ├── Dockerfile.dev        # Development build with hot reload
│   ├── .air.api.toml         # Hot reload config for API
│   ├── .air.worker.toml      # Hot reload config for worker
│   └── .air.scheduler.toml   # Hot reload config for scheduler
├── docker-compose.yml    # Production orchestration
├── docker-compose.dev.yml # Development orchestration with hot reload
├── Makefile              # Convenient command aliases
├── .dockerignore         # Docker build exclusions
├── .gitignore            # Git exclusions
├── go.mod                # Go module definition
└── README.md             # Project documentation
```

### Docker Configuration

**Production (`docker-compose.yml`)**:
- Multi-stage builds for minimal image size
- **Build stage**: `golang:1.23-alpine` for compiling
- **Runtime stage**: `alpine:latest` (small, optimized)
- Optimized binaries with no development tools

**Development (`docker-compose.dev.yml`)**:
- Single-stage build with `golang:1.23-alpine`
- Includes [Air](https://github.com/cosmtrek/air) for hot reload
- Source code mounted as volume
- Instant feedback on code changes (~1s restart)
- No image rebuilds needed during development

Both setups include:
- Health checks for service dependencies
- Restart policies for reliability
- Environment variables for configuration

## License

MIT

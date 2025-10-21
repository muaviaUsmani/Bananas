# Command Line Applications (`cmd/`)

This directory contains the main entry points for all Bananas services. Each subdirectory represents a separate microservice that can be built and deployed independently.

## Services

### 📡 API Server (`api/`)

**Purpose**: REST API server for job submission and management.

**Responsibilities**:
- Accept HTTP requests for job submission
- Provide endpoints for job status queries
- Manage job lifecycle through the queue
- Handle authentication and rate limiting (future)

**Configuration**:
- `API_PORT`: Port to listen on (default: `8080`)
- `REDIS_URL`: Redis connection string

**Running**:
```bash
# Development
make dev

# Production
make up
```

---

### 👷 Worker Service (`worker/`)

**Purpose**: Background worker that processes jobs from the Redis queue.

**Responsibilities**:
- Continuously poll Redis queue for new jobs
- Execute jobs using registered handlers
- Manage worker pool with configurable concurrency
- Handle job timeouts and failures
- Report job completion or failure back to Redis

**Configuration**:
- `WORKER_CONCURRENCY`: Number of concurrent workers (default: `5`)
- `JOB_TIMEOUT`: Maximum time per job (default: `5m`)
- `MAX_RETRIES`: Maximum retry attempts (default: `3`)
- `REDIS_URL`: Redis connection string

**Handler Registration**:
Workers use a registry pattern to map job names to handler functions:

```go
registry := worker.NewRegistry()
registry.Register("send_email", HandleSendEmail)
registry.Register("process_data", HandleProcessData)
```

**Running**:
```bash
# Development (with hot reload)
make dev

# Production
make up
```

---

### ⏰ Scheduler Service (`scheduler/`)

**Purpose**: Background service that manages delayed job execution and retries.

**Responsibilities**:
- Monitor Redis scheduled set every second
- Move jobs from scheduled set to priority queues when ready
- Enable exponential backoff retry strategy
- Handle time-based job scheduling

**How It Works**:
1. Failed jobs are placed in a scheduled set with a future timestamp
2. Scheduler checks every second for jobs whose timestamp has passed
3. Ready jobs are moved back to their respective priority queues
4. Workers can then pick up the retried jobs

**Configuration**:
- `MAX_RETRIES`: Used to validate retry logic (default: `3`)
- `REDIS_URL`: Redis connection string

**Retry Schedule**:
- 1st retry: 2 seconds after failure
- 2nd retry: 4 seconds after failure  
- 3rd retry: 8 seconds after failure
- After max retries: moved to dead letter queue

**Running**:
```bash
# Development
make dev

# Production
make up
```

---

## Service Architecture

```
┌─────────────┐
│   API       │  (Port 8080)
│   Server    │  - Job submission
└──────┬──────┘  - Status queries
       │
       ▼
┌─────────────────────┐
│       Redis         │
│  ┌──────────────┐   │
│  │ Priority     │   │  High → Normal → Low
│  │ Queues       │   │
│  ├──────────────┤   │
│  │ Scheduled    │   │  Jobs waiting for retry time
│  │ Set (ZSET)   │   │
│  ├──────────────┤   │
│  │ Processing   │   │  Currently executing jobs
│  │ Queue        │   │
│  ├──────────────┤   │
│  │ Dead Letter  │   │  Failed jobs (max retries)
│  │ Queue        │   │
│  └──────────────┘   │
└─────┬───────┬───────┘
      │       │
      │       ▼
      │  ┌──────────────┐
      │  │  Scheduler   │  Every 1s: move ready jobs
      │  │  Service     │  from scheduled → queues
      │  └──────────────┘
      │
      ▼
┌─────────────┐
│   Worker    │  Configurable concurrency
│   Pool      │  - Dequeue jobs
│   (5-10+)   │  - Execute handlers
└─────────────┘  - Handle timeouts
```

## Building Services

Each service can be built independently:

```bash
# Build API
go build -o bin/api ./cmd/api

# Build Worker
go build -o bin/worker ./cmd/worker

# Build Scheduler
go build -o bin/scheduler ./cmd/scheduler
```

## Docker Deployment

All services are containerized and orchestrated with Docker Compose:

```bash
# Development mode (hot reload)
make dev-build
make dev

# Production mode
make build
make up

# Stop services
make down
```

## Tests

**Note**: The `cmd/` services themselves don't have unit tests as they are thin wrappers around the internal packages. All business logic is tested in:
- `internal/worker/` - Worker pool and executor tests
- `internal/queue/` - Redis queue operations tests
- `tests/` - End-to-end integration tests

To test the services manually:
```bash
# Start all services
make dev

# Submit a test job (requires API endpoints - to be implemented)
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{"name": "send_email", "payload": {...}, "priority": "high"}'
```


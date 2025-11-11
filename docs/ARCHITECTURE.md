# Bananas Architecture

> **Last Updated:** 2025-11-10
> **Status:** Complete

## Table of Contents

- [Overview](#overview)
- [System Components](#system-components)
- [Data Flow](#data-flow)
- [Redis Data Model](#redis-data-model)
- [Concurrency Model](#concurrency-model)
- [Design Decisions](#design-decisions)
- [Scalability & Performance](#scalability--performance)

## Overview

Bananas is a distributed task queue system built on Redis, designed for high-performance asynchronous job processing with advanced features like task routing, periodic scheduling, and result tracking.

### Architecture Principles

1. **Simplicity** - Easy to understand, deploy, and operate
2. **Performance** - Sub-millisecond latency, high throughput
3. **Reliability** - Automatic retries, dead letter queues, graceful degradation
4. **Scalability** - Horizontal scaling for workers and queue operations
5. **Flexibility** - Multiple worker modes, pluggable backends

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Client Applications                      │
│  (Go SDK, Future: Python, TypeScript, Ruby)                     │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                        API Server (Optional)                     │
│  - HTTP/REST endpoints for job submission                       │
│  - Job status queries                                            │
│  - Metrics endpoints                                             │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                          Redis Cluster                           │
│  ┌──────────────────┐  ┌──────────────────┐  ┌────────────────┐│
│  │   Job Queues     │  │  Result Storage  │  │  Pub/Sub       ││
│  │  (Priority +     │  │  (Hashes + TTL)  │  │  (Notifications)││
│  │   Routing)       │  └──────────────────┘  └────────────────┘│
│  │                  │  ┌──────────────────┐  ┌────────────────┐│
│  │ - Pending        │  │  Scheduled Jobs  │  │  Distributed   ││
│  │ - Processing     │  │  (Sorted Set)    │  │  Locks         ││
│  │ - Dead Letter    │  └──────────────────┘  │  (Cron/Sched)  ││
│  └──────────────────┘                        └────────────────┘│
└────────────────────────────┬────────────────────────────────────┘
                             │
              ┌──────────────┴──────────────┐
              │                             │
              ▼                             ▼
┌──────────────────────────┐  ┌──────────────────────────┐
│    Worker Pool 1         │  │    Worker Pool N         │
│  (Routing: gpu, default) │  │  (Routing: email)        │
│                          │  │                          │
│  ┌────────────────────┐  │  │  ┌────────────────────┐  │
│  │ Worker Goroutines  │  │  │  │ Worker Goroutines  │  │
│  │ (Concurrent: 10)   │  │  │  │ (Concurrent: 5)    │  │
│  └────────────────────┘  │  │  └────────────────────┘  │
│                          │  │                          │
│  ┌────────────────────┐  │  │  ┌────────────────────┐  │
│  │  Job Handlers      │  │  │  │  Job Handlers      │  │
│  │  - process_image   │  │  │  │  - send_email      │  │
│  │  - train_model     │  │  │  │  - send_sms        │  │
│  └────────────────────┘  │  │  └────────────────────┘  │
└──────────────────────────┘  └──────────────────────────┘
              │                             │
              ▼                             ▼
┌──────────────────────────────────────────────────────────┐
│              Scheduler Process (Periodic Tasks)           │
│  - Cron job scheduling                                    │
│  - Distributed locking                                    │
│  - Retry job scheduling (exponential backoff)             │
└──────────────────────────────────────────────────────────┘
```

## System Components

### 1. Client SDK (`pkg/client`)

**Purpose:** Provides a high-level API for job submission and result retrieval.

**Key Features:**
- Job submission with priority and routing
- Result polling and waiting (RPC-style)
- Scheduled job submission
- Configurable TTL for results

**Example:**
```go
client, _ := client.NewClient("redis://localhost:6379")

// Submit job with routing
jobID, _ := client.SubmitJobWithRoute(
    "process_image",
    payload,
    job.PriorityHigh,
    "gpu",
)

// Wait for result (RPC-style)
result, _ := client.SubmitAndWait(ctx, "job_name", payload, priority, 30*time.Second)
```

### 2. Job Queue (`internal/queue`)

**Purpose:** Manages job lifecycle from submission to completion.

**Responsibilities:**
- Enqueue jobs to routing-aware priority queues
- Dequeue jobs with priority ordering
- Job retry with exponential backoff
- Dead letter queue for failed jobs
- Scheduled job management

**Queue Structure:**
```
Priority Queues (per routing key):
  bananas:route:{routing_key}:queue:high
  bananas:route:{routing_key}:queue:normal
  bananas:route:{routing_key}:queue:low

Shared Queues:
  bananas:queue:processing (all jobs being processed)
  bananas:queue:dead (failed jobs after max retries)
  bananas:queue:scheduled (sorted set, score = execution time)

Job Data:
  bananas:job:{job_id} (hash with job details + TTL)

Result Storage:
  bananas:result:{job_id} (hash with result + TTL)
  bananas:result:notify:{job_id} (pub/sub channel)
```

### 3. Worker Pool (`internal/worker`)

**Purpose:** Executes jobs concurrently with configurable worker modes.

**Worker Modes:**
1. **Thin Mode** - Single process, all priorities, low concurrency
2. **Default Mode** - Standard production mode with priority awareness
3. **Specialized Mode** - Dedicated to specific priorities
4. **Job-Specialized Mode** - Handles specific job types only
5. **Scheduler-Only Mode** - No job execution, only scheduling

**Features:**
- Goroutine-based concurrency (configurable 1-1000)
- Graceful shutdown (completes in-flight jobs)
- Panic recovery with stack traces
- Context-based timeout management
- Routing key support (multiple keys per worker)

**Example Configuration:**
```bash
# Environment variables
WORKER_MODE=default
WORKER_CONCURRENCY=10
WORKER_PRIORITIES=high,normal,low
WORKER_ROUTING_KEYS=gpu,default
WORKER_JOB_TYPES=process_image,train_model
```

### 4. Executor (`internal/worker/executor.go`)

**Purpose:** Executes individual jobs using registered handlers.

**Responsibilities:**
- Handler registry management
- Job execution with timeout
- Result storage (if backend configured)
- Error handling and retry triggering
- Metrics recording

**Flow:**
```
1. Lookup handler by job name
2. Execute with context timeout
3. Store result (success/failure)
4. Update queue (complete or fail)
5. Record metrics
```

### 5. Scheduler (`internal/scheduler`)

**Purpose:** Manages periodic tasks and scheduled job execution.

**Components:**

**Cron Scheduler:**
- Distributed cron job execution
- Timezone support (IANA timezones)
- Distributed locking (prevents duplicate execution)
- Schedule state tracking

**Job Scheduler:**
- Moves scheduled jobs to ready queues
- Handles retry scheduling with exponential backoff
- Runs every second (configurable)

**Locking Mechanism:**
```go
// Distributed lock using Redis SET NX
lock := scheduler.AcquireLock(ctx, scheduleID)
if lock != nil {
    defer lock.Release()
    // Execute cron job
}
```

### 6. Result Backend (`internal/result`)

**Purpose:** Stores and retrieves job results with efficient waiting.

**Features:**
- Redis hash storage with configurable TTL
- Pub/sub notifications for result readiness
- Efficient waiting (no polling overhead)
- Pluggable interface for future backends

**Flow:**
```
Worker stores result:
  HSET bananas:result:{job_id} {result_data}
  EXPIRE bananas:result:{job_id} {ttl}
  PUBLISH bananas:result:notify:{job_id} "ready"

Client waits for result:
  SUBSCRIBE bananas:result:notify:{job_id}
  Wait for notification
  HGETALL bananas:result:{job_id}
```

### 7. Configuration (`internal/config`)

**Purpose:** Centralized configuration management with environment variables.

**Configuration Sources:**
1. Environment variables (primary)
2. Defaults (fallback)
3. Validation on load

**Key Settings:**
- Redis connection (URL, pool size, timeouts)
- Worker configuration (mode, concurrency, priorities, routing)
- Scheduler configuration (interval, enable/disable)
- Result backend (enable, TTL for success/failure)
- Logging (level, format, output)

## Data Flow

### Job Submission Flow

```
1. Client creates Job struct
   ├─ Assigns unique ID (UUID)
   ├─ Sets priority (high/normal/low)
   ├─ Sets routing key (default: "default")
   └─ Marshals payload to JSON

2. Client calls queue.Enqueue()
   ├─ Validates routing key
   ├─ Stores job data: SET bananas:job:{id} {job_json}
   └─ Pushes to queue: LPUSH bananas:route:{routing_key}:queue:{priority} {job_id}

3. Metrics recorded
   └─ Queue depth, enqueue rate
```

### Job Processing Flow

```
1. Worker calls queue.DequeueWithRouting(routingKeys)
   ├─ Builds queue list based on routing keys + priorities
   │  Example: ["gpu:high", "gpu:normal", "gpu:low", "default:high", ...]
   ├─ Uses BRPOPLPUSH to atomically move job to processing queue
   │  └─ Blocks until job available or timeout
   └─ Retrieves job data: GET bananas:job:{id}

2. Worker passes job to executor.ExecuteJob()
   ├─ Looks up handler by job name
   ├─ Creates context with timeout
   ├─ Executes handler(ctx, job)
   │  ├─ Handler processes payload
   │  └─ Returns result or error
   └─ Stores result (if backend configured)

3. On Success:
   ├─ queue.Complete(job_id)
   │  ├─ LREM processing queue
   │  └─ SET job status to completed (with TTL)
   └─ resultBackend.StoreResult(success)
       ├─ HSET result data
       ├─ EXPIRE with success TTL (1h)
       └─ PUBLISH notification

4. On Failure:
   ├─ queue.Fail(job, error)
   │  ├─ Increment attempts
   │  ├─ If attempts < maxRetries:
   │  │  ├─ Calculate backoff: 2^attempts seconds
   │  │  ├─ ZADD to scheduled set (score = retry_time)
   │  │  └─ LREM from processing queue
   │  └─ Else:
   │     ├─ LPUSH to dead letter queue
   │     ├─ LREM from processing queue
   │     └─ SET job status to failed (with TTL)
   └─ resultBackend.StoreResult(failure)

5. Scheduler (background process):
   ├─ Runs every second
   ├─ Calls queue.MoveScheduledToReady()
   │  ├─ ZRANGEBYSCORE scheduled set (score <= now)
   │  ├─ For each ready job:
   │  │  ├─ LPUSH to routed priority queue
   │  │  └─ ZREM from scheduled set
   │  └─ Returns count of moved jobs
   └─ Logs moved job count
```

### Periodic Task Flow

```
1. Cron Scheduler initializes
   ├─ Loads schedule definitions
   └─ Starts ticker (checks every minute)

2. On each tick:
   ├─ For each enabled schedule:
   │  ├─ Check if cron expression matches current time
   │  ├─ Attempt distributed lock acquisition
   │  │  └─ SET NX with TTL
   │  ├─ If lock acquired:
   │  │  ├─ Create job from schedule
   │  │  ├─ Enqueue job
   │  │  ├─ Update schedule state (last run, count)
   │  │  └─ Release lock
   │  └─ Else: skip (another scheduler already running it)
   └─ Continue to next schedule

3. Schedule State Persistence
   └─ Redis hash: bananas:cron:state:{schedule_id}
      ├─ last_run: timestamp
      ├─ next_run: timestamp
      ├─ run_count: integer
      └─ last_error: string (if any)
```

## Redis Data Model

### Job Data Structure

```go
type Job struct {
    ID           string          // UUID
    Name         string          // Handler name
    Description  string          // Optional description
    Payload      json.RawMessage // Job-specific data
    Status       JobStatus       // pending|processing|completed|failed|scheduled
    Priority     JobPriority     // high|normal|low
    RoutingKey   string          // Worker routing (default: "default")
    CreatedAt    time.Time       // Job creation time
    UpdatedAt    time.Time       // Last update time
    ScheduledFor *time.Time      // Scheduled execution time (nullable)
    Attempts     int             // Current attempt count
    MaxRetries   int             // Maximum retry attempts
    Error        string          // Error message (if failed)
}
```

### Redis Key Patterns

| Pattern | Type | Purpose | TTL |
|---------|------|---------|-----|
| `bananas:route:{key}:queue:{pri}` | List | Priority queue for routing key | None |
| `bananas:queue:processing` | List | Jobs currently being processed | None |
| `bananas:queue:dead` | List | Failed jobs (max retries exceeded) | None |
| `bananas:queue:scheduled` | ZSet | Scheduled jobs (score = exec time) | None |
| `bananas:job:{id}` | String | Job data (JSON) | 24h-7d |
| `bananas:result:{id}` | Hash | Job result | 1h-24h |
| `bananas:result:notify:{id}` | PubSub | Result ready notification | N/A |
| `bananas:cron:lock:{id}` | String | Distributed cron lock | 5min |
| `bananas:cron:state:{id}` | Hash | Cron schedule state | None |

### Memory Management

**TTL Strategy:**
- Completed jobs: 24 hours (configurable)
- Failed jobs: 7 days (configurable)
- Successful results: 1 hour (configurable)
- Failed results: 24 hours (configurable)
- Cron locks: 5 minutes (prevents stale locks)

**Queue Cleanup:**
- Dead letter queue: manual cleanup or monitoring-triggered
- Processing queue: cleaned up on job completion/failure
- Scheduled set: cleaned up when jobs move to ready queues

## Concurrency Model

### Worker Concurrency

**Goroutine-Based:**
- One goroutine per worker (configurable 1-1000)
- Shared queue reader (thread-safe Redis client)
- Independent job execution contexts

**Example with 10 workers:**
```
Pool starts 10 goroutines:
  Worker-1: loop { dequeue -> execute -> complete }
  Worker-2: loop { dequeue -> execute -> complete }
  ...
  Worker-10: loop { dequeue -> execute -> complete }

Each worker:
  - Independently blocks on BRPOPLPUSH
  - Executes jobs with context timeout
  - Handles panics with recovery
  - Updates shared Redis state atomically
```

### Redis Connection Pooling

**Configuration:**
```go
opts.PoolSize = 50              // Max connections
opts.MinIdleConns = 5           // Keep-alive connections
opts.ConnMaxIdleTime = 10m      // Idle connection timeout
opts.PoolTimeout = 5s           // Wait for connection
opts.ReadTimeout = 10s          // Blocking operations (BRPOPLPUSH)
opts.WriteTimeout = 3s          // Write operations
```

**Connection Usage:**
- Workers: 1 connection per worker (blocking dequeue)
- API: Concurrent connections from pool
- Scheduler: 1-2 connections (move scheduled + cron)

**Total Connections:** ~10 workers + 10 API + 2 scheduler + 5 buffer = 27 connections

### Synchronization

**Redis Atomic Operations:**
- `BRPOPLPUSH`: Atomic dequeue + move to processing
- `PIPELINE`: Batch operations in single round trip
- `SET NX`: Distributed lock acquisition
- `ZADD` + `ZREM`: Atomic scheduled set operations

**No Application-Level Locks:**
- All synchronization via Redis
- No mutexes in application code
- Stateless workers (scale horizontally)

## Design Decisions

### Why Redis?

1. **Performance**: In-memory, sub-millisecond latency
2. **Atomic Operations**: BRPOPLPUSH, pipelines, transactions
3. **Built-in Features**: Lists, sorted sets, pub/sub, TTL
4. **Proven**: Battle-tested in production (Celery, Sidekiq, Bull)
5. **Simple**: Single dependency, easy to deploy

### Why Goroutines for Workers?

1. **Lightweight**: 2KB stack vs MB for threads
2. **Fast**: Context switching in microseconds
3. **Scalable**: 10,000+ goroutines on single machine
4. **Built-in**: Native Go concurrency

### Why Blocking Dequeue (BRPOPLPUSH)?

**Before (Polling):**
```go
for {
    job := queue.Dequeue()
    if job == nil {
        time.Sleep(100 * time.Millisecond) // Wasted CPU + latency
    }
    execute(job)
}
```

**After (Blocking):**
```go
for {
    job := queue.Dequeue() // Blocks until job available
    execute(job)           // No sleep, no wasted CPU
}
```

**Benefits:**
- Eliminates busy-waiting
- Reduces Redis load (no constant polling)
- Lower latency (instant job processing)
- Lower CPU usage

### Why Priority Queues?

Strict priority ordering ensures critical jobs processed first:
```
High:   Job1 Job2 Job3
Normal: Job4 Job5 Job6
Low:    Job7 Job8 Job9

Dequeue order: Job1, Job2, Job3, Job4, Job5, Job6, Job7, Job8, Job9
```

### Why Routing Keys?

**Problem:** All jobs in same queues, any worker can pick up any job.

**Solution:** Route jobs to specific workers based on requirements:
- GPU jobs → GPU-enabled workers
- Email jobs → Email-specialized workers
- Region jobs → Workers in specific regions

**Benefits:**
- Resource isolation
- Independent scaling
- Workload segregation

### Why Exponential Backoff for Retries?

**Linear Backoff (bad):**
```
Retry 1: +5s = 5s
Retry 2: +5s = 10s
Retry 3: +5s = 15s
```
- Constant retry pressure on failing dependency

**Exponential Backoff (good):**
```
Retry 1: 2^1 = 2s
Retry 2: 2^2 = 4s
Retry 3: 2^3 = 8s
```
- Gives failing dependency time to recover
- Prevents thundering herd

### Why Pub/Sub for Results?

**Polling (bad):**
```go
for {
    result := backend.GetResult(jobID)
    if result != nil {
        return result
    }
    time.Sleep(100 * time.Millisecond) // Wasted CPU + latency
}
```

**Pub/Sub (good):**
```go
subscribe(notifyChannel)
wait for notification  // Blocks, no CPU waste
return backend.GetResult(jobID)
```

**Benefits:**
- No polling overhead
- Instant notification
- Lower Redis load

## Scalability & Performance

### Horizontal Scaling

**Workers:**
```bash
# Scale to 50 workers across 5 machines
Machine 1: WORKER_CONCURRENCY=10 ./worker &
Machine 2: WORKER_CONCURRENCY=10 ./worker &
Machine 3: WORKER_CONCURRENCY=10 ./worker &
Machine 4: WORKER_CONCURRENCY=10 ./worker &
Machine 5: WORKER_CONCURRENCY=10 ./worker &
```

**Routing-Based Scaling:**
```bash
# Scale GPU workers independently
Machine 1: WORKER_ROUTING_KEYS=gpu WORKER_CONCURRENCY=5 ./worker &
Machine 2: WORKER_ROUTING_KEYS=gpu WORKER_CONCURRENCY=5 ./worker &

# Scale email workers
Machine 3: WORKER_ROUTING_KEYS=email WORKER_CONCURRENCY=10 ./worker &
```

### Performance Characteristics

From benchmarks (`docs/PERFORMANCE.md`):

| Metric | Value |
|--------|-------|
| Job submission rate | 1,650+ jobs/sec |
| Job processing rate | 1,000+ jobs/sec (10 workers) |
| p50 latency | < 1ms |
| p95 latency | < 2ms |
| p99 latency | < 3ms |
| Queue depth impact | Linear (100-10K jobs) |
| Payload size impact | < 5% (1KB-100KB) |

### Bottlenecks & Solutions

**Redis CPU:**
- Monitor with `INFO CPU`
- Scale to Redis Cluster if needed
- Use pipelining for batch operations

**Network:**
- Deploy workers close to Redis
- Use connection pooling
- Compress large payloads (protobuf)

**Worker CPU:**
- Profile with `pprof`
- Optimize hot paths
- Scale horizontally

### Monitoring

**Key Metrics:**
- Queue depth per routing key
- Job processing rate
- Worker utilization
- Redis CPU/memory
- P99 latency

**Tools:**
- Prometheus metrics (built-in)
- Redis `INFO` command
- Go `pprof` profiling

## Related Documentation

- [Worker Architecture Design](./WORKER_ARCHITECTURE_DESIGN.md) - Worker mode details
- [Multi-Tier Workers](./MULTI_TIER_WORKERS.md) - Worker specialization strategies
- [Task Routing Usage](./TASK_ROUTING_USAGE.md) - Routing configuration guide
- [Result Backend Design](./RESULT_BACKEND_DESIGN.md) - Result storage details
- [Periodic Tasks Design](./PERIODIC_TASKS_DESIGN.md) - Cron scheduling details
- [Performance Guide](./PERFORMANCE.md) - Benchmarks and tuning
- [API Reference](./API_REFERENCE.md) - Complete API documentation

---

**Next:** [API Reference](./API_REFERENCE.md) | [Integration Guide](../INTEGRATION_GUIDE.md) | [Deployment Guide](./DEPLOYMENT.md)

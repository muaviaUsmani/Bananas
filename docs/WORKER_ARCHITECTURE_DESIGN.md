# Multi-Tier Worker Architecture - Design Document

## Overview

The Bananas task queue supports flexible worker deployment configurations to match different production scenarios, from single-process development setups to distributed, specialized worker pools.

## Worker Modes

### 1. Thin Mode (`thin`)

**Use Case:** Development, testing, very low traffic environments (<100 jobs/hour)

**Architecture:**
```
┌─────────────────────────────┐
│   Single Worker Process     │
│  • Handles ALL queues       │
│  • Low concurrency (1-10)   │
│  • All job types            │
└─────────────────────────────┘
```

**Configuration:**
```bash
WORKER_MODE=thin
WORKER_CONCURRENCY=5
```

**Characteristics:**
- Single process polls all queues (high → normal → low)
- Minimal resource usage
- Simple deployment (no orchestration needed)
- Suitable for development and low-traffic production

---

### 2. Default Mode (`default`)

**Use Case:** Standard production deployments (1K-10K jobs/hour)

**Architecture:**
```
┌─────────────────────────────┐
│   Worker Process            │
│  • Priority-aware polling   │
│  • Medium concurrency       │
│  • All job types            │
└─────────────────────────────┘
```

**Configuration:**
```bash
WORKER_MODE=default          # Or omit (default)
WORKER_CONCURRENCY=20
WORKER_PRIORITIES=high,normal,low
```

**Characteristics:**
- Priority-based job processing
- Configurable concurrency (10-50 workers)
- Handles all job types
- Can scale horizontally (multiple instances)

---

### 3. Specialized Mode (`specialized`)

**Use Case:** High-traffic production (10K+ jobs/hour), dedicated resources per queue/job type

**Architecture:**
```
┌──────────────────────┐
│  High Priority Pool  │
│  • Concurrency: 50   │
│  • Priority: high    │
└──────────────────────┘

┌──────────────────────┐
│  Normal Priority Pool│
│  • Concurrency: 30   │
│  • Priority: normal  │
└──────────────────────┘

┌──────────────────────┐
│  Low Priority Pool   │
│  • Concurrency: 10   │
│  • Priority: low     │
└──────────────────────┘
```

**Configuration:**
```bash
# High priority worker
WORKER_MODE=specialized
WORKER_PRIORITIES=high
WORKER_CONCURRENCY=50

# Normal priority worker
WORKER_MODE=specialized
WORKER_PRIORITIES=normal
WORKER_CONCURRENCY=30

# Low priority worker
WORKER_MODE=specialized
WORKER_PRIORITIES=low
WORKER_CONCURRENCY=10
```

**Characteristics:**
- Separate processes per priority queue
- Dedicated resources per queue
- Prevents low-priority jobs from blocking high-priority
- Fine-grained scaling per queue

---

### 4. Job-Type Specialized Mode (`job-specialized`)

**Use Case:** Different resource requirements for different job types

**Architecture:**
```
┌──────────────────────┐
│  CPU-Intensive Pool  │
│  • send_email        │
│  • generate_report   │
│  • Concurrency: 10   │
└──────────────────────┘

┌──────────────────────┐
│  I/O-Intensive Pool  │
│  • process_images    │
│  • fetch_data        │
│  • Concurrency: 50   │
└──────────────────────┘
```

**Configuration:**
```bash
# CPU-intensive worker
WORKER_MODE=job-specialized
WORKER_JOB_TYPES=send_email,generate_report
WORKER_CONCURRENCY=10

# I/O-intensive worker
WORKER_MODE=job-specialized
WORKER_JOB_TYPES=process_images,fetch_data
WORKER_CONCURRENCY=50
```

**Characteristics:**
- Workers register for specific job types only
- Optimize concurrency per job type
- Deploy on appropriate hardware (CPU vs I/O optimized)

---

### 5. Scheduler-Only Mode (`scheduler-only`)

**Use Case:** Dedicated scheduler process for scheduled/periodic tasks

**Architecture:**
```
┌─────────────────────────────┐
│   Scheduler Process         │
│  • No job execution         │
│  • Moves scheduled → ready  │
│  • Runs every 1 second      │
└─────────────────────────────┘
```

**Configuration:**
```bash
WORKER_MODE=scheduler-only
SCHEDULER_INTERVAL=1s
```

**Characteristics:**
- No job execution (zero worker goroutines)
- Only runs MoveScheduledToReady loop
- Lightweight process for time-based scheduling
- High availability through leader election (optional)

---

## Configuration Schema

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `WORKER_MODE` | `default` | Worker mode: `thin`, `default`, `specialized`, `job-specialized`, `scheduler-only` |
| `WORKER_CONCURRENCY` | `10` | Number of concurrent workers |
| `WORKER_PRIORITIES` | `high,normal,low` | Comma-separated priority queues to process |
| `WORKER_JOB_TYPES` | `*` (all) | Comma-separated job types to handle (for job-specialized mode) |
| `SCHEDULER_INTERVAL` | `1s` | How often to check for scheduled jobs |
| `ENABLE_SCHEDULER` | `true` | Whether to run scheduler loop (for non-scheduler-only modes) |

### Configuration Struct

```go
type WorkerConfig struct {
    Mode           WorkerMode     // thin, default, specialized, etc.
    Concurrency    int            // Number of worker goroutines
    Priorities     []JobPriority  // Which priority queues to process
    JobTypes       []string       // Which job types to handle (empty = all)
    SchedulerInterval time.Duration // Scheduler check interval
    EnableScheduler bool           // Run scheduler loop
}
```

---

## Decision Tree: Which Mode to Use?

```
Start
  |
  ├─ Development / Testing?
  │   └─> THIN MODE (concurrency: 1-5)
  |
  ├─ Traffic < 1K jobs/hour?
  │   └─> DEFAULT MODE (concurrency: 10-20)
  |
  ├─ Traffic 1K-10K jobs/hour?
  │   └─> DEFAULT MODE (concurrency: 20-50, scale horizontally)
  |
  ├─ Traffic > 10K jobs/hour?
  │   |
  │   ├─ Need priority isolation?
  │   │   └─> SPECIALIZED MODE (separate process per priority)
  │   |
  │   ├─ Job types have different resource needs?
  │   │   └─> JOB-SPECIALIZED MODE (separate pools)
  │   |
  │   └─ General high traffic?
  │       └─> DEFAULT MODE (many instances, load balancer)
  |
  └─ Need dedicated scheduler?
      └─> SCHEDULER-ONLY MODE (1 instance)
```

---

## Deployment Examples

### Docker Compose - Thin Mode (Development)

```yaml
version: '3.8'
services:
  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]

  worker:
    image: bananas-worker
    environment:
      - WORKER_MODE=thin
      - WORKER_CONCURRENCY=5
      - REDIS_URL=redis://redis:6379
```

### Docker Compose - Specialized Mode (Production)

```yaml
version: '3.8'
services:
  redis:
    image: redis:7-alpine

  # High priority worker
  worker-high:
    image: bananas-worker
    deploy:
      replicas: 3
    environment:
      - WORKER_MODE=specialized
      - WORKER_PRIORITIES=high
      - WORKER_CONCURRENCY=50
      - REDIS_URL=redis://redis:6379

  # Normal priority worker
  worker-normal:
    image: bananas-worker
    deploy:
      replicas: 2
    environment:
      - WORKER_MODE=specialized
      - WORKER_PRIORITIES=normal
      - WORKER_CONCURRENCY=30
      - REDIS_URL=redis://redis:6379

  # Low priority worker
  worker-low:
    image: bananas-worker
    deploy:
      replicas: 1
    environment:
      - WORKER_MODE=specialized
      - WORKER_PRIORITIES=low
      - WORKER_CONCURRENCY=10
      - REDIS_URL=redis://redis:6379

  # Dedicated scheduler
  scheduler:
    image: bananas-worker
    deploy:
      replicas: 1
    environment:
      - WORKER_MODE=scheduler-only
      - SCHEDULER_INTERVAL=1s
      - REDIS_URL=redis://redis:6379
```

### Kubernetes - Job-Specialized Mode

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker-cpu-intensive
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: worker
        image: bananas-worker
        resources:
          requests:
            cpu: "2"
            memory: "4Gi"
        env:
        - name: WORKER_MODE
          value: "job-specialized"
        - name: WORKER_JOB_TYPES
          value: "generate_report,process_analytics"
        - name: WORKER_CONCURRENCY
          value: "10"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker-io-intensive
spec:
  replicas: 5
  template:
    spec:
      containers:
      - name: worker
        image: bananas-worker
        resources:
          requests:
            cpu: "1"
            memory: "2Gi"
        env:
        - name: WORKER_MODE
          value: "job-specialized"
        - name: WORKER_JOB_TYPES
          value: "fetch_data,process_images"
        - name: WORKER_CONCURRENCY
          value: "50"
```

---

## Scaling Guidelines

### Horizontal Scaling (More Instances)

**When:**
- Worker utilization consistently > 80%
- Queue depth growing
- Job processing latency increasing

**How:**
```bash
# Docker
docker-compose up --scale worker=5

# Kubernetes
kubectl scale deployment worker --replicas=5
```

### Vertical Scaling (More Concurrency)

**When:**
- CPU/memory headroom available
- Jobs are I/O bound
- Worker utilization < 50%

**How:**
```bash
# Increase WORKER_CONCURRENCY
WORKER_CONCURRENCY=50 # was 20
```

### Mode Migration

| From | To | When | Benefits |
|------|----|----|----------|
| Thin | Default | Traffic increases | Better performance |
| Default | Specialized | Need priority isolation | Guaranteed high-priority processing |
| Default | Job-Specialized | Different resource needs | Optimize per job type |

---

## Implementation Notes

### Priority Selection Logic

```go
// Priorities are checked in order specified
// Example: WORKER_PRIORITIES=high,normal,low

for _, priority := range priorities {
    job := queue.Dequeue(priority)
    if job != nil {
        execute(job)
        break // Process one job, then restart priority scan
    }
}
```

### Job Type Filtering

```go
// In job-specialized mode
allowedTypes := config.JobTypes
if len(allowedTypes) > 0 && !contains(allowedTypes, job.Name) {
    // Skip this job, let another worker handle it
    continue
}
```

### Scheduler Leader Election (Advanced)

For scheduler-only mode in HA setup:

```go
// Use Redis-based distributed lock
lock := queue.AcquireLock("scheduler:leader", 10*time.Second)
if lock != nil {
    defer lock.Release()
    // Run scheduler logic
    queue.MoveScheduledToReady()
}
```

---

## Monitoring Recommendations

### Metrics to Track Per Mode

| Metric | Thin | Default | Specialized | Scheduler-Only |
|--------|------|---------|-------------|----------------|
| Worker Utilization | ✓ | ✓ | ✓ | - |
| Queue Depth | ✓ | ✓ | ✓ per queue | ✓ (scheduled) |
| Jobs Processed | ✓ | ✓ | ✓ per queue | ✓ (moved) |
| Error Rate | ✓ | ✓ | ✓ | - |

### Alert Thresholds

```yaml
# Thin mode
- worker_utilization > 90% for 5min → Scale to default mode

# Default mode
- queue_depth > 1000 for 5min → Add more workers
- worker_utilization > 95% for 10min → Increase concurrency

# Specialized mode
- high_priority_queue_depth > 100 for 2min → CRITICAL
- low_priority_worker_utilization < 20% for 30min → Reduce low workers
```

---

## Next Steps

1. Implement WorkerConfig struct
2. Add mode validation logic
3. Implement priority filtering
4. Implement job type filtering
5. Add scheduler-only mode support
6. Write comprehensive tests
7. Create deployment examples
8. Document migration paths

---

**Last Updated:** 2025-11-10

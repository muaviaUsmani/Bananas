# Task Routing Usage Guide

Task routing enables you to direct jobs to specific worker pools based on routing keys. This allows resource isolation, workload segregation, and independent scaling of different job types.

## Table of Contents

- [Quick Start](#quick-start)
- [Concepts](#concepts)
- [Submitting Jobs with Routing](#submitting-jobs-with-routing)
- [Configuring Workers](#configuring-workers)
- [Routing Strategies](#routing-strategies)
- [Monitoring](#monitoring)
- [Best Practices](#best-practices)
- [Examples](#examples)

## Quick Start

### 1. Submit a job with a routing key

```go
import (
    "github.com/muaviaUsmani/bananas/pkg/client"
    "github.com/muaviaUsmani/bananas/internal/job"
)

client, _ := client.NewClient("redis://localhost:6379")
defer client.Close()

// Submit job to GPU workers
jobID, err := client.SubmitJobWithRoute(
    "process_image",
    payload,
    job.PriorityHigh,
    "gpu", // routing key
)
```

### 2. Configure a worker to handle specific routing keys

```bash
# Start a GPU worker (only processes "gpu" jobs)
WORKER_ROUTING_KEYS=gpu ./worker

# Start a default worker (only processes "default" jobs)
WORKER_ROUTING_KEYS=default ./worker

# Start a multi-key worker (handles both, prioritizes gpu)
WORKER_ROUTING_KEYS=gpu,default ./worker
```

That's it! Jobs with routing key "gpu" will only be processed by workers configured with the "gpu" routing key.

## Concepts

### Routing Key

A routing key is a string identifier (alphanumeric, underscore, hyphen) that determines which worker pool processes a job:

- **Valid**: `gpu`, `email`, `high_memory`, `us-east-1`, `critical`, `Worker123`
- **Invalid**: `gpu worker` (space), `gpu.worker` (dot), `gpu@worker` (special char)
- **Max length**: 64 characters

### Default Routing Key

If no routing key is specified, jobs use the `"default"` routing key for backward compatibility:

```go
// These are equivalent:
client.SubmitJob("send_email", payload, job.PriorityNormal)
client.SubmitJobWithRoute("send_email", payload, job.PriorityNormal, "default")
```

### Worker Routing Configuration

Workers specify which routing keys they handle via the `WORKER_ROUTING_KEYS` environment variable:

```bash
# Single routing key
WORKER_ROUTING_KEYS=gpu

# Multiple routing keys (comma-separated, prioritized in order)
WORKER_ROUTING_KEYS=gpu,default

# Default if not specified
WORKER_ROUTING_KEYS=default
```

### Queue Structure

Each routing key has its own priority queues:

```
bananas:route:gpu:queue:high
bananas:route:gpu:queue:normal
bananas:route:gpu:queue:low

bananas:route:email:queue:high
bananas:route:email:queue:normal
bananas:route:email:queue:low

bananas:route:default:queue:high
bananas:route:default:queue:normal
bananas:route:default:queue:low
```

## Submitting Jobs with Routing

### Basic Job Submission

```go
client, _ := client.NewClient("redis://localhost:6379")

// GPU job
jobID, err := client.SubmitJobWithRoute(
    "process_image",
    map[string]interface{}{"url": "image.jpg"},
    job.PriorityHigh,
    "gpu",
)

// Email job
jobID, err = client.SubmitJobWithRoute(
    "send_email",
    map[string]interface{}{"to": "user@example.com"},
    job.PriorityNormal,
    "email",
)

// Default job (backward compatible)
jobID, err = client.SubmitJob(
    "generate_report",
    map[string]interface{}{"type": "sales"},
    job.PriorityNormal,
)
```

### Programmatic Routing Key Setting

```go
import "github.com/muaviaUsmani/bananas/internal/job"

// Create job
j := job.NewJob("process_video", payload, job.PriorityHigh)

// Set routing key
if err := j.SetRoutingKey("gpu"); err != nil {
    log.Fatal(err)
}

// Enqueue (internal use)
queue.Enqueue(ctx, j)
```

### Routing Key Validation

```go
// Validate before setting
if err := job.ValidateRoutingKey("my-routing-key"); err != nil {
    log.Fatalf("Invalid routing key: %v", err)
}
```

## Configuring Workers

### Environment Variables

```bash
# Worker routing keys (comma-separated)
WORKER_ROUTING_KEYS=gpu,default

# Other worker configuration
WORKER_CONCURRENCY=10
WORKER_PRIORITIES=high,normal
```

### Programmatic Configuration

```go
import (
    "github.com/muaviaUsmani/bananas/internal/config"
    "github.com/muaviaUsmani/bananas/internal/worker"
)

workerConfig := &config.WorkerConfig{
    Mode:        config.WorkerModeDefault,
    Concurrency: 10,
    Priorities:  []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow},
    RoutingKeys: []string{"gpu", "default"}, // Multiple routing keys
}

pool := worker.NewPoolWithConfig(executor, queue, workerConfig, jobTimeout)
pool.Start(ctx)
```

### Priority Ordering with Multiple Routing Keys

When a worker handles multiple routing keys, jobs are dequeued in this order:

**Config**: `WORKER_ROUTING_KEYS=gpu,default`

**Dequeue order**:
1. `gpu:high`
2. `gpu:normal`
3. `gpu:low`
4. `default:high`
5. `default:normal`
6. `default:low`

This ensures:
- GPU jobs are preferred over default jobs
- Priority is respected within each routing key
- Fair processing across routing keys

## Routing Strategies

### 1. Resource Isolation

Route jobs requiring specific resources to dedicated workers:

```bash
# GPU workers (on GPU-enabled machines)
WORKER_ROUTING_KEYS=gpu ./worker

# CPU workers (on regular machines)
WORKER_ROUTING_KEYS=default ./worker
```

```go
// Submit GPU job
client.SubmitJobWithRoute("train_model", payload, job.PriorityHigh, "gpu")

// Submit CPU job
client.SubmitJobWithRoute("send_email", payload, job.PriorityNormal, "default")
```

### 2. Workload Segregation

Separate critical jobs from regular jobs:

```bash
# Critical worker pool (dedicated, high SLA)
WORKER_ROUTING_KEYS=critical

# Regular worker pool
WORKER_ROUTING_KEYS=default
```

```go
// Critical payment processing
client.SubmitJobWithRoute("process_payment", payload, job.PriorityHigh, "critical")

// Regular background job
client.SubmitJobWithRoute("cleanup_old_data", payload, job.PriorityLow, "default")
```

### 3. Geographic Distribution

Route jobs to workers in specific regions:

```bash
# US East workers
WORKER_ROUTING_KEYS=us-east-1

# EU workers
WORKER_ROUTING_KEYS=eu-west-1
```

```go
// Route to US workers
client.SubmitJobWithRoute("process_data", payload, job.PriorityNormal, "us-east-1")
```

### 4. Fallback Workers

Create general workers that can handle overflow from specialized workers:

```bash
# Specialized GPU worker (only GPU jobs)
WORKER_ROUTING_KEYS=gpu

# General worker (can handle GPU + default jobs)
WORKER_ROUTING_KEYS=default,gpu
```

This setup ensures:
- GPU jobs are processed by dedicated GPU workers first
- If GPU workers are busy, general workers can help with GPU jobs
- General workers also handle default jobs

### 5. Team/Tenant Isolation

Isolate jobs from different teams or tenants:

```bash
# Team Alpha workers
WORKER_ROUTING_KEYS=team-alpha

# Team Beta workers
WORKER_ROUTING_KEYS=team-beta
```

```go
// Team Alpha job
client.SubmitJobWithRoute("process_data", payload, job.PriorityNormal, "team-alpha")

// Team Beta job
client.SubmitJobWithRoute("process_data", payload, job.PriorityNormal, "team-beta")
```

## Monitoring

### Queue Depth by Routing Key

Check queue depth using Redis CLI:

```bash
# GPU queue depths
redis-cli LLEN bananas:route:gpu:queue:high
redis-cli LLEN bananas:route:gpu:queue:normal
redis-cli LLEN bananas:route:gpu:queue:low

# Email queue depths
redis-cli LLEN bananas:route:email:queue:high
redis-cli LLEN bananas:route:email:queue:normal
redis-cli LLEN bananas:route:email:queue:low
```

### Worker Logs

Workers log routing key information:

```
Enqueued job abc123 to routing key 'gpu' with priority high
Dequeued job abc123 from routing key 'gpu' with priority high
Moved scheduled job abc123 to routing key 'gpu' with priority normal (attempt 1/3)
```

### Metrics (if configured)

Metrics include routing key labels:

```go
metrics.RecordJobEnqueued(priority, routingKey)
metrics.RecordJobDequeued(priority, routingKey)
metrics.RecordJobCompleted(priority, routingKey)
metrics.RecordJobFailed(priority, routingKey)
```

## Best Practices

### 1. Naming Conventions

Use descriptive, short routing keys:

**Good**:
- `gpu` - Short, clear
- `high_memory` - Descriptive with underscore
- `us-east-1` - Geographic with hyphen
- `critical` - Priority-based
- `team-alpha` - Team isolation

**Avoid**:
- `GPU_WORKER_POOL_FOR_IMAGE_PROCESSING` - Too long
- `g` - Too short, unclear
- `gpu worker` - Invalid (space)
- `team@alpha` - Invalid (special char)

### 2. Capacity Planning

1. **Monitor queue depth** per routing key
2. **Scale independently**: Add more workers for specific routing keys as needed
3. **Use fallback workers**: Include popular routing keys in general workers
4. **Test under load**: Ensure workers can handle peak traffic

### 3. Routing Key Selection

Choose routing keys based on:
- **Resource requirements**: `gpu`, `high_memory`, `disk_intensive`
- **Geographic location**: `us-east-1`, `eu-west-1`, `asia-pacific`
- **Criticality**: `critical`, `normal`, `background`
- **Team/tenant**: `team-alpha`, `tenant-123`
- **Job type**: `email`, `reports`, `analytics`

### 4. Avoid Over-Routing

Don't create too many routing keys:
- More routing keys = more queues to monitor
- More routing keys = more complex worker configuration
- Start simple, add routing keys as needed

### 5. Backward Compatibility

When adding routing to an existing system:
1. Deploy queue changes first (supports routing keys)
2. Jobs default to "default" routing key automatically
3. Gradually introduce new routing keys
4. Old workers continue working with "default" routing key

## Examples

See [examples/task_routing](../examples/task_routing) for complete examples:

- **GPU Worker**: Dedicated GPU job processing
- **Multi-Key Worker**: Handles multiple routing keys with priority
- **Client**: Submitting jobs with different routing keys

### Running Examples

```bash
# Start Redis
docker run -d -p 6379:6379 redis:7-alpine

# Start GPU worker
WORKER_ROUTING_KEYS=gpu go run examples/task_routing/gpu_worker/main.go

# Start multi-key worker
WORKER_ROUTING_KEYS=gpu,email,default go run examples/task_routing/multi_worker/main.go

# Submit jobs
go run examples/task_routing/client/main.go
```

## Troubleshooting

### Jobs not being processed

**Problem**: Jobs with routing key "gpu" are not being processed

**Solution**:
1. Check worker has correct routing keys: `WORKER_ROUTING_KEYS=gpu`
2. Verify job is in correct queue: `redis-cli LLEN bananas:route:gpu:queue:high`
3. Check worker logs for errors

### Invalid routing key error

**Problem**: `invalid routing key format` error

**Solution**: Ensure routing key is alphanumeric with only underscores/hyphens, max 64 characters

**Valid**: `gpu`, `email_worker`, `us-east-1`
**Invalid**: `gpu worker`, `gpu.worker`, `gpu@worker`

### Jobs going to wrong workers

**Problem**: GPU jobs being processed by default workers

**Solution**:
1. Verify routing key is set correctly on job submission
2. Check worker configuration matches intended routing keys
3. Review queue keys in Redis to ensure correct routing

## Migration Guide

### Adding Routing to Existing System

**Phase 1: Deploy with backward compatibility**
```go
// No changes needed - all jobs default to "default" routing key
// All workers default to handling "default" routing key
```

**Phase 2: Introduce specialized workers**
```bash
# Start GPU workers alongside existing workers
WORKER_ROUTING_KEYS=gpu ./worker
```

**Phase 3: Route new jobs**
```go
// Start using routing for new GPU jobs
client.SubmitJobWithRoute("process_image", payload, job.PriorityHigh, "gpu")

// Existing jobs continue using default routing
client.SubmitJob("send_email", payload, job.PriorityNormal)
```

**Phase 4: Gradually migrate**
```go
// Migrate existing job submissions to use routing
client.SubmitJobWithRoute("send_email", payload, job.PriorityNormal, "email")
```

### Zero-Downtime Migration

1. Deploy queue changes (routing support)
2. Jobs without routing key automatically use "default"
3. Workers without routing config automatically use "default"
4. Start specialized workers with new routing keys
5. Begin submitting new jobs with routing keys
6. Old jobs continue processing on default workers
7. Gradually migrate all jobs to use routing keys

No service interruption required!

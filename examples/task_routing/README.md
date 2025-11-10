# Task Routing Example

This example demonstrates how to use task routing to direct jobs to specific worker pools.

## Overview

Task routing allows you to:
- Route GPU-intensive jobs to GPU workers
- Route email jobs to email workers
- Create specialized worker pools for different job types
- Scale different job types independently

## Running the Example

### 1. Start Redis

```bash
docker run -d -p 6379:6379 redis:7-alpine
```

### 2. Start Specialized Workers

**GPU Worker** (processes only GPU jobs):
```bash
WORKER_ROUTING_KEYS=gpu go run examples/task_routing/gpu_worker/main.go
```

**Email Worker** (processes only email jobs):
```bash
WORKER_ROUTING_KEYS=email go run examples/task_routing/email_worker/main.go
```

**Default Worker** (processes default jobs + can handle GPU/email overflow):
```bash
WORKER_ROUTING_KEYS=default,gpu,email go run examples/task_routing/default_worker/main.go
```

### 3. Submit Jobs

```bash
go run examples/task_routing/client/main.go
```

## How It Works

### Job Routing

Jobs are submitted with a routing key:

```go
// Submit GPU job
jobID, err := client.SubmitJobWithRoute(
    "process_image",
    payload,
    job.PriorityHigh,
    "gpu", // routing key
)

// Submit email job
jobID, err := client.SubmitJobWithRoute(
    "send_email",
    payload,
    job.PriorityNormal,
    "email", // routing key
)
```

### Worker Configuration

Workers specify which routing keys they handle:

```bash
# GPU worker - only processes GPU jobs
WORKER_ROUTING_KEYS=gpu

# Email worker - only processes email jobs
WORKER_ROUTING_KEYS=email

# Default worker - handles multiple routing keys (prioritized in order)
WORKER_ROUTING_KEYS=default,gpu,email
```

### Priority Ordering

When a worker handles multiple routing keys, jobs are dequeued in this order:
1. First routing key, high priority
2. First routing key, normal priority
3. First routing key, low priority
4. Second routing key, high priority
5. ...and so on

Example for `WORKER_ROUTING_KEYS=gpu,default`:
```
gpu:high -> gpu:normal -> gpu:low -> default:high -> default:normal -> default:low
```

## Architecture

```
                      ┌─────────────┐
                      │   Client    │
                      └──────┬──────┘
                             │
                ┌────────────┴────────────┐
                │                         │
         routing_key=gpu          routing_key=email
                │                         │
                ▼                         ▼
        ┌───────────────┐         ┌───────────────┐
        │ Redis Queues  │         │ Redis Queues  │
        │  gpu:high     │         │  email:high   │
        │  gpu:normal   │         │  email:normal │
        │  gpu:low      │         │  email:low    │
        └───────┬───────┘         └───────┬───────┘
                │                         │
                ▼                         ▼
        ┌───────────────┐         ┌───────────────┐
        │  GPU Worker   │         │ Email Worker  │
        │ (routes: gpu) │         │ (routes:email)│
        └───────────────┘         └───────────────┘
```

## Use Cases

### 1. Resource Isolation
Route GPU jobs to GPU-enabled machines:
```bash
WORKER_ROUTING_KEYS=gpu ./worker  # Run on GPU machine
```

### 2. Scaling Specific Job Types
Scale email workers independently:
```bash
# Start 10 email workers
for i in {1..10}; do
  WORKER_ROUTING_KEYS=email ./worker &
done
```

### 3. Workload Segregation
Separate critical jobs from regular jobs:
```bash
WORKER_ROUTING_KEYS=critical  # Dedicated high-SLA workers
WORKER_ROUTING_KEYS=default   # Regular workers
```

### 4. Fallback Workers
Create workers that handle overflow:
```bash
# Specialized GPU worker
WORKER_ROUTING_KEYS=gpu

# General worker that can handle GPU jobs when GPU workers are busy
WORKER_ROUTING_KEYS=default,gpu
```

## Best Practices

1. **Use descriptive routing keys**: `gpu`, `email`, `high_memory`, `us-east-1`
2. **Keep routing keys short**: Max 64 characters, alphanumeric + underscore/hyphen
3. **Plan capacity**: Monitor queue depth per routing key
4. **Use fallback workers**: Include `default` in multi-key workers for load balancing
5. **Test routing**: Verify jobs go to the correct workers

## Monitoring

Check queue depth by routing key:
```bash
redis-cli LLEN bananas:route:gpu:queue:high
redis-cli LLEN bananas:route:email:queue:normal
```

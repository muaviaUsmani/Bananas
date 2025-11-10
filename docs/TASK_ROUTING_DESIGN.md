# Task Routing Design

## Overview

This document describes the design and implementation of task routing in Bananas, enabling jobs to be directed to specific workers based on routing keys. This feature allows resource isolation, specialized worker pools, and workload distribution across different worker groups.

## Motivation

### Problem Statement

Currently, all jobs go into the same priority-based queues (high, normal, low), and any worker can pick up any job. This creates several problems:

1. **No Resource Isolation**: GPU-intensive jobs compete with CPU jobs for worker slots
2. **No Specialization**: Workers that are optimized for specific tasks (e.g., GPU workers) can't be dedicated to those tasks
3. **No Workload Segregation**: Critical jobs can't be guaranteed to run on dedicated worker pools
4. **Scaling Challenges**: Can't scale specific job types independently

### Use Cases

1. **GPU vs CPU Jobs**: Route image processing to GPU workers, other jobs to CPU workers
2. **Critical vs Regular Jobs**: Route critical jobs to dedicated high-SLA worker pools
3. **Geographic Distribution**: Route jobs to workers in specific regions/data centers
4. **Resource Requirements**: Route memory-intensive jobs to high-memory workers
5. **Team Segregation**: Route jobs from different teams/tenants to separate worker pools

## Design Goals

1. **Backward Compatible**: Existing jobs and workers should continue working without changes
2. **Simple to Use**: Easy to route jobs, easy to configure workers
3. **Efficient**: No performance degradation for routing
4. **Flexible**: Support multiple routing keys per worker
5. **Observable**: Easy to monitor routing behavior

## Architecture

### Components

```
┌─────────────────────────────────────────────────────────────────┐
│                          Client SDK                              │
│  SubmitJob(name, payload, priority, routingKey="default")       │
└─────────────────────────┬───────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                       Redis Queue                                │
│                                                                   │
│  Priority Queues (per routing key):                             │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ bananas:route:default:queue:high                         │  │
│  │ bananas:route:default:queue:normal                       │  │
│  │ bananas:route:default:queue:low                          │  │
│  └──────────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ bananas:route:gpu:queue:high                             │  │
│  │ bananas:route:gpu:queue:normal                           │  │
│  │ bananas:route:gpu:queue:low                              │  │
│  └──────────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ bananas:route:email:queue:high                           │  │
│  │ bananas:route:email:queue:normal                         │  │
│  │ bananas:route:email:queue:low                            │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                   │
│  Shared:                                                         │
│  - bananas:queue:processing (all jobs, regardless of route)     │
│  - bananas:queue:dead (DLQ, all failed jobs)                    │
│  - bananas:queue:scheduled (scheduled jobs with routing key)    │
│  - bananas:job:{id} (job data with routing key field)           │
└─────────────────────────┬───────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Worker Pools                                │
│                                                                   │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐   │
│  │ Default Workers│  │  GPU Workers   │  │ Email Workers  │   │
│  │                │  │                │  │                │   │
│  │ Routes:        │  │ Routes:        │  │ Routes:        │   │
│  │ - default      │  │ - gpu          │  │ - email        │   │
│  │                │  │ - default      │  │ - default      │   │
│  │                │  │                │  │                │   │
│  │ Handlers:      │  │ Handlers:      │  │ Handlers:      │   │
│  │ - all jobs     │  │ - gpu jobs     │  │ - email jobs   │   │
│  └────────────────┘  └────────────────┘  └────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### Data Model

#### Job Structure

Add `RoutingKey` field to existing Job struct:

```go
type Job struct {
    ID           string          `json:"id"`
    Name         string          `json:"name"`
    Description  string          `json:"description,omitempty"`
    Payload      json.RawMessage `json:"payload"`
    Status       JobStatus       `json:"status"`
    Priority     JobPriority     `json:"priority"`
    RoutingKey   string          `json:"routing_key"` // NEW: routing key for worker selection
    CreatedAt    time.Time       `json:"created_at"`
    UpdatedAt    time.Time       `json:"updated_at"`
    ScheduledFor *time.Time      `json:"scheduled_for,omitempty"`
    Attempts     int             `json:"attempts"`
    MaxRetries   int             `json:"max_retries"`
    Error        string          `json:"error,omitempty"`
}
```

**Default Value**: "default" (for backward compatibility)

#### Redis Key Structure

**Queue Keys (per routing key + priority):**
```
bananas:route:{routing_key}:queue:high
bananas:route:{routing_key}:queue:normal
bananas:route:{routing_key}:queue:low
```

**Shared Keys (unchanged):**
```
bananas:queue:processing   # All jobs being processed
bananas:queue:dead         # Dead letter queue (all failed jobs)
bananas:queue:scheduled    # Scheduled jobs (all routing keys)
bananas:job:{id}           # Job data (includes routing key)
```

**Key Design Rationale:**
- Routing key comes first in the key structure for better Redis key scanning
- Each routing key has its own priority queues (high, normal, low)
- Processing, dead letter, and scheduled queues are shared (routing is in job data)
- Job data includes routing key for full job context

### Queue Operations

#### Enqueue

```go
func (q *RedisQueue) Enqueue(ctx context.Context, j *Job) error {
    // Default routing key if not specified
    if j.RoutingKey == "" {
        j.RoutingKey = "default"
    }

    // Determine queue key based on routing key + priority
    queueKey := q.routeQueueKey(j.RoutingKey, j.Priority)

    // Store job data
    jobKey := q.jobKey(j.ID)
    jobData, err := json.Marshal(j)
    if err != nil {
        return err
    }

    // Use pipeline for atomicity
    pipe := q.client.Pipeline()
    pipe.Set(ctx, jobKey, jobData, 0)
    pipe.LPush(ctx, queueKey, j.ID)
    _, err = pipe.Exec(ctx)

    return err
}
```

#### Dequeue (Worker Side)

```go
func (q *RedisQueue) Dequeue(ctx context.Context, routingKeys []string, timeout time.Duration) (*Job, error) {
    // Build list of queue keys to check based on routing keys and priorities
    var queueKeys []string
    for _, routingKey := range routingKeys {
        queueKeys = append(queueKeys,
            q.routeQueueKey(routingKey, PriorityHigh),
            q.routeQueueKey(routingKey, PriorityNormal),
            q.routeQueueKey(routingKey, PriorityLow),
        )
    }

    // Use BRPOPLPUSH to atomically move from queue to processing
    result, err := q.client.BRPopLPush(ctx, queueKeys, q.processingKey, timeout).Result()
    if err == redis.Nil {
        return nil, nil // Timeout, no job available
    }
    if err != nil {
        return nil, err
    }

    jobID := result

    // Fetch job data
    jobData, err := q.client.Get(ctx, q.jobKey(jobID)).Bytes()
    if err != nil {
        return nil, err
    }

    var j Job
    if err := json.Unmarshal(jobData, &j); err != nil {
        return nil, err
    }

    return &j, nil
}
```

**Priority Ordering with Multiple Routing Keys:**

When a worker handles multiple routing keys (e.g., ["gpu", "default"]), the dequeue order is:
1. gpu:high
2. gpu:normal
3. gpu:low
4. default:high
5. default:normal
6. default:low

This ensures:
- GPU jobs are preferred over default jobs
- Within each routing key, priority is respected
- Fair processing across routing keys

### Worker Configuration

#### Config Structure

```go
type WorkerConfig struct {
    RoutingKeys []string // Routing keys this worker handles (default: ["default"])
    // ... other config fields
}
```

#### Environment Variable

```bash
# Single routing key
WORKER_ROUTING_KEYS=gpu

# Multiple routing keys (comma-separated)
WORKER_ROUTING_KEYS=gpu,default

# Default if not specified
WORKER_ROUTING_KEYS=default
```

#### Worker Startup

```go
func main() {
    config := loadConfig()

    // Default to "default" routing key if not specified
    if len(config.RoutingKeys) == 0 {
        config.RoutingKeys = []string{"default"}
    }

    log.Printf("Worker handling routing keys: %v", config.RoutingKeys)

    pool := worker.NewPool(queue, executor, config.RoutingKeys, concurrency)
    pool.Start()
}
```

### Client SDK

#### SubmitJob with Routing

```go
// Submit job with routing key
jobID, err := client.SubmitJobWithRoute(
    "process_image",
    payload,
    job.PriorityHigh,
    "gpu", // routing key
)

// Submit job with default routing (backward compatible)
jobID, err := client.SubmitJob(
    "send_email",
    payload,
    job.PriorityNormal,
    // routing key defaults to "default"
)
```

### Scheduler Integration

The scheduler needs to route scheduled jobs correctly when moving them to ready queues:

```go
func (s *Scheduler) MoveScheduledToReady(ctx context.Context) error {
    // Get ready jobs
    readyJobs := fetchReadyJobs()

    for _, job := range readyJobs {
        // Use job's routing key when enqueuing
        queueKey := q.routeQueueKey(job.RoutingKey, job.Priority)

        // Move to appropriate routed queue
        pipe.LPush(ctx, queueKey, job.ID)
        pipe.ZRem(ctx, scheduledSetKey, job.ID)
    }

    pipe.Exec(ctx)
}
```

## Implementation Strategy

### Phase 1: Core Routing Infrastructure (Day 1)

1. ✅ **Design Document** - This document
2. **Job Type Extension**
   - Add `RoutingKey` field to Job struct
   - Update `NewJob()` to accept optional routing key
   - Add validation for routing key format (alphanumeric + underscore)
   - Default to "default" for backward compatibility

3. **Queue Extension**
   - Add `routeQueueKey()` helper method
   - Update `Enqueue()` to use routing-aware queue keys
   - Update `Dequeue()` to accept routing keys array
   - Update key generation to support routing prefix

4. **Configuration**
   - Add `RoutingKeys` to worker config
   - Add environment variable parsing
   - Add validation for routing keys

### Phase 2: Worker and Client Integration (Day 1-2)

5. **Worker Pool Updates**
   - Update worker pool to accept routing keys
   - Pass routing keys to dequeue operations
   - Update worker startup logging

6. **Client SDK Extensions**
   - Add `SubmitJobWithRoute()` method
   - Keep `SubmitJob()` backward compatible (defaults to "default")
   - Update documentation

7. **Scheduler Updates**
   - Update scheduled job processing to respect routing keys
   - Ensure scheduled jobs go to correct routed queues

### Phase 3: Testing and Documentation (Day 2)

8. **Comprehensive Tests**
   - Unit tests for routing key validation
   - Queue tests for routed enqueue/dequeue
   - Integration tests for multi-key workers
   - Worker pool tests with routing
   - Backward compatibility tests

9. **Examples**
   - GPU vs CPU worker example
   - Multi-routing-key worker example
   - Email worker example

10. **Documentation**
    - Usage guide
    - Configuration reference
    - Migration guide
    - Best practices

## Backward Compatibility

### Existing Jobs

Jobs without a `RoutingKey` field will:
1. Be assigned "default" routing key automatically
2. Go to `bananas:route:default:queue:{priority}` queues
3. Be processed by workers handling the "default" routing key

### Existing Workers

Workers not configured with routing keys will:
1. Default to `RoutingKeys: ["default"]`
2. Process jobs from default routing key queues
3. Continue working as before

### Migration Path

**Zero-downtime migration:**

1. **Deploy queue changes first**
   - New code recognizes routing keys
   - Defaults all jobs to "default"
   - No impact on existing jobs

2. **Deploy workers**
   - Workers default to ["default"] routing key
   - Continue processing all existing jobs

3. **Gradually introduce routing**
   - Start submitting new jobs with specific routing keys
   - Start specialized workers with specific routing keys
   - Old jobs continue processing on default workers

## Performance Considerations

### Memory

- **Minimal overhead**: One routing key string per job (~10-20 bytes)
- **Queue keys**: Number of keys = routing_keys × priorities (3 per routing key)
- **Example**: 10 routing keys = 30 queue keys (negligible Redis memory)

### Latency

- **No degradation**: Routing key is just part of the Redis key
- **String concatenation**: Pre-computed keys avoid repeated allocations
- **Dequeue**: BRPOPLPUSH supports multiple keys natively (no additional round trips)

### Throughput

- **No impact**: Routing is a key namespace change, not additional operations
- **Benefits**: Better distribution can actually improve throughput
- **Scalability**: Each routing key can be scaled independently

## Monitoring and Observability

### Metrics

Add routing key label to existing metrics:

```go
metrics.RecordJobEnqueued(priority, routingKey)
metrics.RecordJobDequeued(priority, routingKey)
metrics.RecordJobCompleted(priority, routingKey)
metrics.RecordJobFailed(priority, routingKey)
```

### Logging

```go
log.Printf("Enqueued job %s to routing key '%s' with priority %s", jobID, routingKey, priority)
log.Printf("Worker handling routing keys: %v", routingKeys)
log.Printf("Dequeued job %s from routing key '%s'", jobID, routingKey)
```

### Queue Depth Monitoring

Monitor queue depth per routing key:

```bash
# Redis command to check queue depth
LLEN bananas:route:gpu:queue:high
LLEN bananas:route:gpu:queue:normal
LLEN bananas:route:gpu:queue:low
```

## Security Considerations

### Routing Key Validation

```go
func validateRoutingKey(key string) error {
    if key == "" {
        return fmt.Errorf("routing key cannot be empty")
    }
    if len(key) > 64 {
        return fmt.Errorf("routing key too long (max 64 chars)")
    }
    // Alphanumeric + underscore + hyphen only
    if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(key) {
        return fmt.Errorf("invalid routing key format")
    }
    return nil
}
```

### Resource Isolation

- Workers can only process jobs from their configured routing keys
- No way for jobs to "escape" to different routing keys once enqueued
- Tenants can be isolated using routing keys + worker pools

## Best Practices

### Naming Conventions

```go
// Good routing key names
"gpu"           // Short, descriptive
"high_memory"   // Underscore separator
"us-east-1"     // Geographic region
"team-alpha"    // Team isolation
"critical"      // Priority-based

// Avoid
"GPU_WORKER_POOL_FOR_IMAGE_PROCESSING"  // Too long
"g"                                      // Too short
"team@alpha"                             // Invalid chars
```

### Worker Configuration

```yaml
# Specialized worker (handles only GPU jobs)
worker:
  routing_keys: ["gpu"]
  handlers:
    - process_image
    - train_model

# Default worker (handles general jobs + GPU fallback)
worker:
  routing_keys: ["default", "gpu"]
  handlers:
    - send_email
    - process_data
    - process_image  # Can handle GPU jobs if GPU workers are busy

# Email-only worker
worker:
  routing_keys: ["email"]
  handlers:
    - send_email
```

### Capacity Planning

1. **Dedicated Pools**: Critical jobs should have dedicated routing keys
2. **Fallback Workers**: Include "default" in specialized workers for load balancing
3. **Monitoring**: Track queue depth per routing key
4. **Scaling**: Scale workers independently per routing key

## Future Enhancements

1. **Dynamic Routing**: Route jobs based on payload content
2. **Weighted Routing**: Priority-weighted dequeue across routing keys
3. **Queue Binding**: Celery-style queue binding configuration
4. **Auto-routing**: Automatic routing based on job name patterns
5. **Routing Metrics Dashboard**: Visualize routing behavior
6. **Rate Limiting per Routing Key**: Throttle specific job types

## Testing Strategy

### Unit Tests

- Routing key validation
- Queue key generation
- Job creation with routing key
- Config parsing

### Integration Tests

- Enqueue to routed queues
- Dequeue from multiple routing keys
- Priority ordering with routing
- Backward compatibility (no routing key)

### End-to-End Tests

- GPU worker scenario
- Multi-key worker scenario
- Routing with scheduling
- Routing with retries
- Migration scenario (old to new)

## Summary

Task routing provides:
- ✅ **Resource Isolation**: Dedicated worker pools for specific job types
- ✅ **Scalability**: Independent scaling per job type
- ✅ **Flexibility**: Workers can handle multiple routing keys
- ✅ **Backward Compatible**: Zero-downtime migration
- ✅ **Simple to Use**: Just add a routing key to jobs and workers
- ✅ **Performant**: No overhead, native Redis support

This brings Bananas to feature parity with Celery's task routing capabilities while maintaining simplicity and performance.

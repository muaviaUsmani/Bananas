# Result Backend Design

## Overview

The result backend allows clients to retrieve results from completed jobs. This enables RPC-style task execution where clients can submit jobs and wait for results, similar to Celery's result backend.

## Requirements

1. Store job results in Redis with configurable TTL
2. Retrieve results by job ID
3. Support blocking wait for results (SubmitAndWait)
4. Handle both successful and failed job outcomes
5. Automatic result expiration
6. Minimal performance impact on workers

## Architecture

### Components

1. **JobResult** - New type representing a job's result
2. **ResultBackend** - Interface for storing/retrieving results
3. **RedisResultBackend** - Redis implementation
4. **Client extensions** - SubmitAndWait, GetResult methods
5. **Worker integration** - Automatic result storage after job execution

### Data Model

#### JobResult Type

```go
type JobResult struct {
    JobID       string          // Job identifier
    Status      JobStatus       // completed or failed
    Result      json.RawMessage // Result data (for successful jobs)
    Error       string          // Error message (for failed jobs)
    CompletedAt time.Time       // When the job completed
    Duration    time.Duration   // How long the job took to execute
}
```

#### Redis Storage

**Key Pattern:**
```
bananas:result:{job_id}
```

**Data Structure:** Redis Hash

**Fields:**
- `status`: "completed" or "failed"
- `result`: JSON-encoded result data
- `error`: Error message (if failed)
- `completed_at`: RFC3339 timestamp
- `duration_ms`: Execution duration in milliseconds

**TTL:** Configurable (default: 1 hour for completed, 24 hours for failed)

**Notification Channel:**
```
bananas:result:notify:{job_id}
```

Used for pub/sub notifications when result is ready.

## API Design

### ResultBackend Interface

```go
type ResultBackend interface {
    // StoreResult stores a job result in Redis
    StoreResult(ctx context.Context, result *JobResult) error

    // GetResult retrieves a result by job ID
    // Returns nil if result doesn't exist yet
    GetResult(ctx context.Context, jobID string) (*JobResult, error)

    // WaitForResult blocks until result is available or timeout
    WaitForResult(ctx context.Context, jobID string, timeout time.Duration) (*JobResult, error)

    // DeleteResult removes a result from Redis
    DeleteResult(ctx context.Context, jobID string) error
}
```

### Client SDK Extensions

```go
// SubmitAndWait submits a job and waits for its result
func (c *Client) SubmitAndWait(
    ctx context.Context,
    name string,
    payload interface{},
    priority JobPriority,
    timeout time.Duration,
) (*JobResult, error)

// GetResult retrieves a job result by ID
func (c *Client) GetResult(ctx context.Context, jobID string) (*JobResult, error)
```

### Configuration

Environment variables:
- `RESULT_BACKEND_ENABLED`: Enable result storage (default: true)
- `RESULT_BACKEND_TTL_SUCCESS`: TTL for successful results (default: 1h)
- `RESULT_BACKEND_TTL_FAILURE`: TTL for failed results (default: 24h)

## Implementation Strategy

### Phase 1: Basic Result Storage

1. Create `JobResult` type in `internal/job/result.go`
2. Create `ResultBackend` interface in `internal/result/backend.go`
3. Implement `RedisResultBackend` in `internal/result/redis.go`
4. Update worker to store results after job completion

### Phase 2: Blocking Wait

Two approaches considered:

**Option A: Redis Pub/Sub (Chosen)**
- Worker publishes notification to channel when result ready
- Client subscribes to channel and waits with timeout
- More efficient, lower latency
- Requires pub/sub connection management

**Option B: Polling**
- Client polls GetResult() every 100ms until result appears
- Simpler implementation
- Higher Redis load
- Higher latency

**Decision:** Use Pub/Sub for better performance, with polling as fallback.

### Phase 3: Client SDK Integration

1. Add `SubmitAndWait()` method to client
2. Add `GetResult()` method to client
3. Handle timeouts gracefully
4. Add context cancellation support

## Redis Operations

### Storing a Result

```
MULTI
  HSET bananas:result:{job_id} status completed result {...} completed_at {ts} duration_ms 1234
  EXPIRE bananas:result:{job_id} 3600
  PUBLISH bananas:result:notify:{job_id} "ready"
EXEC
```

### Retrieving a Result

```
HGETALL bananas:result:{job_id}
```

### Waiting for Result (Pub/Sub)

```
SUBSCRIBE bananas:result:notify:{job_id}
# Wait for message or timeout
# Then: HGETALL bananas:result:{job_id}
```

### Waiting for Result (Polling Fallback)

```
# Loop with 100ms interval:
HGETALL bananas:result:{job_id}
# If empty, sleep 100ms and retry
# Continue until result appears or timeout
```

## Worker Integration

### Executor Updates

In `internal/worker/executor.go`:

```go
func (e *Executor) Execute(ctx context.Context, j *job.Job) error {
    startTime := time.Now()

    // Execute job handler
    var result interface{}
    err := handler(ctx, j, &result)

    duration := time.Since(startTime)

    // Store result if enabled
    if e.resultBackend != nil {
        jobResult := &job.JobResult{
            JobID:       j.ID,
            CompletedAt: time.Now(),
            Duration:    duration,
        }

        if err != nil {
            jobResult.Status = job.StatusFailed
            jobResult.Error = err.Error()
        } else {
            jobResult.Status = job.StatusCompleted
            resultBytes, _ := json.Marshal(result)
            jobResult.Result = resultBytes
        }

        if err := e.resultBackend.StoreResult(ctx, jobResult); err != nil {
            log.Warn("Failed to store result", "job_id", j.ID, "error", err)
        }
    }

    return err
}
```

### Handler Signature Update (Optional)

**Current:**
```go
type JobHandler func(ctx context.Context, j *job.Job) error
```

**Enhanced (for result support):**
```go
type JobHandler func(ctx context.Context, j *job.Job, result *interface{}) error
```

Or keep current signature and use a separate mechanism for results.

**Decision:** Add optional result parameter via context or new handler type.

## Job Handler Result Return

### Approach 1: Return Value (Chosen)

Modify handler signature:
```go
type JobHandlerWithResult func(ctx context.Context, j *job.Job) (interface{}, error)
```

Workers can support both old and new handler types:
- Old handlers: Store nil result
- New handlers: Store returned value

### Approach 2: Context-Based

Handler stores result in context:
```go
func handler(ctx context.Context, j *job.Job) error {
    result := map[string]interface{}{"count": 42}
    result.StoreResult(ctx, result)
    return nil
}
```

**Decision:** Use Approach 1 (return value) for simplicity and type safety.

## Error Handling

1. **Result storage failure**: Log warning, don't fail job
2. **Result retrieval failure**: Return error to client
3. **Timeout waiting for result**: Return timeout error
4. **Job still running**: Return nil result with no error
5. **Pub/sub connection failure**: Fall back to polling

## Performance Considerations

1. **Redis Load**: Results add 1 HSET + 1 EXPIRE + 1 PUBLISH per job
2. **Memory**: Results consume Redis memory (mitigated by TTL)
3. **Latency**: Pub/sub adds <10ms overhead
4. **Scalability**: Independent of job volume (results are optional)

### Optimization Strategies

1. Make result backend optional (can disable)
2. Use pipeline for HSET + EXPIRE + PUBLISH (1 round trip)
3. Configure aggressive TTL for high-volume jobs
4. Support result-less jobs (handler doesn't return value)

## Security Considerations

1. **Result access**: No authentication (anyone with job ID can read result)
2. **Result size**: Limit result data to 10MB (prevent memory exhaustion)
3. **TTL enforcement**: Always set TTL to prevent unbounded growth

## Testing Strategy

### Unit Tests

1. `result_test.go`: JobResult marshaling/unmarshaling
2. `backend_test.go`: ResultBackend interface tests
3. `redis_test.go`: Redis implementation tests (with miniredis)

### Integration Tests

1. End-to-end: Submit job, wait for result, verify data
2. Timeout handling: Verify timeout behavior
3. Concurrent access: Multiple clients waiting for same result
4. Pub/sub failure: Verify polling fallback

### Performance Tests

1. Measure overhead of result storage
2. Measure wait latency (pub/sub vs polling)
3. Measure memory impact of retained results

## Migration Path

1. Feature is opt-in (workers store results only if enabled)
2. Old clients continue to work (backward compatible)
3. New clients can use SubmitAndWait() for RPC-style execution
4. No schema changes to existing Job type

## Example Usage

### Simple Result Retrieval

```go
// Submit job
client := bananas.NewClient("redis://localhost:6379")
jobID, err := client.SubmitJob("process_data", payload, job.PriorityNormal)

// Later: Get result
result, err := client.GetResult(context.Background(), jobID)
if err != nil {
    // Handle error
}

if result == nil {
    // Job not yet complete
} else if result.Status == job.StatusCompleted {
    // Success! Use result.Result
    var data MyResultType
    json.Unmarshal(result.Result, &data)
} else {
    // Job failed, check result.Error
}
```

### RPC-Style Execution

```go
// Submit and wait (blocks until complete or timeout)
client := bananas.NewClient("redis://localhost:6379")
result, err := client.SubmitAndWait(
    context.Background(),
    "calculate_score",
    payload,
    job.PriorityHigh,
    30*time.Second, // timeout
)

if err != nil {
    // Timeout or other error
}

if result.Status == job.StatusCompleted {
    // Use result.Result
    var score float64
    json.Unmarshal(result.Result, &score)
    fmt.Printf("Score: %.2f\n", score)
}
```

### Job Handler with Result

```go
// Old style (still supported)
func OldHandler(ctx context.Context, j *job.Job) error {
    // Process job
    return nil
}

// New style (returns result)
func NewHandler(ctx context.Context, j *job.Job) (interface{}, error) {
    // Process job
    result := map[string]interface{}{
        "status": "success",
        "count":  42,
    }
    return result, nil
}

// Register handlers
worker.RegisterHandler("old_task", OldHandler)
worker.RegisterHandlerWithResult("new_task", NewHandler)
```

## Implementation Checklist

- [ ] Create JobResult type
- [ ] Create ResultBackend interface
- [ ] Implement RedisResultBackend
- [ ] Add configuration options
- [ ] Update worker to store results
- [ ] Add GetResult to client
- [ ] Add SubmitAndWait to client
- [ ] Implement pub/sub notification
- [ ] Implement polling fallback
- [ ] Write comprehensive tests
- [ ] Write documentation
- [ ] Create usage examples

## Future Enhancements

1. **Result streaming**: Support large results via chunking
2. **Result compression**: Compress large results in Redis
3. **Result metadata**: Add more fields (worker ID, hostname, etc.)
4. **Result callbacks**: Notify external systems when result ready
5. **Result persistence**: Optional permanent storage (S3, database)
6. **Result grouping**: Batch results for multi-job operations

## Comparison with Celery

| Feature | Bananas | Celery |
|---------|---------|--------|
| Result storage | Redis | Redis, Database, RPC |
| Result retrieval | Blocking/Non-blocking | Blocking/Non-blocking |
| Result TTL | Configurable | Configurable |
| Result format | JSON | Pickle, JSON, YAML |
| Notification | Pub/Sub | Polling |
| Handler result | Return value | Return value |

Bananas aims for simplicity with Redis-only storage, JSON format, and pub/sub notifications.

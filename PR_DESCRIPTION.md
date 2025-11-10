# Task 3.3: Result Backend - Store and Retrieve Job Results

## Overview

This PR implements a comprehensive result backend system for Bananas, enabling clients to retrieve job results after execution. This feature supports both RPC-style synchronous execution and asynchronous result polling patterns, making Bananas suitable for request/response workflows.

## Problem Statement

Previously, Bananas only supported fire-and-forget job execution. Clients had no way to:
- Wait for job completion and retrieve results
- Check job execution status after submission
- Implement synchronous request/response patterns over the async queue
- Retrieve error details from failed jobs

This limitation prevented use cases like API endpoints that need to return job results, batch processing workflows that collect results, and debugging failed jobs.

## Solution

Implemented a complete result backend system with the following components:

### 1. Core Result Backend (`internal/result/`)

**New Files:**
- `backend.go` - Backend interface definition for pluggable implementations
- `redis.go` - Redis-based backend with pub/sub for efficient result waiting

**Key Features:**
- Redis hash storage: `bananas:result:{job_id}`
- Pub/sub notifications: `bananas:result:notify:{job_id}`
- Configurable TTL (1h for success, 24h for failure)
- Pipeline operations for atomic HSET + EXPIRE + PUBLISH
- Efficient waiting with pub/sub (no polling overhead)

### 2. JobResult Type (`internal/job/result.go`)

**New Type:**
```go
type JobResult struct {
    JobID       string
    Status      JobStatus
    Result      json.RawMessage
    Error       string
    CompletedAt time.Time
    Duration    time.Duration
}
```

**Helper Methods:**
- `IsSuccess()` - Check if job completed successfully
- `IsFailed()` - Check if job failed
- `UnmarshalResult()` - Parse result data into typed structure

### 3. Client SDK Extensions (`pkg/client/client.go`)

**New Methods:**
- `GetResult(ctx, jobID)` - Retrieve result (returns nil if not ready)
- `SubmitAndWait(ctx, name, payload, priority, timeout)` - RPC-style execution
- `NewClientWithConfig(redisURL, successTTL, failureTTL)` - Custom TTL configuration

**Usage Patterns:**

**RPC-style (synchronous):**
```go
result, err := client.SubmitAndWait(ctx, "job_name", payload, priority, 30*time.Second)
if result.IsSuccess() {
    fmt.Printf("Duration: %v\n", result.Duration)
}
```

**Async (polling):**
```go
jobID, _ := client.SubmitJob("job_name", payload, priority)
// Later...
result, _ := client.GetResult(ctx, jobID)
if result != nil {
    // Job is complete
}
```

### 4. Worker Integration (`internal/worker/executor.go`)

**Changes:**
- Added `resultBackend` field to Executor
- `SetResultBackend()` method to enable result storage
- Automatic result storage after job execution (success or failure)
- Best-effort storage pattern (failures logged but don't fail job)

**Modified:**
- `ExecuteJob()` now stores results for both success and failure cases
- Results include status, error messages, duration, and completion timestamp

### 5. Configuration (`internal/config/config.go`)

**New Settings:**
- `RESULT_BACKEND_ENABLED` (default: true)
- `RESULT_BACKEND_TTL_SUCCESS` (default: 1h)
- `RESULT_BACKEND_TTL_FAILURE` (default: 24h)

### 6. Design Documentation (`docs/RESULT_BACKEND_DESIGN.md`)

Comprehensive design document covering:
- Architecture and component interactions
- Data model and Redis storage patterns
- API design decisions
- Pub/sub vs polling trade-offs
- Implementation strategy (3-phase approach)
- Error handling and performance considerations
- Future enhancements

## Testing

### Unit Tests (17 tests, all passing)

**`internal/job/result_test.go`:**
- TestJobResult_IsSuccess (4 status types)
- TestJobResult_IsFailed (3 status types)
- TestJobResult_UnmarshalResult (4 scenarios: success with data, success no data, failed job, invalid JSON)
- TestJobResult_JSON (marshaling/unmarshaling)

**`internal/result/redis_test.go`:**
- TestNewRedisBackend
- TestRedisBackend_StoreAndGetResult_Success
- TestRedisBackend_StoreAndGetResult_Failure
- TestRedisBackend_GetResult_NotFound
- TestRedisBackend_WaitForResult_AlreadyExists
- TestRedisBackend_WaitForResult_Timeout
- TestRedisBackend_WaitForResult_Notified (pub/sub validation)
- TestRedisBackend_DeleteResult
- TestRedisBackend_DeleteResult_NotFound
- TestRedisBackend_TTL (2 subtests: success/failure TTL)

All tests use miniredis for isolated testing without external dependencies.

## Examples

### Complete Working Example (`examples/result_backend/`)

Demonstrates three usage patterns:

1. **RPC-style execution** - Submit and wait for result
2. **Async execution** - Submit job, poll for result later
3. **Batch operations** - Submit multiple jobs, collect all results

Includes comprehensive README with:
- Prerequisites and setup instructions
- Expected output
- Configuration options
- Use cases (API endpoints, webhooks, batch processing, RPC)
- Code snippets for common patterns

## Technical Highlights

### Efficient Result Waiting with Pub/Sub

Instead of polling, clients subscribe to a Redis pub/sub channel and receive notifications when results are ready:

```go
func (r *RedisBackend) WaitForResult(ctx, jobID, timeout) (*JobResult, error) {
    // Check if result already exists
    if result := r.GetResult(ctx, jobID); result != nil {
        return result, nil
    }

    // Subscribe to notification channel
    pubsub := r.client.Subscribe(ctx, notifyChannel)
    defer pubsub.Close()

    // Wait for notification or timeout
    select {
    case <-pubsub.Channel():
        return r.GetResult(ctx, jobID)
    case <-time.After(timeout):
        return nil, nil
    }
}
```

### Atomic Operations with Pipelines

Results are stored atomically using Redis pipelines (single round trip):

```go
pipe := r.client.Pipeline()
pipe.HSet(ctx, key, resultData)
pipe.Expire(ctx, key, ttl)
pipe.Publish(ctx, notifyChannel, "ready")
_, err := pipe.Exec(ctx)
```

### Best-Effort Storage Pattern

Result storage never fails the job execution:

```go
func (e *Executor) storeResult(...) {
    if e.resultBackend == nil {
        return // Result backend not configured
    }

    if err := e.resultBackend.StoreResult(ctx, result); err != nil {
        log.Printf("Failed to store result: %v", err)
        // Job continues normally
    }
}
```

## Backward Compatibility

This is a purely additive feature:
- Result backend is opt-in (enabled by default but can be disabled)
- No changes to existing job submission API
- No breaking changes to worker behavior
- Existing jobs continue working without modification

## Use Cases Enabled

1. **API Endpoints** - Submit job, return job ID, client polls for result
2. **Webhooks** - Submit job, store result, trigger webhook when complete
3. **Batch Processing** - Submit many jobs, collect all results
4. **Synchronous RPC** - Request/response pattern over async queue
5. **Debugging** - Retrieve error details from failed jobs

## Performance Considerations

- **Memory**: Results expire automatically via Redis TTL (1h success, 24h failure)
- **Network**: Pipeline operations minimize round trips (1 for store)
- **Latency**: Pub/sub provides instant notifications (no polling delay)
- **Scale**: Backend interface allows future implementations (disk, S3, etc.)

## Files Changed

### New Files
- `docs/RESULT_BACKEND_DESIGN.md` - Design documentation
- `internal/job/result.go` - JobResult type and methods
- `internal/result/backend.go` - Backend interface
- `internal/result/redis.go` - Redis implementation
- `internal/job/result_test.go` - JobResult tests
- `internal/result/redis_test.go` - Redis backend tests
- `examples/result_backend/main.go` - Working example
- `examples/result_backend/README.md` - Example documentation

### Modified Files
- `pkg/client/client.go` - Added GetResult, SubmitAndWait methods
- `internal/worker/executor.go` - Added result storage
- `cmd/worker/main.go` - Initialize result backend
- `internal/config/config.go` - Added result backend configuration

## Commits

1. `041a18a` - Add result backend core implementation
2. `5071a85` - Integrate result backend with worker and client
3. `5ffb415` - Add comprehensive tests for result backend
4. `a022825` - Add result backend example and documentation

## Testing Instructions

```bash
# Run all tests
go test ./...

# Run result backend tests specifically
go test ./internal/result/...
go test ./internal/job/...

# Run the example
cd examples/result_backend
go run main.go
```

## Next Steps

After this PR is merged, potential enhancements include:
- Result pagination for batch operations
- Result streaming for large payloads
- Additional backend implementations (disk, S3)
- Result retention policies
- Admin API for result management

## References

- Design Document: `docs/RESULT_BACKEND_DESIGN.md`
- Example: `examples/result_backend/`
- Project Plan: Task 3.3 in `PROJECT_PLAN.md`

# Result Backend Example

This example demonstrates the Bananas result backend for retrieving job results.

## Features Demonstrated

1. **RPC-style execution**: Submit a job and wait for its result
2. **Async execution**: Submit a job and check for results later
3. **Batch operations**: Submit multiple jobs and collect results

## Prerequisites

1. Redis running on `localhost:6379`
2. Worker running with result backend enabled (default)

## Running the Example

```bash
# Start Redis (if not already running)
redis-server

# Start a worker (in another terminal)
cd ../../cmd/worker
go run main.go

# Run the example
cd ../../examples/result_backend
go run main.go
```

## Expected Output

```
=== Bananas Result Backend Example ===

Example 1: Submit and Wait (RPC-style)
---------------------------------------
✓ Job completed successfully!
  Duration: 125ms
  Completed at: 2025-11-10T03:00:00Z

Example 2: Submit and Check Later
----------------------------------
Job submitted: abc123...
Waiting for completion...
  Still running... (1/10)
✓ Job completed after 245ms

Example 3: Batch Job Submission
--------------------------------
Submitted 5 jobs
Waiting for all to complete...
✓ Completed: 5/5 jobs

=== Example Complete ===
```

## How It Works

### RPC-Style Execution

```go
result, err := client.SubmitAndWait(ctx, "job_name", payload, priority, timeout)
if result.IsSuccess() {
    // Job completed successfully
    fmt.Printf("Result: %v\n", result.Duration)
}
```

### Async Execution

```go
// Submit job
jobID, err := client.SubmitJob("job_name", payload, priority)

// Later: Check for result
result, err := client.GetResult(ctx, jobID)
if result != nil {
    // Job is complete
}
```

## Configuration

Result backend is enabled by default with:
- Success TTL: 1 hour
- Failure TTL: 24 hours

To customize:

```bash
export RESULT_BACKEND_ENABLED=true
export RESULT_BACKEND_TTL_SUCCESS=2h
export RESULT_BACKEND_TTL_FAILURE=48h
```

## Result Structure

```go
type JobResult struct {
    JobID       string        // Job identifier
    Status      JobStatus     // completed or failed
    Result      []byte        // Result data (JSON)
    Error       string        // Error message (if failed)
    CompletedAt time.Time     // Completion timestamp
    Duration    time.Duration // Execution duration
}
```

## Use Cases

1. **API Endpoints**: Submit job, return job ID, client polls for result
2. **Webhooks**: Submit job, store result, trigger webhook when complete
3. **Batch Processing**: Submit many jobs, collect all results
4. **RPC**: Synchronous request/response pattern over async queue

## Notes

- Results are stored in Redis with TTL
- Pub/sub is used for efficient wait notifications
- Results expire automatically (1h success, 24h failure)
- Workers store results automatically when enabled

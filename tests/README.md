# Integration Tests (`tests/`)

This directory contains end-to-end integration tests that verify the complete Bananas task queue system works correctly when all components interact together.

## Purpose

Integration tests differ from unit tests by:
- Testing **interactions between multiple components**
- Verifying **complete workflows** from submission to completion
- Ensuring **data flows correctly** through the entire system
- Catching **integration issues** that unit tests might miss

## Test Structure

All integration tests are in `integration_test.go` and use the actual internal packages (not mocks) to simulate real-world usage.

---

## Tests (`integration_test.go`)

**6 comprehensive integration tests**:

### 1. ✅ TestFullWorkflow_EndToEnd

**Purpose**: Verifies the complete happy path from job submission to execution.

**What it tests**:
- Create client and submit 3 different job types
- Create handler registry and register 3 handlers
- Create executor with mock queue
- Execute all jobs successfully
- Verify execution completes without errors

**Job types tested**:
- `count_items` - JSON array counting
- `send_email` - Email simulation (2s execution)
- `process_data` - Data processing simulation (3s execution)

**Duration**: ~5 seconds (due to simulated work)

**Code flow**:
```
Client → Submit Jobs → Registry → Executor → Handlers → Success
```

---

### 2. ✅ TestFullWorkflow_WithDifferentPriorities

**Purpose**: Tests job submission with all three priority levels.

**What it tests**:
- Submit jobs with high, normal, and low priority
- Verify each job has correct priority set
- Execute jobs in submitted order
- Ensure priority field is preserved

**Priorities tested**:
- `PriorityHigh`
- `PriorityNormal`
- `PriorityLow`

**Note**: Actual priority-based dequeuing is tested in `internal/queue/redis_test.go`. This test verifies the priority field is correctly set and passed through.

**Duration**: <1 second

---

### 3. ✅ TestFullWorkflow_InvalidJobName

**Purpose**: Tests error handling when job name doesn't match any registered handler.

**What it tests**:
- Register handler for "valid_job"
- Submit job with name "unknown_job"
- Attempt execution
- Verify appropriate error returned
- Check job is marked as failed in queue

**Expected behavior**:
- Executor returns error: "no handler registered for job: unknown_job"
- Job is NOT executed
- Fail() is called on the queue

**Duration**: <1 second

---

### 4. ✅ TestFullWorkflow_HandlerFailure

**Purpose**: Tests error handling when a handler fails during execution.

**What it tests**:
- Create handler that returns an error
- Submit and execute job
- Verify error is propagated correctly
- Check Fail() is called with error message

**Handler behavior**:
```go
func failingHandler(ctx context.Context, j *job.Job) error {
    return errors.New("something went wrong")
}
```

**Expected behavior**:
- Executor catches handler error
- Returns error to caller
- Calls `queue.Fail()` with error message
- Job can be retried (if under max attempts)

**Duration**: <1 second

---

### 5. ✅ TestFullWorkflow_ConcurrentExecution

**Purpose**: Tests concurrent job execution and thread-safety.

**What it tests**:
- Submit 10 jobs to client concurrently
- Execute all jobs concurrently using goroutines
- Use WaitGroup to synchronize completion
- Verify no race conditions occur
- Check all jobs complete successfully

**Concurrency aspects**:
- Client's `SubmitJob()` is called from multiple goroutines
- Executor's `ExecuteJob()` is called concurrently
- Tests thread-safety of client's `sync.RWMutex`
- Simulates real-world concurrent worker pool

**Duration**: <1 second (jobs execute in parallel)

**What would fail**:
- Race conditions in client
- Shared state corruption
- Deadlocks in executor

---

### 6. ✅ TestFullWorkflow_ListAllJobs

**Purpose**: Tests job listing and retrieval functionality.

**What it tests**:
- Submit multiple jobs with different types
- Call `ListJobs()` to get all jobs
- Verify correct count returned
- Check job properties are accessible

**Jobs created**:
- "job_type_1" with high priority
- "job_type_2" with normal priority  
- "job_type_3" with low priority

**Verifications**:
- Exactly 3 jobs returned
- All jobs have unique IDs
- Job properties (name, priority) are correct

**Duration**: <1 second

---

## Running Integration Tests

### Run all tests (including unit tests)
```bash
make test
```

### Run only integration tests
```bash
go test ./tests/ -v
```

### Run specific integration test
```bash
go test ./tests/ -v -run TestFullWorkflow_EndToEnd
```

### Run with race detector (recommended for concurrency tests)
```bash
go test ./tests/ -race -v
```

### Run with coverage
```bash
go test ./tests/ -cover
```

---

## Test Dependencies

Integration tests depend on:
- `pkg/client` - For job submission
- `internal/job` - For job types and priorities
- `internal/worker` - For registry, executor, and handlers
- Standard library: `testing`, `context`, `sync`

**Mock components used**:
- `mockQueue` - Simulates Redis queue operations without real Redis
  - Tracks `Complete()` and `Fail()` calls
  - Allows testing without external dependencies

**Why mock the queue?**:
- Integration tests focus on job execution flow
- Real Redis is tested in `internal/queue/redis_test.go`
- Mocking makes tests faster and more reliable
- Removes external dependency for CI/CD

---

## Test Output Example

```bash
$ make test

Running tests in Docker (Go 1.23)...
?       github.com/muaviaUsmani/bananas/cmd/api          [no test files]
?       github.com/muaviaUsmani/bananas/cmd/scheduler    [no test files]
?       github.com/muaviaUsmani/bananas/cmd/worker       [no test files]
?       github.com/muaviaUsmani/bananas/internal/config  [no test files]
ok      github.com/muaviaUsmani/bananas/internal/job     0.013s
ok      github.com/muaviaUsmani/bananas/internal/queue   2.213s
ok      github.com/muaviaUsmani/bananas/internal/worker  9.546s
ok      github.com/muaviaUsmani/bananas/pkg/client       0.002s
ok      github.com/muaviaUsmani/bananas/tests            5.011s  ← Integration tests
```

---

## What Integration Tests Don't Cover

Integration tests focus on component interaction. These aspects are tested elsewhere:

**Covered in unit tests**:
- Redis connection failures (`internal/queue`)
- Job status transitions (`internal/job`)
- Worker pool concurrency limits (`internal/worker`)
- Handler registry edge cases (`internal/worker`)

**Not yet covered** (future work):
- API endpoint integration (requires HTTP server)
- Scheduler integration with real time delays
- Full system test with all services running in Docker
- Performance/load testing
- Network partition scenarios

---

## Adding New Integration Tests

When adding new tests, follow this pattern:

```go
func TestFullWorkflow_YourFeature(t *testing.T) {
    // 1. Setup client and dependencies
    c := client.NewClient()
    registry := worker.NewRegistry()
    mockQ := &mockQueue{}
    executor := worker.NewExecutor(registry, mockQ, 5)
    
    // 2. Register handlers
    registry.Register("your_job", YourHandler)
    
    // 3. Submit jobs
    jobID, err := c.SubmitJob("your_job", payload, job.PriorityNormal)
    if err != nil {
        t.Fatalf("failed to submit job: %v", err)
    }
    
    // 4. Execute
    j, _ := c.GetJob(jobID)
    ctx := context.Background()
    err = executor.ExecuteJob(ctx, j)
    
    // 5. Verify behavior
    if err != nil {
        t.Errorf("expected success, got error: %v", err)
    }
    
    // 6. Check side effects
    if mockQ.completeCalls != 1 {
        t.Errorf("expected Complete() called once")
    }
}
```

**Best practices**:
- Name tests `TestFullWorkflow_*` for consistency
- Test one workflow/scenario per test function
- Use descriptive failure messages
- Clean up resources (if any)
- Keep tests independent (no shared state between tests)

---

## CI/CD Integration

Integration tests are designed to run in CI pipelines:

**Docker-based execution** (recommended):
```bash
make test
```
- Uses Go 1.23 in Docker
- Consistent environment
- No local Go installation needed

**Local execution** (requires Go 1.21+):
```bash
make test-local
```

**Benefits for CI**:
- Fast execution (~5 seconds for integration tests)
- No external dependencies (Redis is mocked)
- Deterministic results
- Works in isolated containers

---

## Future Enhancements

### Planned integration tests:

1. **TestFullWorkflow_WithRealRedis**
   - Use testcontainers to spin up real Redis
   - Test actual queue operations
   - Verify data persistence

2. **TestFullWorkflow_SchedulerIntegration**
   - Submit failing jobs
   - Verify they go to scheduled set
   - Run scheduler
   - Check jobs are retried

3. **TestFullWorkflow_WorkerPoolUnderLoad**
   - Submit 1000 jobs
   - Start worker pool
   - Measure throughput
   - Verify all jobs complete

4. **TestFullWorkflow_GracefulShutdown**
   - Start worker pool with long-running job
   - Trigger shutdown
   - Verify job completes before exit
   - Check no jobs are lost

5. **TestFullWorkflow_ErrorRecovery**
   - Kill worker mid-execution
   - Verify job returns to queue
   - Restart worker
   - Check job is retried

---

## Test Coverage

Current integration test coverage:

| Component | Coverage |
|-----------|----------|
| Job submission | ✅ Full |
| Handler execution | ✅ Full |
| Error handling | ✅ Full |
| Concurrent execution | ✅ Full |
| Priority handling | ✅ Basic |
| Retry logic | ❌ Not yet (queue mock) |
| Dead letter queue | ❌ Not yet (queue mock) |
| Scheduler integration | ❌ Not yet |

**Overall**: Strong coverage of core execution flow. Redis-specific features are tested in `internal/queue/redis_test.go`.

---

## Performance Benchmarks

While not formal benchmarks, the integration tests provide insight into performance:

- **Job submission**: <1ms per job
- **Handler execution**: Depends on handler (2-3s for example handlers)
- **Concurrent execution**: 10 jobs in parallel complete in ~3s
- **Total test suite**: ~5 seconds

For production load testing, use dedicated benchmarking tools.

---

## Summary

**Total Tests**: 6 integration tests  
**Total Duration**: ~5 seconds  
**Purpose**: Verify complete workflows work end-to-end  
**Coverage**: Job submission → Handler execution → Completion  
**Dependencies**: Client, Job, Worker packages (with mocked queue)  

All tests pass consistently in both local and Docker environments.


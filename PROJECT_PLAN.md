# Bananas - Distributed Task Queue System
## Project Overview

Bananas is a distributed task queue system built with Go, Redis, and Docker. It enables asynchronous job processing across multiple workers with priority-based execution, automatic retries with exponential backoff, and scheduled job execution.

The system is designed for two deployment models:

1. **Self-Managed (Current Focus)**: Users import Bananas as a library in their Go projects, register job handlers in their code, and run workers alongside their applications
2. **Cloud-Managed (Future)**: SaaS model where users define handlers via web/CLI interface and submit jobs via API (similar to AWS Lambda)

## Current State

### ✅ What's Built

**Core Infrastructure (100% Complete)**
- Redis queue operations with atomic job handling (RPOPLPUSH pattern)
- Priority-based queues (High > Normal > Low)
- Exponential backoff retry mechanism with scheduled set
- Dead letter queue for permanently failed jobs
- Job model with comprehensive metadata tracking
- Configuration management via environment variables
- Docker containerization (dev + prod modes with hot reload)

**Worker System (95% Complete)**
- Handler registry for mapping job names to functions
- Job executor with timeout enforcement and context cancellation
- Example handlers demonstrating the pattern
- Graceful shutdown support
- Missing: Continuous polling loop to actually process jobs from Redis

**Client SDK (50% Complete)**
- Go client with clean API for job submission
- Currently in-memory only (not connected to Redis)
- Missing: Redis integration to make it truly distributed

**Testing (93.3% Coverage)**
- Comprehensive unit tests for all components
- Integration tests for end-to-end workflows
- Redis operations tested with miniredis

**Documentation**
- README with setup instructions
- Integration guide for users
- Individual package documentation
- Docker setup well-documented

### ❌ What's Missing for End-to-End Usability

**Critical Path to Self-Managed Model:**
1. Worker polling loop - workers don't continuously process jobs yet
2. Scheduler implementation - scheduled jobs aren't being moved to ready queues
3. Client SDK Redis integration - can't actually submit jobs to the queue
4. Performance benchmarking - no data on throughput, latency, or bottlenecks
5. Production deployment guide - how to actually run this in production

**Documentation Gaps:**
- Internal documentation explaining architecture decisions and component interactions
- External documentation for library integration (examples, tutorials, best practices)
- Performance characteristics and tuning guides
- Troubleshooting guide

---

## PHASE 1: Make It Work End-to-End (Priority: CRITICAL) ✅ COMPLETE

### Task 1.1: Implement Worker Polling Loop ✅
**Goal**: Workers continuously poll Redis and process jobs

**Location**: `internal/worker/pool.go`

**Requirements:**
- Add `Start(ctx context.Context)` method to Pool
- Spawn N goroutines (based on concurrency config)
- Each goroutine continuously dequeues and processes jobs
- Graceful shutdown with 30-second timeout
- Panic recovery

**Success Criteria:** ✅ All achieved

### Task 1.2: Implement Scheduler Service ✅
**Goal**: Periodically move scheduled jobs to ready queues

**Location**: `cmd/scheduler/main.go`

**Requirements:**
- Initialize Redis queue connection
- Every 1 second, call `queue.MoveScheduledToReady()`
- Retry Redis connection on failure with exponential backoff

**Success Criteria:** ✅ All achieved

### Task 1.3: Integrate Client SDK with Redis ✅
**Goal**: Client can submit jobs to actual Redis queue

**Location**: `pkg/client/client.go`

**Requirements:**
- Refactor Client struct to use Redis
- Add Redis queue field
- Update all methods to interact with Redis

**Success Criteria:** ✅ All achieved

### Task 1.4: Create End-to-End Example ✅
**Goal**: Working example demonstrating the complete workflow

**Location**: `examples/complete_workflow/main.go`

**Success Criteria:** ✅ All achieved

---

## PHASE 2: Performance & Reliability (Priority: HIGH)

### Task 2.1: Performance Benchmarking
**Goal**: Establish performance baselines and identify bottlenecks

**Location**: `tests/benchmark_test.go`

**Requirements**: Create comprehensive benchmarks for:

1. **Job Submission Rate:**
   - Benchmark `client.SubmitJob()` throughput
   - Test with different payload sizes (1KB, 10KB, 100KB)
   - Measure latency percentiles (p50, p95, p99)

2. **Job Processing Rate:**
   - Benchmark end-to-end job processing (enqueue → execute → complete)
   - Test with different worker counts (1, 5, 10, 20)
   - Measure jobs/second and latency

3. **Queue Operations:**
   - Benchmark `Enqueue()`, `Dequeue()`, `Complete()`, `Fail()`
   - Test with varying queue depths (100, 1000, 10000 jobs)

4. **Concurrent Load:**
   - Simulate 100 concurrent clients submitting jobs
   - Measure throughput degradation
   - Identify contention points

5. **Output Format:**
   - Generate markdown report with tables and graphs
   - Include system specs (CPU, memory, Redis version)
   - Compare results across different configurations

**Documentation**: Add `docs/PERFORMANCE.md` with:
- Benchmark results
- Performance tuning recommendations
- Scaling guidelines (when to add more workers/Redis instances)
- Known bottlenecks and limitations

**Success Criteria:**
- Clear performance metrics documented
- Can process 10,000+ jobs/second with 20 workers
- p99 latency < 100ms for simple jobs
- Identify top 3 bottlenecks for optimization

---

### Task 2.2: Add Comprehensive Logging & Observability
**Goal**: Make system behavior visible and debuggable

**Location**: Multiple files

**Requirements:**

1. **Structured Logging** (`internal/logger/logger.go`):
   - Use `log/slog` for structured logging
   - Define log levels: DEBUG, INFO, WARN, ERROR
   - Include context fields: job_id, worker_id, queue_name, duration
   - Make log level configurable via `LOG_LEVEL` env var

2. **Worker Logging:**
   - Log when worker starts/stops
   - Log job dequeue (with queue wait time)
   - Log job execution start/end (with duration)
   - Log retry attempts (with attempt number and delay)
   - Log failures (with error details)

3. **Queue Logging:**
   - Log queue operations (enqueue, dequeue, complete, fail)
   - Log queue depth periodically (every 10 seconds)
   - Log scheduled set size

4. **Metrics Collection** (`internal/metrics/metrics.go`):
   - Track metrics in-memory (expose via API later):
     - Total jobs processed
     - Jobs by status (completed, failed, pending)
     - Average job duration
     - Queue depths
     - Worker utilization
   - Provide `GetMetrics()` function returning struct

5. **Health Checks:**
   - Add `/health` endpoint concept (document for future API)
   - Worker health: can connect to Redis, can dequeue jobs
   - Queue health: Redis connection alive, queue not stalled

**Tests:**
- Test structured logging produces correct format
- Test metrics are tracked accurately
- Test health checks detect failures

**Documentation**: Add `docs/OBSERVABILITY.md` with:
- Log format and fields explanation
- Available metrics and their meaning
- Troubleshooting guide based on logs/metrics
- Example log queries for common issues

**Success Criteria:**
- All significant events are logged with context
- Can diagnose issues from logs alone
- Metrics provide visibility into system health
- Documentation explains how to interpret logs/metrics

---

### Task 2.3: Error Handling & Recovery
**Goal**: System gracefully handles all failure scenarios

**Requirements:**

1. **Redis Connection Failures:**
   - Worker: retry connection with exponential backoff, continue processing when reconnected
   - Client: return clear error, don't panic
   - Scheduler: retry connection, log errors
   - All: max retry attempts before giving up

2. **Job Handler Panics:**
   - Worker: recover from panic, mark job as failed with panic stack trace
   - Don't crash worker goroutine
   - Log panic details

3. **Timeout Handling:**
   - Jobs exceeding timeout are cancelled via context
   - Mark as failed with timeout error
   - Log timeout with job details

4. **Invalid Job Payloads:**
   - Handler receives malformed JSON
   - Return clear error, don't retry (move to dead letter immediately)
   - Log payload for debugging

5. **Redis Data Corruption:**
   - Handle missing job data gracefully
   - Skip corrupted jobs, log warning
   - Continue processing valid jobs

**Tests**: Create `tests/failure_scenarios_test.go`:
- Test Redis disconnection during job processing
- Test handler panic recovery
- Test job timeout cancellation
- Test invalid payload handling
- Test Redis data loss scenarios

**Documentation**: Add to `docs/TROUBLESHOOTING.md`:
- Common failure scenarios and solutions
- How to handle dead letter queue jobs
- Recovery procedures
- When to scale vs when to fix code

**Success Criteria:**
- No panics crash the system
- Clear error messages for all failure modes
- System recovers automatically from transient failures
- Permanent failures are logged and moved to dead letter queue

---

## PHASE 3: Documentation Excellence (Priority: HIGH)

### Task 3.1: Internal Architecture Documentation
**Goal**: Developers can understand system internals quickly

**Location**: `docs/ARCHITECTURE.md`

### Task 3.2: External Integration Guide
**Goal**: Users can integrate Bananas into their projects quickly

**Location**: `docs/INTEGRATION.md` (enhance existing)

### Task 3.3: API Reference Documentation
**Goal**: Complete reference for all public APIs

**Location**: Enhance existing package READMEs + add `docs/API_REFERENCE.md`

---

## PHASE 4: Multi-Language SDKs (Priority: MEDIUM)

### Task 4.1: Python SDK
**Goal**: Python developers can use Bananas easily

**Location**: `sdks/python/bananas/`

### Task 4.2: TypeScript SDK
**Goal**: Node.js/TypeScript developers can use Bananas easily

**Location**: `sdks/typescript/`

---

## PHASE 5: Production Readiness (Priority: MEDIUM)

### Task 5.1: Production Deployment Guide
**Goal**: Clear path from development to production

**Location**: `docs/DEPLOYMENT.md`

### Task 5.2: Security Hardening
**Goal**: Production-ready security

---

## Success Metrics for Each Phase

### Phase 1 (Make It Work): ✅ COMPLETE
- ✅ Can submit 1000 jobs and all complete successfully
- ✅ Workers continuously process jobs without manual intervention
- ✅ Scheduled jobs execute at correct times
- ✅ Complete example runs without errors

### Phase 2 (Performance & Reliability):
- [ ] Can process 10,000+ jobs/second with 20 workers
- [ ] p99 latency < 100ms for simple jobs
- [ ] System recovers from Redis disconnection automatically
- [ ] All failure scenarios are handled gracefully

### Phase 3 (Documentation):
- [ ] New developer can understand architecture in 30 minutes
- [ ] User can integrate library in under 1 hour
- [ ] Every public API is documented with examples
- [ ] Troubleshooting guide resolves common issues

### Phase 4 (Multi-Language SDKs):
- [ ] Python SDK works identically to Go client
- [ ] TypeScript SDK works identically to Go client
- [ ] Both SDKs are pip/npm installable
- [ ] Full test coverage for both SDKs

### Phase 5 (Production Readiness):
- [ ] User can deploy to production in 1 hour
- [ ] Redis is secured with AUTH and TLS
- [ ] Monitoring setup is documented
- [ ] High availability setup is documented

---

## Testing Requirements for All Tasks

Every task must include:
1. Unit tests for new functions/methods (aim for 90%+ coverage)
2. Integration tests for cross-component interactions
3. Error case tests (what happens when things fail)
4. Documentation of what's being tested and why

Run tests frequently:
```bash
make test              # Run all tests
make test-coverage     # Generate coverage report
make test-verbose      # Run with detailed output
```

---

## Documentation Requirements for All Tasks

Every task must update relevant documentation:
1. Code comments for complex logic
2. Package-level documentation (what package does)
3. Function/method documentation (parameters, returns, errors)
4. README updates if public API changes
5. Architecture docs if design changes

Documentation should answer:
- What does this do?
- Why does it work this way?
- When should I use this?
- How do I handle errors?

---

## Implementation Order

**Priority for immediate work:**
1. ✅ Phase 1, Task 1.1 (Worker polling loop) - CRITICAL
2. ✅ Phase 1, Task 1.2 (Scheduler implementation) - CRITICAL
3. ✅ Phase 1, Task 1.3 (Client SDK Redis integration) - CRITICAL
4. ✅ Phase 1, Task 1.4 (End-to-end example) - HIGH
5. ✅ Phase 3, Task 3.2 (Integration guide) - HIGH

**After core works:**
6. Phase 2, Task 2.1 (Performance benchmarking) - HIGH
7. Phase 2, Task 2.2 (Logging & observability) - HIGH
8. Phase 2, Task 2.3 (Error handling) - HIGH
9. Phase 3, Task 3.1 (Architecture docs) - HIGH
10. Phase 3, Task 3.3 (API reference) - MEDIUM

**Multi-language support:**
11. Phase 4, Task 4.1 (Python SDK) - MEDIUM
12. Phase 4, Task 4.2 (TypeScript SDK) - MEDIUM

**Production hardening:**
13. Phase 5, Task 5.1 (Deployment guide) - MEDIUM
14. Phase 5, Task 5.2 (Security hardening) - MEDIUM

---

## Future Work (Not in Current Scope)

These are intentionally deferred:
- REST API for job submission (Phase 6)
- Web UI for job management (Phase 7)
- CLI tool for job management (Phase 7)
- Job result storage (Phase 8)
- Webhook notifications (Phase 8)
- Job dependencies/DAGs (Phase 9)
- Rate limiting per job type (Phase 9)

**Focus on making the library-based model excellent before building the SaaS layer.**

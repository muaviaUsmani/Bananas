# Internal Packages (`internal/`)

This directory contains all internal application logic that is not exposed as a public API. These packages implement the core functionality of the Bananas task queue system.

## Packages

### ⚙️ Config (`config/`)

**Purpose**: Centralized configuration management for all services.

**Key Components**:
- `Config` struct: Holds all application settings
- `LoadConfig()`: Loads configuration from environment variables with sensible defaults

**Configuration Fields**:
```go
type Config struct {
    RedisURL          string        // Redis connection URL
    APIPort           string        // API server port
    WorkerConcurrency int           // Number of concurrent workers
    JobTimeout        time.Duration // Max job execution time
    MaxRetries        int           // Max retry attempts per job
}
```

**Environment Variables**:
- `REDIS_URL` (default: `redis://localhost:6379`)
- `API_PORT` (default: `8080`)
- `WORKER_CONCURRENCY` (default: `5`)
- `JOB_TIMEOUT` (default: `5m`)
- `MAX_RETRIES` (default: `3`)

**Tests**: No dedicated tests (simple getters with defaults)

---

### 📦 Job (`job/`)

**Purpose**: Core job model and type definitions.

**Key Components**:

**Job Statuses**:
```go
const (
    StatusPending    JobStatus = "pending"    // Waiting in queue
    StatusProcessing JobStatus = "processing" // Currently executing
    StatusCompleted  JobStatus = "completed"  // Successfully finished
    StatusFailed     JobStatus = "failed"     // Failed permanently
    StatusScheduled  JobStatus = "scheduled"  // Waiting for retry time
)
```

**Job Priorities**:
```go
const (
    PriorityHigh   JobPriority = "high"   // Processed first
    PriorityNormal JobPriority = "normal" // Default priority
    PriorityLow    JobPriority = "low"    // Processed last
)
```

**Job Structure**:
```go
type Job struct {
    ID           string          // Unique UUID
    Name         string          // Job type (maps to handler)
    Description  string          // Optional description
    Payload      json.RawMessage // Job-specific data
    Status       JobStatus       // Current status
    Priority     JobPriority     // Processing priority
    CreatedAt    time.Time       // Creation timestamp
    UpdatedAt    time.Time       // Last update timestamp
    ScheduledFor *time.Time      // Retry execution time
    Attempts     int             // Number of attempts made
    MaxRetries   int             // Maximum retry attempts
    Error        string          // Error message if failed
}
```

**Methods**:
- `NewJob(name, payload, priority, description)`: Create a new job
- `UpdateStatus(status)`: Update job status and timestamp

#### Tests (`types_test.go`)

**9 tests covering**:

1. ✅ **TestNewJob_CreatesWithCorrectDefaults**
   - Verifies job is created with UUID, timestamps, and default values
   - Checks status is `pending`, attempts is `0`, max retries is `3`

2. ✅ **TestNewJob_GeneratesUniqueIDs**
   - Creates multiple jobs and verifies each has a unique ID
   - Ensures UUID collision doesn't occur

3. ✅ **TestNewJob_WithDescription**
   - Tests optional description parameter
   - Verifies description is properly set when provided

4. ✅ **TestNewJob_WithoutDescription**
   - Tests job creation without description
   - Verifies description field is empty string

5. ✅ **TestUpdateStatus_ChangesStatusAndTimestamp**
   - Tests `UpdateStatus()` method
   - Verifies both status and `UpdatedAt` are updated
   - Checks `UpdatedAt` is after `CreatedAt`

6. ✅ **TestJobPriority_Values**
   - Validates all priority constants
   - Ensures string values are correct

7. ✅ **TestJobStatus_Values**
   - Validates all status constants
   - Ensures string values are correct

8. ✅ **TestJob_JSONMarshaling**
   - Tests JSON serialization and deserialization
   - Verifies all fields are properly marshaled
   - Checks `omitempty` tags work correctly

9. ✅ **TestJob_TimestampsSet**
   - Verifies `CreatedAt` and `UpdatedAt` are non-zero
   - Checks timestamps are close to current time

**Run tests**:
```bash
make test
# or specifically
go test ./internal/job/
```

---

### 🗄️ Queue (`queue/`)

**Purpose**: Redis-backed queue operations with priority support, retry logic, and dead letter queue.

**Key Components**:

**RedisQueue**: Main queue implementation
```go
type RedisQueue struct {
    client    *redis.Client
    keyPrefix string // "bananas:"
}
```

**Redis Data Structures**:
1. **Job Hashes**: `bananas:job:{id}` - Store complete job data
2. **Priority Queues**: 
   - `bananas:queue:high` - High priority jobs
   - `bananas:queue:normal` - Normal priority jobs
   - `bananas:queue:low` - Low priority jobs
3. **Processing Queue**: `bananas:queue:processing` - Currently executing jobs
4. **Scheduled Set**: `bananas:queue:scheduled` - Jobs waiting for retry (sorted by timestamp)
5. **Dead Letter Queue**: `bananas:queue:dead` - Jobs that exceeded max retries

**Methods**:
- `NewRedisQueue(url)`: Connect to Redis
- `Enqueue(ctx, job)`: Add job to priority queue
- `Dequeue(ctx, priorities)`: Atomically move job from queue to processing
- `Complete(ctx, jobID)`: Mark job as completed and remove from processing
- `Fail(ctx, job, error)`: Handle failure with exponential backoff or DLQ
- `GetJob(ctx, jobID)`: Retrieve job data
- `MoveScheduledToReady(ctx)`: Move ready jobs from scheduled set to queues
- `Close()`: Close Redis connection

**Retry Strategy**:
- Uses exponential backoff: `delay = 2^attempts seconds`
- 1st retry: 2s, 2nd: 4s, 3rd: 8s, etc.
- Jobs stored in scheduled set instead of immediate re-queue
- Prevents thundering herd on failing dependencies

#### Tests (`redis_test.go`)

**18 tests covering**:

1. ✅ **TestNewRedisQueue_Success**
   - Tests successful Redis connection
   - Verifies queue initialization

2. ✅ **TestNewRedisQueue_InvalidURL**
   - Tests connection failure with invalid URL
   - Verifies error handling

3. ✅ **TestEnqueue_Success**
   - Tests job enqueue to priority queue
   - Verifies job data stored in Redis hash
   - Checks job added to correct priority queue

4. ✅ **TestDequeue_Success**
   - Tests atomic dequeue from priority queue
   - Verifies job moved to processing queue
   - Checks RPOPLPUSH atomicity

5. ✅ **TestDequeue_EmptyQueue**
   - Tests dequeue when no jobs available
   - Verifies returns nil without error

6. ✅ **TestDequeue_PriorityOrder**
   - Enqueues jobs to all three priority queues
   - Verifies high priority dequeued first
   - Checks normal, then low priority order

7. ✅ **TestComplete_Success**
   - Tests marking job as completed
   - Verifies job status updated in Redis
   - Checks job removed from processing queue

8. ✅ **TestFail_WithRetry**
   - Tests failure with retries remaining
   - Verifies job moved to scheduled set (NOT priority queue)
   - Checks exponential backoff delay calculation
   - Validates `ScheduledFor` timestamp set
   - Ensures attempts incremented

9. ✅ **TestFail_MaxRetriesExceeded**
   - Tests failure after max retries
   - Verifies job moved to dead letter queue
   - Checks status set to `failed`
   - Ensures removed from processing queue

10. ✅ **TestGetJob_Success**
    - Tests job retrieval by ID
    - Verifies all fields deserialized correctly

11. ✅ **TestGetJob_NotFound**
    - Tests retrieving non-existent job
    - Verifies appropriate error returned

12. ✅ **TestClose**
    - Tests Redis connection close
    - Verifies cleanup

13. ✅ **TestMoveScheduledToReady_Success**
    - Enqueues job, dequeues, fails it (goes to scheduled set)
    - Manually sets score to past time
    - Calls `MoveScheduledToReady()`
    - Verifies job moved back to priority queue
    - Checks `ScheduledFor` cleared
    - Ensures removed from scheduled set

14. ✅ **TestMoveScheduledToReady_NoJobs**
    - Tests with empty scheduled set
    - Verifies returns 0 count without error

15. ✅ **TestMoveScheduledToReady_FutureJobs**
    - Adds job to scheduled set with future timestamp
    - Calls `MoveScheduledToReady()`
    - Verifies job NOT moved (stays in scheduled set)
    - Checks count is 0

16. ✅ **TestExponentialBackoff_Calculation**
    - Creates job and fails it 4 times
    - Verifies delays: 2s, 4s, 8s, 16s
    - Checks exponential formula: `2^attempts`
    - Validates `ScheduledFor` timestamps increase exponentially

17. ✅ **TestMoveScheduledToReady_MultipleJobs**
    - Enqueues 3 jobs (high, normal, low priority)
    - Fails all (go to scheduled set)
    - Sets all scores to past time
    - Moves all to ready queues
    - Verifies each returns to its original priority queue

18. ✅ **TestDequeue_MultipleWorkers**
    - Simulates 3 concurrent workers
    - Enqueues 3 jobs
    - Dequeues concurrently
    - Verifies each worker gets a different job (no duplication)
    - Checks all jobs moved to processing queue

**Run tests**:
```bash
make test
# or specifically
go test ./internal/queue/
```

---

### 👷 Worker (`worker/`)

**Purpose**: Job execution engine with handler registry, worker pool, and concurrency management.

**Key Components**:

**1. Handler Registry** (`handler.go`):
```go
type HandlerFunc func(ctx context.Context, j *job.Job) error
type Registry struct {
    handlers map[string]HandlerFunc
}
```
- Maps job names to handler functions
- Simple registration: `registry.Register("job_name", handlerFunc)`

**2. Executor** (`executor.go`):
```go
type Executor struct {
    registry    *Registry
    queue       Queue
    concurrency int
}
```
- Executes individual jobs
- Looks up handler by job name
- Updates job status (Pending → Processing → Completed/Failed)
- Calls `queue.Complete()` or `queue.Fail()` based on result
- Handles context cancellation and timeouts

**3. Worker Pool** (`pool.go`):
```go
type Pool struct {
    executor    *Executor
    queue       QueueOperations
    concurrency int
    jobTimeout  time.Duration
    wg          sync.WaitGroup
    stopChan    chan struct{}
}
```
- Manages multiple worker goroutines
- Configurable concurrency
- Per-job timeout enforcement
- Graceful shutdown support
- Continuous dequeue loop

**Example Handlers** (`example_handlers.go`):
- `HandleCountItems`: Count items in JSON array
- `HandleSendEmail`: Simulate sending email
- `HandleProcessData`: Simulate data processing

#### Tests

**Executor Tests** (`executor_test.go` - 6 tests):

1. ✅ **TestNewExecutor**
   - Tests executor creation
   - Verifies registry and queue set correctly

2. ✅ **TestExecuteJob_ValidHandler**
   - Registers a handler and executes a job
   - Verifies handler is called
   - Checks `queue.Complete()` called on success

3. ✅ **TestExecuteJob_UnknownHandler**
   - Tests execution with unregistered job name
   - Verifies error returned
   - Checks `queue.Fail()` called

4. ✅ **TestExecuteJob_StatusUpdates**
   - Tests job status transitions
   - Verifies: Pending → Processing → Completed
   - Checks `UpdatedAt` timestamp changes

5. ✅ **TestExecuteJob_HandlerError**
   - Handler returns an error
   - Verifies error propagated
   - Checks `queue.Fail()` called with error message

6. ✅ **TestExecuteJob_ContextCancellation**
   - Cancels context during execution
   - Verifies job execution stops
   - Checks `queue.Fail()` called with cancellation error

**Handler Tests** (`handler_test.go` - 9 tests):

7. ✅ **TestRegistry_Register**
   - Tests registering a handler
   - Verifies handler stored in registry

8. ✅ **TestRegistry_Get_RegisteredHandler**
   - Registers and retrieves handler
   - Verifies correct handler returned

9. ✅ **TestRegistry_Get_UnregisteredHandler**
   - Tests getting non-existent handler
   - Verifies returns `false`

10. ✅ **TestHandleCountItems_ExecutesWithoutError**
    - Tests example `HandleCountItems` handler
    - Provides valid JSON array payload
    - Verifies executes successfully

11. ✅ **TestHandleCountItems_InvalidPayload**
    - Tests with invalid JSON payload
    - Verifies returns error

12. ✅ **TestHandleSendEmail_ExecutesWithoutError**
    - Tests example `HandleSendEmail` handler
    - Provides valid email payload
    - Verifies executes successfully (simulates 2s delay)

13. ✅ **TestHandleSendEmail_InvalidPayload**
    - Tests with invalid payload
    - Verifies returns error

14. ✅ **TestHandleProcessData_ExecutesWithoutError**
    - Tests example `HandleProcessData` handler
    - Verifies executes successfully (simulates 3s delay)

15. ✅ **TestRegistry_MultipleHandlers**
    - Registers multiple handlers
    - Verifies all can be retrieved
    - Checks `Count()` returns correct number

**Pool Tests** (`pool_test.go` - 5 tests):

16. ✅ **TestNewPool**
    - Tests pool creation
    - Verifies concurrency and timeout set correctly

17. ✅ **TestPool_StartStop**
    - Starts pool with workers
    - Tests graceful shutdown
    - Verifies `Stop()` blocks until workers finish

18. ✅ **TestPool_ProcessesJobs**
    - Enqueues 2 jobs
    - Verifies both are processed by worker pool
    - Checks completion callbacks called

19. ✅ **TestPool_ConcurrencyLimit**
    - Sets concurrency to 2
    - Enqueues 5 jobs (more than concurrency)
    - Verifies max 2 workers execute simultaneously
    - Checks no more than 2 jobs run concurrently

20. ✅ **TestPool_RespectsJobTimeout**
    - Sets short job timeout (100ms)
    - Enqueues job that takes 500ms
    - Verifies job is cancelled due to timeout
    - Checks `queue.Fail()` called

**Run tests**:
```bash
make test
# or specifically
go test ./internal/worker/
```

---

## Package Dependencies

```
internal/
├── config/           (no dependencies)
├── job/              (depends on: uuid)
├── queue/            (depends on: job, redis)
└── worker/           (depends on: job, queue interfaces)
```

## Testing

Run all internal package tests:
```bash
# All internal tests
make test

# With verbose output
make test-verbose

# With coverage
make test-coverage
```

**Total Internal Tests: 42 tests**
- Config: 0 tests (simple getters)
- Job: 9 tests
- Queue: 18 tests
- Worker: 20 tests (6 executor + 9 handler + 5 pool)


# API Reference

> **Last Updated:** 2025-11-10
> **Go Version:** 1.21+

Complete API reference for Bananas distributed task queue system.

## Table of Contents

- [Client API](#client-api)
- [Job Types](#job-types)
- [Worker API](#worker-api)
- [Configuration API](#configuration-api)
- [Queue API (Internal)](#queue-api-internal)
- [Result Backend API](#result-backend-api)
- [Scheduler API](#scheduler-api)
- [Error Types](#error-types)

---

## Client API

Package: `github.com/muaviaUsmani/bananas/pkg/client`

### Client

The main client for interacting with the Bananas task queue.

#### NewClient

```go
func NewClient(redisURL string) (*Client, error)
```

Creates a new client connected to Redis with default result backend TTLs.

**Parameters:**
- `redisURL` (string): Redis connection URL (e.g., `redis://localhost:6379/0`)

**Returns:**
- `*Client`: Configured client instance
- `error`: Connection error if Redis unreachable

**Example:**
```go
client, err := client.NewClient("redis://localhost:6379")
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

#### NewClientWithConfig

```go
func NewClientWithConfig(redisURL string, successTTL, failureTTL time.Duration) (*Client, error)
```

Creates a client with custom result backend TTL settings.

**Parameters:**
- `redisURL` (string): Redis connection URL
- `successTTL` (time.Duration): TTL for successful job results
- `failureTTL` (time.Duration): TTL for failed job results

**Example:**
```go
client, err := client.NewClientWithConfig(
    "redis://localhost:6379",
    2*time.Hour,    // Success results expire in 2 hours
    48*time.Hour,   // Failed results kept for 2 days
)
```

#### SubmitJob

```go
func (c *Client) SubmitJob(
    name string,
    payload interface{},
    priority job.JobPriority,
    description ...string,
) (string, error)
```

Submits a job to the default routing queue.

**Parameters:**
- `name` (string): Job handler name (must be registered on workers)
- `payload` (interface{}): Job payload (will be JSON marshaled)
- `priority` (job.JobPriority): Job priority (`PriorityHigh`, `PriorityNormal`, `PriorityLow`)
- `description` (...string): Optional job description (first value used if provided)

**Returns:**
- `string`: Job ID (UUID)
- `error`: Submission error

**Example:**
```go
type EmailPayload struct {
    To      string `json:"to"`
    Subject string `json:"subject"`
    Body    string `json:"body"`
}

payload := EmailPayload{
    To:      "user@example.com",
    Subject: "Welcome!",
    Body:    "Thanks for signing up",
}

jobID, err := client.SubmitJob(
    "send_email",
    payload,
    job.PriorityNormal,
    "Welcome email to new user",
)
```

#### SubmitJobWithRoute

```go
func (c *Client) SubmitJobWithRoute(
    name string,
    payload interface{},
    priority job.JobPriority,
    routingKey string,
    description ...string,
) (string, error)
```

Submits a job to a specific routing queue.

**Parameters:**
- `name` (string): Job handler name
- `payload` (interface{}): Job payload
- `priority` (job.JobPriority): Job priority
- `routingKey` (string): Routing key for worker selection
- `description` (...string): Optional job description

**Routing Key Rules:**
- Alphanumeric, underscore, hyphen only
- 1-64 characters
- Examples: `gpu`, `email`, `high_memory`, `us-east-1`

**Example:**
```go
// Route GPU job to GPU workers
jobID, err := client.SubmitJobWithRoute(
    "process_image",
    map[string]interface{}{
        "url":    "https://example.com/image.jpg",
        "width":  1920,
        "height": 1080,
    },
    job.PriorityHigh,
    "gpu",
    "Resize user avatar",
)

// Route email job to email workers
jobID, err = client.SubmitJobWithRoute(
    "send_email",
    emailPayload,
    job.PriorityNormal,
    "email",
)
```

#### SubmitJobScheduled

```go
func (c *Client) SubmitJobScheduled(
    name string,
    payload interface{},
    priority job.JobPriority,
    scheduledFor time.Time,
    description ...string,
) (string, error)
```

Submits a job for future execution.

**Parameters:**
- `name` (string): Job handler name
- `payload` (interface{}): Job payload
- `priority` (job.JobPriority): Job priority
- `scheduledFor` (time.Time): Execution time
- `description` (...string): Optional description

**Example:**
```go
// Schedule job for 1 hour from now
scheduledTime := time.Now().Add(1 * time.Hour)

jobID, err := client.SubmitJobScheduled(
    "send_reminder",
    map[string]string{"user_id": "123"},
    job.PriorityNormal,
    scheduledTime,
    "Appointment reminder",
)
```

#### GetJob

```go
func (c *Client) GetJob(jobID string) (*job.Job, error)
```

Retrieves job details by ID.

**Parameters:**
- `jobID` (string): Job ID returned from submission

**Returns:**
- `*job.Job`: Job details (nil if not found)
- `error`: Retrieval error

**Example:**
```go
job, err := client.GetJob(jobID)
if err != nil {
    log.Printf("Failed to get job: %v", err)
    return
}

fmt.Printf("Job status: %s\n", job.Status)
fmt.Printf("Attempts: %d/%d\n", job.Attempts, job.MaxRetries)
```

#### GetResult

```go
func (c *Client) GetResult(ctx context.Context, jobID string) (*job.JobResult, error)
```

Retrieves job result (non-blocking).

**Parameters:**
- `ctx` (context.Context): Request context
- `jobID` (string): Job ID

**Returns:**
- `*job.JobResult`: Result (nil if not ready yet)
- `error`: Retrieval error

**Example:**
```go
// Poll for result
result, err := client.GetResult(ctx, jobID)
if err != nil {
    log.Printf("Failed to get result: %v", err)
    return
}

if result == nil {
    fmt.Println("Job not complete yet")
    return
}

if result.IsSuccess() {
    var data map[string]interface{}
    result.UnmarshalResult(&data)
    fmt.Printf("Result: %+v\n", data)
} else {
    fmt.Printf("Job failed: %s\n", result.Error)
}
```

#### SubmitAndWait

```go
func (c *Client) SubmitAndWait(
    ctx context.Context,
    name string,
    payload interface{},
    priority job.JobPriority,
    timeout time.Duration,
) (*job.JobResult, error)
```

Submits a job and waits for the result (RPC-style).

**Parameters:**
- `ctx` (context.Context): Request context
- `name` (string): Job handler name
- `payload` (interface{}): Job payload
- `priority` (job.JobPriority): Job priority
- `timeout` (time.Duration): Maximum wait time

**Returns:**
- `*job.JobResult`: Job result
- `error`: Timeout or execution error

**Example:**
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := client.SubmitAndWait(
    ctx,
    "process_payment",
    map[string]interface{}{
        "amount": 99.99,
        "card":   "****1234",
    },
    job.PriorityHigh,
    30*time.Second,
)

if err != nil {
    log.Printf("Payment processing failed: %v", err)
    return
}

if result.IsSuccess() {
    fmt.Printf("Payment processed in %v\n", result.Duration)
} else {
    fmt.Printf("Payment failed: %s\n", result.Error)
}
```

#### Close

```go
func (c *Client) Close() error
```

Closes Redis connections.

**Example:**
```go
defer client.Close()
```

---

## Job Types

Package: `github.com/muaviaUsmani/bananas/internal/job`

### Job

```go
type Job struct {
    ID           string
    Name         string
    Description  string
    Payload      json.RawMessage
    Status       JobStatus
    Priority     JobPriority
    RoutingKey   string
    CreatedAt    time.Time
    UpdatedAt    time.Time
    ScheduledFor *time.Time
    Attempts     int
    MaxRetries   int
    Error        string
}
```

#### JobStatus

```go
type JobStatus string

const (
    StatusPending    JobStatus = "pending"
    StatusProcessing JobStatus = "processing"
    StatusCompleted  JobStatus = "completed"
    StatusFailed     JobStatus = "failed"
    StatusScheduled  JobStatus = "scheduled"
)
```

#### JobPriority

```go
type JobPriority string

const (
    PriorityHigh   JobPriority = "high"
    PriorityNormal JobPriority = "normal"
    PriorityLow    JobPriority = "low"
)
```

#### Methods

##### NewJob

```go
func NewJob(
    name string,
    payload []byte,
    priority JobPriority,
    description ...string,
) *Job
```

Creates a new job with default settings.

**Defaults:**
- Routing key: `"default"`
- Max retries: `3`
- Status: `StatusPending`

**Example:**
```go
payload, _ := json.Marshal(map[string]string{"key": "value"})
job := job.NewJob("my_task", payload, job.PriorityNormal, "My task description")
```

##### SetRoutingKey

```go
func (j *Job) SetRoutingKey(routingKey string) error
```

Sets the routing key with validation.

**Example:**
```go
if err := job.SetRoutingKey("gpu"); err != nil {
    log.Fatal(err)
}
```

##### ValidateRoutingKey

```go
func ValidateRoutingKey(key string) error
```

Validates routing key format.

**Rules:**
- Alphanumeric + underscore + hyphen only
- 1-64 characters
- Non-empty

**Example:**
```go
if err := job.ValidateRoutingKey("my-routing-key"); err != nil {
    log.Printf("Invalid routing key: %v", err)
}
```

##### UnmarshalPayload

```go
func (j *Job) UnmarshalPayload(v interface{}) error
```

Unmarshals job payload into a struct.

**Example:**
```go
type MyPayload struct {
    UserID string `json:"user_id"`
    Action string `json:"action"`
}

var payload MyPayload
if err := job.UnmarshalPayload(&payload); err != nil {
    return err
}

fmt.Printf("Processing action %s for user %s\n", payload.Action, payload.UserID)
```

### JobResult

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

#### Methods

##### IsSuccess

```go
func (r *JobResult) IsSuccess() bool
```

Returns true if job completed successfully.

**Example:**
```go
if result.IsSuccess() {
    fmt.Println("Job succeeded!")
}
```

##### IsFailed

```go
func (r *JobResult) IsFailed() bool
```

Returns true if job failed.

**Example:**
```go
if result.IsFailed() {
    fmt.Printf("Job failed: %s\n", result.Error)
}
```

##### UnmarshalResult

```go
func (r *JobResult) UnmarshalResult(v interface{}) error
```

Unmarshals result data into a struct.

**Example:**
```go
type ProcessResult struct {
    Count    int      `json:"count"`
    Items    []string `json:"items"`
    Duration float64  `json:"duration"`
}

var result ProcessResult
if err := jobResult.UnmarshalResult(&result); err != nil {
    return err
}

fmt.Printf("Processed %d items in %.2fs\n", result.Count, result.Duration)
```

---

## Worker API

Package: `github.com/muaviaUsmani/bananas/internal/worker`

### Registry

Handler registry for job execution.

#### NewRegistry

```go
func NewRegistry() *Registry
```

Creates a new handler registry.

**Example:**
```go
registry := worker.NewRegistry()
```

#### Register

```go
func (r *Registry) Register(name string, handler HandlerFunc)
```

Registers a job handler.

**Handler Signature:**
```go
type HandlerFunc func(context.Context, *job.Job) error
```

**Example:**
```go
registry.Register("send_email", func(ctx context.Context, j *job.Job) error {
    type EmailPayload struct {
        To      string `json:"to"`
        Subject string `json:"subject"`
        Body    string `json:"body"`
    }

    var payload EmailPayload
    if err := j.UnmarshalPayload(&payload); err != nil {
        return err
    }

    // Send email (simplified)
    fmt.Printf("Sending email to %s: %s\n", payload.To, payload.Subject)

    // Respect context cancellation
    select {
    case <-ctx.Done():
        return ctx.Err()
    case <-time.After(1 * time.Second):
        return nil
    }
})
```

#### Get

```go
func (r *Registry) Get(name string) (HandlerFunc, bool)
```

Retrieves a handler by name.

**Example:**
```go
handler, exists := registry.Get("send_email")
if !exists {
    log.Fatal("Handler not found")
}
```

#### Count

```go
func (r *Registry) Count() int
```

Returns the number of registered handlers.

### Executor

Job execution engine.

#### NewExecutor

```go
func NewExecutor(registry *Registry, queue Queue, concurrency int) *Executor
```

Creates a new executor.

**Parameters:**
- `registry` (*Registry): Handler registry
- `queue` (Queue): Queue interface for job updates
- `concurrency` (int): Max concurrent jobs

**Example:**
```go
executor := worker.NewExecutor(registry, queue, 10)
```

#### SetResultBackend

```go
func (e *Executor) SetResultBackend(backend result.Backend)
```

Configures result storage (optional).

**Example:**
```go
resultBackend := result.NewRedisBackend(redisClient, 1*time.Hour, 24*time.Hour)
executor.SetResultBackend(resultBackend)
```

#### ExecuteJob

```go
func (e *Executor) ExecuteJob(ctx context.Context, j *job.Job) error
```

Executes a single job.

**Example:**
```go
if err := executor.ExecuteJob(ctx, job); err != nil {
    log.Printf("Job execution failed: %v", err)
}
```

### Pool

Worker pool for concurrent job processing.

#### NewPoolWithConfig

```go
func NewPoolWithConfig(
    executor *Executor,
    queue QueueReader,
    workerConfig *config.WorkerConfig,
    jobTimeout time.Duration,
) *Pool
```

Creates a worker pool with configuration.

**Example:**
```go
workerConfig := &config.WorkerConfig{
    Mode:        config.WorkerModeDefault,
    Concurrency: 10,
    Priorities:  []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow},
    RoutingKeys: []string{"gpu", "default"},
}

pool := worker.NewPoolWithConfig(executor, queue, workerConfig, 5*time.Minute)
```

#### Start

```go
func (p *Pool) Start(ctx context.Context)
```

Starts worker goroutines.

**Example:**
```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

pool.Start(ctx)
```

#### Stop

```go
func (p *Pool) Stop()
```

Gracefully stops workers (waits for in-flight jobs to complete).

**Example:**
```go
// Graceful shutdown
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
<-sigChan

pool.Stop() // Waits up to 30s for jobs to complete
```

---

## Configuration API

Package: `github.com/muaviaUsmani/bananas/internal/config`

### WorkerConfig

```go
type WorkerConfig struct {
    Mode              WorkerMode
    Concurrency       int
    Priorities        []job.JobPriority
    RoutingKeys       []string
    JobTypes          []string
    SchedulerInterval time.Duration
    EnableScheduler   bool
}
```

#### WorkerMode

```go
type WorkerMode string

const (
    WorkerModeThin           WorkerMode = "thin"
    WorkerModeDefault        WorkerMode = "default"
    WorkerModeSpecialized    WorkerMode = "specialized"
    WorkerModeJobSpecialized WorkerMode = "job-specialized"
    WorkerModeSchedulerOnly  WorkerMode = "scheduler-only"
)
```

#### LoadWorkerConfig

```go
func LoadWorkerConfig() (*WorkerConfig, error)
```

Loads configuration from environment variables.

**Environment Variables:**
```bash
WORKER_MODE=default
WORKER_CONCURRENCY=10
WORKER_PRIORITIES=high,normal,low
WORKER_ROUTING_KEYS=gpu,default
WORKER_JOB_TYPES=send_email,send_sms
SCHEDULER_INTERVAL=1s
ENABLE_SCHEDULER=true
```

**Example:**
```go
config, err := config.LoadWorkerConfig()
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Worker mode: %s\n", config.Mode)
fmt.Printf("Concurrency: %d\n", config.Concurrency)
```

---

## Queue API (Internal)

Package: `github.com/muaviaUsmani/bananas/internal/queue`

**Note:** Queue API is internal and primarily used by workers. Client API is recommended for external use.

### RedisQueue

#### NewRedisQueue

```go
func NewRedisQueue(redisURL string) (*RedisQueue, error)
```

Creates a new Redis queue connection.

#### Enqueue

```go
func (q *RedisQueue) Enqueue(ctx context.Context, j *job.Job) error
```

Enqueues a job to the appropriate routing and priority queue.

#### DequeueWithRouting

```go
func (q *RedisQueue) DequeueWithRouting(ctx context.Context, routingKeys []string) (*job.Job, error)
```

Dequeues a job from specified routing keys with priority ordering.

#### Complete

```go
func (q *RedisQueue) Complete(ctx context.Context, jobID string) error
```

Marks a job as completed.

#### Fail

```go
func (q *RedisQueue) Fail(ctx context.Context, j *job.Job, errMsg string) error
```

Handles job failure with retry scheduling or dead letter queue.

#### MoveScheduledToReady

```go
func (q *RedisQueue) MoveScheduledToReady(ctx context.Context) (int, error)
```

Moves scheduled jobs to ready queues (called by scheduler).

---

## Result Backend API

Package: `github.com/muaviaUsmani/bananas/internal/result`

### Backend Interface

```go
type Backend interface {
    StoreResult(ctx context.Context, result *job.JobResult) error
    GetResult(ctx context.Context, jobID string) (*job.JobResult, error)
    WaitForResult(ctx context.Context, jobID string, timeout time.Duration) (*job.JobResult, error)
    DeleteResult(ctx context.Context, jobID string) error
    Close() error
}
```

### RedisBackend

#### NewRedisBackend

```go
func NewRedisBackend(
    client *redis.Client,
    successTTL time.Duration,
    failureTTL time.Duration,
) *RedisBackend
```

Creates a Redis-based result backend.

**Example:**
```go
opts, _ := redis.ParseURL("redis://localhost:6379")
redisClient := redis.NewClient(opts)

backend := result.NewRedisBackend(
    redisClient,
    1*time.Hour,    // Success TTL
    24*time.Hour,   // Failure TTL
)
```

---

## Scheduler API

Package: `github.com/muaviaUsmani/bananas/internal/scheduler`

### CronScheduler

#### NewCronScheduler

```go
func NewCronScheduler(
    queue queue.Queue,
    schedules []*Schedule,
    location *time.Location,
) (*CronScheduler, error)
```

Creates a cron scheduler.

**Example:**
```go
schedules := []*scheduler.Schedule{
    {
        ID:       "daily-report",
        Cron:     "0 9 * * *",     // Daily at 9 AM
        Job:      "generate_report",
        Payload:  []byte(`{"type":"daily"}`),
        Priority: job.PriorityNormal,
        Timezone: "America/New_York",
        Enabled:  true,
    },
}

location, _ := time.LoadLocation("America/New_York")
cronScheduler, err := scheduler.NewCronScheduler(queue, schedules, location)
```

#### Start

```go
func (s *CronScheduler) Start(ctx context.Context)
```

Starts the cron scheduler.

#### Stop

```go
func (s *CronScheduler) Stop()
```

Stops the cron scheduler.

### Schedule

```go
type Schedule struct {
    ID          string
    Cron        string
    Job         string
    Payload     []byte
    Priority    job.JobPriority
    Timezone    string
    Enabled     bool
    Description string
}
```

**Cron Format:**
```
┌─────── Minute (0-59)
│ ┌───── Hour (0-23)
│ │ ┌─── Day of Month (1-31)
│ │ │ ┌─ Month (1-12)
│ │ │ │ ┌ Day of Week (0-6, Sunday = 0)
│ │ │ │ │
* * * * *
```

**Examples:**
- `0 * * * *` - Every hour at minute 0
- `*/15 * * * *` - Every 15 minutes
- `0 9 * * 1-5` - Weekdays at 9 AM
- `0 0 1 * *` - First day of month at midnight

---

## Error Types

### Common Errors

```go
// Job not found
job.ErrNotFound

// Invalid routing key
job.ErrInvalidRoutingKey

// Handler not registered
worker.ErrHandlerNotFound

// Result not ready
result.ErrNotReady

// Timeout waiting for result
context.DeadlineExceeded
```

### Error Handling Example

```go
job, err := client.GetJob(jobID)
if err != nil {
    if errors.Is(err, job.ErrNotFound) {
        fmt.Println("Job not found")
    } else {
        fmt.Printf("Error: %v\n", err)
    }
    return
}
```

---

## Related Documentation

- [Architecture Overview](./ARCHITECTURE.md) - System design and components
- [Integration Guide](../INTEGRATION_GUIDE.md) - Getting started guide
- [Task Routing Guide](./TASK_ROUTING_USAGE.md) - Routing configuration
- [Performance Guide](./PERFORMANCE.md) - Benchmarks and tuning
- [Deployment Guide](./DEPLOYMENT.md) - Production deployment

---

**Next:** [Integration Guide](../INTEGRATION_GUIDE.md) | [Examples](../examples/)

# Bananas Integration Guide

**Last Updated:** 2025-11-11
**Status:** Complete

A comprehensive guide for integrating the Bananas distributed task queue into your existing infrastructure and projects.

## Table of Contents

1. [Overview](#overview)
2. [Core Concepts](#core-concepts)
3. [Integration Patterns](#integration-patterns)
4. [Quick Start](#quick-start)
5. [Client SDK Guide](#client-sdk-guide)
6. [Creating Job Handlers](#creating-job-handlers)
7. [Task Routing](#task-routing)
8. [Result Backend](#result-backend)
9. [Periodic Tasks](#periodic-tasks)
10. [Deployment Strategies](#deployment-strategies)
11. [Configuration](#configuration)
12. [Best Practices](#best-practices)
13. [Monitoring & Observability](#monitoring--observability)
14. [Troubleshooting](#troubleshooting)
15. [Migration Guide](#migration-guide)

---

## Overview

Bananas is a high-performance distributed task queue system built in Go that provides enterprise-grade features for reliable background job processing.

### Key Features

- **Priority-based Processing**: High, normal, and low priority queues
- **Task Routing**: Route jobs to specialized worker pools (GPU, email, regions)
- **Result Backend**: Synchronous job execution with `SubmitAndWait()`
- **Periodic Tasks**: Cron-based scheduled jobs
- **Exponential Backoff**: Automatic retry with configurable strategies
- **Dead Letter Queue**: Failed job isolation and analysis
- **Concurrent Worker Pools**: Goroutine-based lightweight workers
- **Redis-backed Persistence**: Reliable job storage and distribution
- **Production Ready**: Prometheus metrics, structured logging, health checks

### Use Cases

| Use Case | Priority | Routing Key | Pattern |
|----------|----------|-------------|---------|
| OTP/Password reset emails | High | `email` | Fire-and-forget |
| Image processing | Normal | `gpu` | Fire-and-forget |
| Video transcoding | Normal | `gpu` | Result backend |
| Report generation | Low | `default` | Result backend |
| Data imports/exports | Normal | `high_memory` | Fire-and-forget |
| Webhook delivery | High | `default` | Retry with backoff |
| Nightly cleanup | Low | `default` | Periodic task |
| Regional processing | Normal | `us-east-1` | Task routing |

---

## Core Concepts

### Jobs

A job is a unit of work with:
- **Name**: Handler identifier (`"send_email"`, `"process_image"`)
- **Payload**: JSON-serializable data
- **Priority**: High (1), Normal (2), Low (3)
- **Routing Key**: Worker pool selector (default: `"default"`)
- **Status**: Pending, Processing, Completed, Failed, Scheduled
- **Retry Logic**: Max attempts, exponential backoff

### Queues

Jobs are stored in Redis priority queues:

```
bananas:route:{routing_key}:queue:{priority}
```

Examples:
- `bananas:route:default:queue:high` - Default high-priority jobs
- `bananas:route:gpu:queue:normal` - GPU worker normal-priority jobs
- `bananas:route:email:queue:high` - Email worker high-priority jobs

### Workers

Workers are lightweight goroutine-based processors that:
- Pull jobs from queues using blocking Redis operations (BRPOPLPUSH)
- Execute registered handlers with context cancellation
- Update job status and handle retries
- Support multiple routing keys with priority ordering

### Scheduler

The scheduler manages:
- **Retry jobs**: Exponential backoff (2^attempts seconds)
- **Periodic tasks**: Cron-based scheduled execution
- **Scheduled jobs**: User-defined execution times
- **Distributed locking**: Single scheduler across multiple instances

---

## Integration Patterns

Choose the pattern that best fits your architecture:

### Pattern 1: Microservices (Recommended)

**Best for**: Production deployments, independent scaling, distributed systems

```
┌─────────────┐         ┌─────────────┐
│   Your App  │         │   Workers   │
│  (Client)   │────────▶│   (GPU)     │
└─────────────┘         └─────────────┘
      │                 ┌─────────────┐
      │                 │   Workers   │
      └────────────────▶│   (Email)   │
                        └─────────────┘
         ▲                     ▲
         │                     │
         └──────Redis──────────┘
```

**Pros**:
- Independent scaling (add more GPU workers as needed)
- Fault isolation (worker crashes don't affect your app)
- Zero-downtime deployments (update workers separately)
- Better resource management (GPU workers on GPU instances)

**Cons**:
- More infrastructure to manage
- Network latency for job submission
- Requires service discovery/configuration

---

### Pattern 2: Embedded Workers

**Best for**: Monolithic applications, simple deployments, getting started

```
┌────────────────────────────┐
│       Your Application     │
│  ┌──────────┐  ┌─────────┐│
│  │  Client  │  │ Workers ││
│  └──────────┘  └─────────┘│
└────────────────────────────┘
              │
              ▼
          ┌───────┐
          │ Redis │
          └───────┘
```

**Pros**:
- Simplest deployment (single binary)
- No network latency
- Shared memory and resources

**Cons**:
- Tightly coupled (restart required for updates)
- Can't scale workers independently
- Worker failures affect your app

---

### Pattern 3: Hybrid

**Best for**: Gradual migration, flexible deployments

```
┌─────────────┐
│   Your App  │
│  (Client +  │         ┌─────────────┐
│   Workers)  │────────▶│   Workers   │
└─────────────┘         │   (GPU)     │
      │                 └─────────────┘
      │                 ┌─────────────┐
      └────────────────▶│ Scheduler   │
                        └─────────────┘
         ▲                     ▲
         └──────Redis──────────┘
```

**Pros**:
- Easy migration path (start embedded, scale out later)
- Flexibility (critical jobs locally, batch jobs remotely)
- Best of both worlds

---

## Quick Start

### Step 1: Install Dependencies

```bash
# Add Bananas to your Go project
go get github.com/muaviaUsmani/bananas@latest

# Start Redis
docker run -d -p 6379:6379 redis:7-alpine
```

### Step 2: Submit Your First Job

```go
package main

import (
    "log"

    "github.com/muaviaUsmani/bananas/pkg/client"
    "github.com/muaviaUsmani/bananas/internal/job"
)

func main() {
    // Create client
    c, err := client.NewClient("redis://localhost:6379")
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()

    // Submit a job
    jobID, err := c.SubmitJob(
        "send_welcome_email",           // job name
        map[string]string{              // payload
            "email": "user@example.com",
            "name":  "John Doe",
        },
        job.PriorityHigh,               // priority
        "Welcome email for new user",   // description
    )

    if err != nil {
        log.Fatalf("Failed to submit job: %v", err)
    }

    log.Printf("Job submitted: %s", jobID)
}
```

### Step 3: Create a Worker

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/muaviaUsmani/bananas/internal/config"
    "github.com/muaviaUsmani/bananas/internal/job"
    "github.com/muaviaUsmani/bananas/internal/queue"
    "github.com/muaviaUsmani/bananas/internal/worker"
)

func main() {
    // Load configuration
    cfg, err := config.LoadWorkerConfig()
    if err != nil {
        log.Fatal(err)
    }

    // Connect to Redis
    redisQueue, err := queue.NewRedisQueue(cfg.RedisURL)
    if err != nil {
        log.Fatal(err)
    }
    defer redisQueue.Close()

    // Create handler registry
    registry := worker.NewRegistry()
    registry.Register("send_welcome_email", handleWelcomeEmail)

    // Create executor and worker pool
    executor := worker.NewExecutor(registry, redisQueue, cfg.Concurrency)
    pool := worker.NewPool(executor, redisQueue, cfg.Concurrency, cfg.JobTimeout)

    // Start worker pool
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go pool.Start(ctx)
    log.Printf("Worker pool started with %d workers", cfg.Concurrency)

    // Graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    <-sigChan

    log.Println("Shutting down gracefully...")
    cancel()
    pool.Stop()
    log.Println("Shutdown complete")
}

func handleWelcomeEmail(ctx context.Context, j *job.Job) error {
    var payload struct {
        Email string `json:"email"`
        Name  string `json:"name"`
    }

    if err := json.Unmarshal(j.Payload, &payload); err != nil {
        return err
    }

    log.Printf("Sending welcome email to %s (%s)", payload.Name, payload.Email)

    // Your email sending logic here...

    return nil
}
```

### Step 4: Run the System

```bash
# Terminal 1: Start worker
REDIS_URL=redis://localhost:6379 \
WORKER_CONCURRENCY=5 \
JOB_TIMEOUT=5m \
go run worker.go

# Terminal 2: Submit jobs
go run main.go
```

---

## Client SDK Guide

### Creating a Client

```go
import "github.com/muaviaUsmani/bananas/pkg/client"

// With default Redis URL (redis://localhost:6379)
c, err := client.NewClient()

// With custom Redis URL
c, err := client.NewClient("redis://redis.example.com:6379")

// With Redis password
c, err := client.NewClient("redis://:password@localhost:6379")

// Always close when done
defer c.Close()
```

### Submitting Jobs

#### Basic Job Submission

```go
jobID, err := c.SubmitJob(
    "job_name",         // handler name
    payload,            // JSON-serializable data
    job.PriorityNormal, // priority
    "Optional description",
)
```

#### Job with Routing Key

```go
jobID, err := c.SubmitJobWithRoute(
    "process_image",
    payload,
    job.PriorityHigh,
    "gpu", // routing key
    "Process user avatar",
)
```

#### Synchronous Job (Wait for Result)

```go
result, err := c.SubmitAndWait(
    ctx,
    "generate_report",
    payload,
    job.PriorityNormal,
    5*time.Minute, // timeout
)

if err != nil {
    log.Fatalf("Job failed: %v", err)
}

// Use the result
var report Report
json.Unmarshal(result.Data, &report)
```

### Priority Levels

Choose priority based on business requirements:

```go
// High Priority: User-facing, time-sensitive
// - OTP codes, password resets
// - Real-time notifications
// - Critical API callbacks
c.SubmitJob("send_otp", data, job.PriorityHigh)

// Normal Priority: Standard operations (default)
// - Order processing
// - Email notifications
// - Image processing
c.SubmitJob("process_order", data, job.PriorityNormal)

// Low Priority: Background tasks, non-urgent
// - Report generation
// - Data cleanup
// - Analytics
c.SubmitJob("generate_report", data, job.PriorityLow)
```

### Checking Job Status

```go
// Get job details
j, err := c.GetJob(jobID)
if err != nil {
    log.Printf("Job not found: %v", err)
    return
}

// Check status
switch j.Status {
case job.StatusPending:
    log.Println("Job waiting in queue")
case job.StatusProcessing:
    log.Println("Job currently running")
case job.StatusCompleted:
    log.Println("Job finished successfully")
case job.StatusFailed:
    log.Printf("Job failed: %s", j.Error)
case job.StatusScheduled:
    log.Printf("Job scheduled for retry at %s", j.ScheduledFor)
}

log.Printf("Attempts: %d/%d", j.Attempts, j.MaxRetries)
```

### Getting Job Results

```go
// For jobs that return results
result, err := c.GetResult(ctx, jobID, 5*time.Minute)
if err != nil {
    log.Fatalf("Failed to get result: %v", err)
}

// Parse result data
var output MyOutputType
if err := json.Unmarshal(result.Data, &output); err != nil {
    log.Fatalf("Failed to parse result: %v", err)
}

log.Printf("Job completed in %s", result.Duration)
log.Printf("Result: %+v", output)
```

### Complex Payloads

Any JSON-serializable data works:

```go
type OrderPayload struct {
    OrderID    string    `json:"order_id"`
    CustomerID string    `json:"customer_id"`
    Items      []Item    `json:"items"`
    Total      float64   `json:"total"`
    Metadata   map[string]interface{} `json:"metadata"`
    CreatedAt  time.Time `json:"created_at"`
}

payload := OrderPayload{
    OrderID:    "ORD-12345",
    CustomerID: "CUST-789",
    Items:      []Item{{SKU: "WIDGET", Qty: 2, Price: 24.99}},
    Total:      49.98,
    Metadata:   map[string]interface{}{"source": "mobile_app"},
    CreatedAt:  time.Now(),
}

jobID, err := c.SubmitJob("process_order", payload, job.PriorityHigh)
```

---

## Creating Job Handlers

### Handler Function Signature

All handlers must implement:

```go
func HandlerName(ctx context.Context, j *job.Job) error
```

- **ctx**: Context for cancellation and timeouts
- **j**: Job with ID, Name, Payload, Priority, etc.
- **return**: `nil` for success, `error` for failure (will retry)

### Example: Email Handler

```go
import (
    "context"
    "encoding/json"
    "fmt"
    "log"

    "github.com/muaviaUsmani/bananas/internal/job"
)

type EmailPayload struct {
    To      string `json:"to"`
    Subject string `json:"subject"`
    Body    string `json:"body"`
}

func HandleSendEmail(ctx context.Context, j *job.Job) error {
    // 1. Parse payload
    var payload EmailPayload
    if err := json.Unmarshal(j.Payload, &payload); err != nil {
        return fmt.Errorf("invalid payload: %w", err)
    }

    // 2. Validate
    if payload.To == "" {
        return fmt.Errorf("recipient email is required")
    }

    // 3. Execute business logic
    log.Printf("Sending email to %s: %s", payload.To, payload.Subject)

    err := yourEmailService.Send(ctx, email.Message{
        To:      payload.To,
        Subject: payload.Subject,
        Body:    payload.Body,
    })

    if err != nil {
        return fmt.Errorf("failed to send email: %w", err)
    }

    // 4. Return nil on success
    return nil
}
```

### Example: Image Processing Handler

```go
func HandleResizeImage(ctx context.Context, j *job.Job) error {
    var payload struct {
        ImageURL string `json:"image_url"`
        Width    int    `json:"width"`
        Height   int    `json:"height"`
    }

    if err := json.Unmarshal(j.Payload, &payload); err != nil {
        return err
    }

    // Download image
    img, err := downloadImage(ctx, payload.ImageURL)
    if err != nil {
        return fmt.Errorf("download failed: %w", err)
    }

    // Process with context cancellation check
    select {
    case <-ctx.Done():
        return ctx.Err() // Job timed out or cancelled
    default:
        resized := resizeImage(img, payload.Width, payload.Height)

        // Upload result
        url, err := uploadImage(ctx, resized)
        if err != nil {
            return fmt.Errorf("upload failed: %w", err)
        }

        log.Printf("Image processed: %s", url)
    }

    return nil
}
```

### Example: Handler with Result

```go
func HandleGenerateReport(ctx context.Context, j *job.Job) error {
    var payload struct {
        StartDate string `json:"start_date"`
        EndDate   string `json:"end_date"`
    }

    if err := json.Unmarshal(j.Payload, &payload); err != nil {
        return err
    }

    // Generate report
    report, err := generateReport(ctx, payload.StartDate, payload.EndDate)
    if err != nil {
        return err
    }

    // Store result (for SubmitAndWait clients)
    resultData, _ := json.Marshal(report)
    j.SetResult(resultData)

    return nil
}
```

### Handler Best Practices

#### ✅ DO

**1. Respect context cancellation**
```go
func HandleLongRunning(ctx context.Context, j *job.Job) error {
    for i := 0; i < 1000; i++ {
        // Check for cancellation regularly
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            processItem(i)
        }
    }
    return nil
}
```

**2. Return descriptive errors**
```go
return fmt.Errorf("failed to connect to API: %w", err)
return fmt.Errorf("invalid customer ID %s: not found", customerID)
```

**3. Use structured logging**
```go
log.Printf("[%s] Processing order %s for customer %s",
    j.ID, orderID, customerID)
```

**4. Validate input early**
```go
if payload.Amount <= 0 {
    return fmt.Errorf("invalid amount: %f", payload.Amount)
}
if payload.CustomerID == "" {
    return fmt.Errorf("customer_id is required")
}
```

**5. Make handlers idempotent**
```go
// Check if already processed
if order.IsProcessed() {
    log.Printf("Order %s already processed, skipping", order.ID)
    return nil // Safe to retry
}

// Process with idempotency key
err := payment.ProcessWithIdempotencyKey(order.ID, amount)
```

**6. Distinguish retriable vs. permanent errors**
```go
resp, err := api.Call(payload)
if err != nil {
    // Network error - will retry
    if isNetworkError(err) {
        return err
    }

    // Invalid data - won't retry, move to DLQ
    if resp.StatusCode == 400 {
        return fmt.Errorf("permanent error: invalid data: %w", err)
    }

    // Server error - will retry
    return err
}
```

#### ❌ DON'T

**1. Don't update job status directly**
```go
// ❌ Wrong - executor handles this
j.Status = job.StatusCompleted

// ✅ Correct - just return nil
return nil
```

**2. Don't panic**
```go
// ❌ Wrong
if err != nil {
    panic(err)
}

// ✅ Correct
if err != nil {
    return err
}
```

**3. Don't block forever**
```go
// ❌ Wrong - ignores context
time.Sleep(1 * time.Hour)

// ✅ Correct - respects context
select {
case <-time.After(1 * time.Hour):
    // Process
case <-ctx.Done():
    return ctx.Err()
}
```

**4. Don't swallow errors**
```go
// ❌ Wrong - job marked as successful!
if err != nil {
    log.Println(err)
    return nil
}

// ✅ Correct - job will retry
if err != nil {
    return err
}
```

### Registering Handlers

```go
func main() {
    registry := worker.NewRegistry()

    // Register all your handlers
    registry.Register("send_email", HandleSendEmail)
    registry.Register("send_sms", HandleSendSMS)
    registry.Register("resize_image", HandleResizeImage)
    registry.Register("process_order", HandleProcessOrder)
    registry.Register("generate_report", HandleGenerateReport)
    registry.Register("cleanup_old_data", HandleCleanup)

    log.Printf("Registered %d handlers", len(registry.Handlers()))

    // ... rest of worker setup
}
```

---

## Task Routing

Task routing enables you to direct jobs to specific worker pools based on routing keys. This allows resource isolation, workload segregation, and independent scaling.

### When to Use Task Routing

| Scenario | Routing Key | Workers |
|----------|-------------|---------|
| GPU-intensive jobs | `gpu` | GPU-enabled instances |
| High-memory jobs | `high_memory` | Large memory instances |
| Email sending | `email` | Email-optimized workers |
| Geographic distribution | `us-east-1`, `eu-west-1` | Regional workers |
| Team isolation | `team-a`, `team-b` | Team-specific workers |
| Critical vs. batch | `critical`, `batch` | Dedicated pools |

### Submitting Routed Jobs

```go
// GPU job → GPU workers
jobID, err := c.SubmitJobWithRoute(
    "process_image",
    payload,
    job.PriorityHigh,
    "gpu",
)

// Email job → Email workers
jobID, err = c.SubmitJobWithRoute(
    "send_email",
    payload,
    job.PriorityNormal,
    "email",
)

// Regional job → Regional workers
jobID, err = c.SubmitJobWithRoute(
    "process_payment",
    payload,
    job.PriorityHigh,
    "us-east-1",
)

// Default job (backward compatible)
jobID, err = c.SubmitJob(
    "generate_report",
    payload,
    job.PriorityLow,
)
```

### Configuring Workers with Routing Keys

#### Single Routing Key

```bash
# GPU worker (only processes GPU jobs)
WORKER_ROUTING_KEYS=gpu \
WORKER_CONCURRENCY=5 \
./worker
```

#### Multiple Routing Keys

Workers can handle multiple routing keys with priority ordering:

```bash
# Handles GPU jobs first, then default jobs
WORKER_ROUTING_KEYS=gpu,default \
WORKER_CONCURRENCY=10 \
./worker
```

Dequeue order:
1. `gpu:high`
2. `gpu:normal`
3. `gpu:low`
4. `default:high`
5. `default:normal`
6. `default:low`

#### Default Worker

```bash
# Handles default jobs (backward compatible)
./worker  # Defaults to WORKER_ROUTING_KEYS=default
```

### Routing Strategies

#### Resource Isolation

Route jobs requiring specific resources to specialized workers:

```bash
# GPU workers on GPU instances
WORKER_ROUTING_KEYS=gpu ./worker

# High-memory workers on large instances
WORKER_ROUTING_KEYS=high_memory ./worker

# Email workers with email service credentials
WORKER_ROUTING_KEYS=email ./worker
```

#### Workload Segregation

Separate critical and batch workloads:

```bash
# Critical workers (high SLA)
WORKER_ROUTING_KEYS=critical \
WORKER_CONCURRENCY=20 \
./worker

# Batch workers (best effort)
WORKER_ROUTING_KEYS=batch \
WORKER_CONCURRENCY=5 \
./worker
```

#### Geographic Distribution

Route jobs to workers in specific regions:

```bash
# US East workers
WORKER_ROUTING_KEYS=us-east-1 ./worker

# EU West workers
WORKER_ROUTING_KEYS=eu-west-1 ./worker

# Multi-region worker (handles both)
WORKER_ROUTING_KEYS=us-east-1,eu-west-1 ./worker
```

### Monitoring Routed Queues

```bash
# Check queue depth per routing key
redis-cli LLEN bananas:route:gpu:queue:high
redis-cli LLEN bananas:route:gpu:queue:normal
redis-cli LLEN bananas:route:gpu:queue:low

redis-cli LLEN bananas:route:email:queue:high
redis-cli LLEN bananas:route:email:queue:normal
redis-cli LLEN bananas:route:email:queue:low
```

For complete task routing documentation, see [Task Routing Usage Guide](docs/TASK_ROUTING_USAGE.md).

---

## Result Backend

The result backend enables synchronous job execution where clients wait for job completion and receive results.

### When to Use Result Backend

| Use Case | Pattern |
|----------|---------|
| Generate PDF and download | `SubmitAndWait()` |
| Process data and return summary | `SubmitAndWait()` |
| Validate data asynchronously | `SubmitAndWait()` |
| Background computation with result | `SubmitAndWait()` |

### Synchronous Job Execution

```go
ctx := context.Background()

// Submit job and wait for result (5 minute timeout)
result, err := c.SubmitAndWait(
    ctx,
    "generate_report",
    map[string]string{
        "start_date": "2025-01-01",
        "end_date":   "2025-01-31",
    },
    job.PriorityHigh,
    5*time.Minute,
)

if err != nil {
    log.Fatalf("Job failed: %v", err)
}

// Parse result
var report Report
if err := json.Unmarshal(result.Data, &report); err != nil {
    log.Fatalf("Failed to parse result: %v", err)
}

log.Printf("Report generated in %s", result.Duration)
log.Printf("Total sales: $%.2f", report.TotalSales)
```

### Handler with Result

Handlers store results using `SetResult()`:

```go
func HandleGenerateReport(ctx context.Context, j *job.Job) error {
    var payload struct {
        StartDate string `json:"start_date"`
        EndDate   string `json:"end_date"`
    }

    if err := json.Unmarshal(j.Payload, &payload); err != nil {
        return err
    }

    // Generate report
    report := Report{
        StartDate:  payload.StartDate,
        EndDate:    payload.EndDate,
        TotalSales: calculateSales(payload.StartDate, payload.EndDate),
        OrderCount: countOrders(payload.StartDate, payload.EndDate),
    }

    // Store result (will be returned to client)
    resultData, _ := json.Marshal(report)
    j.SetResult(resultData)

    return nil
}
```

### Asynchronous with Manual Result Retrieval

For more control, submit job and retrieve result separately:

```go
// Submit job
jobID, err := c.SubmitJob("process_data", payload, job.PriorityNormal)
if err != nil {
    log.Fatal(err)
}

// Do other work...

// Retrieve result later
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

result, err := c.GetResult(ctx, jobID, 5*time.Minute)
if err != nil {
    log.Fatalf("Failed to get result: %v", err)
}

log.Printf("Result: %s", string(result.Data))
```

---

## Periodic Tasks

Schedule jobs to run on a recurring basis using cron expressions.

### Creating Periodic Tasks

```go
import (
    "github.com/muaviaUsmani/bananas/internal/scheduler"
    "github.com/muaviaUsmani/bananas/internal/job"
)

func main() {
    // Create cron scheduler
    cronScheduler := scheduler.NewCronScheduler(redisQueue)

    // Schedule nightly cleanup (every day at 2 AM)
    err := cronScheduler.Schedule(scheduler.Schedule{
        ID:       "nightly-cleanup",
        Name:     "cleanup_old_data",
        Cron:     "0 2 * * *",
        Payload:  json.RawMessage(`{"days": 30}`),
        Priority: job.PriorityLow,
    })

    // Schedule hourly reports (every hour at :00)
    err = cronScheduler.Schedule(scheduler.Schedule{
        ID:       "hourly-report",
        Name:     "generate_hourly_report",
        Cron:     "0 * * * *",
        Payload:  json.RawMessage(`{}`),
        Priority: job.PriorityNormal,
    })

    // Start scheduler
    ctx := context.Background()
    cronScheduler.Start(ctx)
}
```

### Cron Expression Examples

| Schedule | Cron Expression | Description |
|----------|----------------|-------------|
| Every minute | `* * * * *` | Every minute |
| Every hour | `0 * * * *` | At minute 0 of every hour |
| Every day at 2 AM | `0 2 * * *` | Daily at 02:00 |
| Every Monday at 9 AM | `0 9 * * 1` | Weekly on Monday |
| First day of month | `0 0 1 * *` | Monthly at midnight |
| Every 15 minutes | `*/15 * * * *` | Every 15 minutes |
| Business hours | `0 9-17 * * 1-5` | Hourly, 9 AM - 5 PM, weekdays |

### Periodic Task Handlers

```go
func HandleNightlyCleanup(ctx context.Context, j *job.Job) error {
    var payload struct {
        Days int `json:"days"`
    }

    if err := json.Unmarshal(j.Payload, &payload); err != nil {
        return err
    }

    log.Printf("Cleaning up data older than %d days", payload.Days)

    cutoffDate := time.Now().AddDate(0, 0, -payload.Days)
    rowsDeleted, err := db.DeleteOldRecords(cutoffDate)
    if err != nil {
        return err
    }

    log.Printf("Deleted %d old records", rowsDeleted)
    return nil
}
```

---

## Deployment Strategies

### Docker Compose (Development & Small Production)

**Best for**: Development, staging, small deployments

```yaml
# docker-compose.yml
version: '3.8'

services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    command: redis-server --appendonly yes

  worker:
    build: .
    environment:
      REDIS_URL: redis://redis:6379
      WORKER_CONCURRENCY: 10
      JOB_TIMEOUT: 5m
      MAX_RETRIES: 3
    depends_on:
      - redis
    restart: unless-stopped

  worker-gpu:
    build: .
    environment:
      REDIS_URL: redis://redis:6379
      WORKER_ROUTING_KEYS: gpu
      WORKER_CONCURRENCY: 5
      JOB_TIMEOUT: 30m
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
    depends_on:
      - redis
    restart: unless-stopped

  scheduler:
    build: .
    command: ./scheduler
    environment:
      REDIS_URL: redis://redis:6379
    depends_on:
      - redis
    restart: unless-stopped

volumes:
  redis-data:
```

**Start services**:
```bash
docker-compose up -d

# Scale workers
docker-compose up -d --scale worker=5

# View logs
docker-compose logs -f worker
```

---

### Kubernetes (Production)

**Best for**: Large deployments, auto-scaling, cloud-native

See [DEPLOYMENT.md](docs/DEPLOYMENT.md) for complete Kubernetes manifests including:
- StatefulSet for Redis
- Deployments for workers and scheduler
- HorizontalPodAutoscaler for auto-scaling
- ConfigMaps and Secrets
- Service and Ingress
- PersistentVolumeClaims

**Quick deployment**:
```bash
# Deploy Redis
kubectl apply -f k8s/redis-statefulset.yaml

# Deploy workers
kubectl apply -f k8s/worker-deployment.yaml

# Deploy scheduler
kubectl apply -f k8s/scheduler-deployment.yaml

# Scale workers
kubectl scale deployment bananas-worker --replicas=10

# Auto-scale based on CPU
kubectl autoscale deployment bananas-worker \
  --cpu-percent=70 \
  --min=3 \
  --max=20
```

---

### Systemd (Traditional VMs)

**Best for**: Traditional VM deployments, non-containerized environments

```ini
# /etc/systemd/system/bananas-worker.service
[Unit]
Description=Bananas Worker
After=network.target redis.service

[Service]
Type=simple
User=bananas
WorkingDirectory=/opt/bananas
Environment="REDIS_URL=redis://localhost:6379"
Environment="WORKER_CONCURRENCY=10"
Environment="JOB_TIMEOUT=5m"
Environment="MAX_RETRIES=3"
ExecStart=/opt/bananas/worker
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

**Manage service**:
```bash
# Enable and start
sudo systemctl enable bananas-worker
sudo systemctl start bananas-worker

# Check status
sudo systemctl status bananas-worker

# View logs
sudo journalctl -u bananas-worker -f
```

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_URL` | `redis://localhost:6379` | Redis connection URL |
| `WORKER_CONCURRENCY` | `5` | Number of concurrent workers |
| `JOB_TIMEOUT` | `5m` | Max execution time per job |
| `MAX_RETRIES` | `3` | Max retry attempts before DLQ |
| `WORKER_ROUTING_KEYS` | `default` | Comma-separated routing keys |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `LOG_FORMAT` | `json` | Log format (json, text) |

### Configuration Examples

**Development**:
```bash
export REDIS_URL=redis://localhost:6379
export WORKER_CONCURRENCY=2
export JOB_TIMEOUT=30s
export MAX_RETRIES=1
export LOG_LEVEL=debug
export LOG_FORMAT=text
```

**Production**:
```bash
export REDIS_URL=redis://:password@redis-cluster:6379
export WORKER_CONCURRENCY=20
export JOB_TIMEOUT=10m
export MAX_RETRIES=5
export LOG_LEVEL=info
export LOG_FORMAT=json
export WORKER_ROUTING_KEYS=default,email
```

**GPU Workers**:
```bash
export REDIS_URL=redis://redis:6379
export WORKER_ROUTING_KEYS=gpu
export WORKER_CONCURRENCY=3
export JOB_TIMEOUT=30m
export MAX_RETRIES=2
```

---

## Best Practices

### 1. Job Design

**Keep jobs small and focused**:
```go
// ❌ Bad - too much in one job
SubmitJob("process_signup", signupData, ...)

// ✅ Good - split into multiple jobs
SubmitJob("send_welcome_email", emailData, ...)
SubmitJob("create_user_profile", profileData, ...)
SubmitJob("notify_sales_team", salesData, ...)
```

**Make jobs idempotent**:
```go
func HandleProcessPayment(ctx context.Context, j *job.Job) error {
    // Check if already processed
    if payment.IsProcessed(payload.OrderID) {
        log.Printf("Payment %s already processed", payload.OrderID)
        return nil // Safe to retry
    }

    // Process with idempotency key
    return payment.ProcessWithIdempotencyKey(payload.OrderID, payload.Amount)
}
```

**Use descriptive job names**:
```go
// ❌ Bad
c.SubmitJob("job1", data, ...)

// ✅ Good
c.SubmitJob("send_welcome_email", data, ...)
c.SubmitJob("process_order_payment", data, ...)
```

### 2. Error Handling

**Distinguish retriable vs. permanent errors**:
```go
func HandleAPICall(ctx context.Context, j *job.Job) error {
    resp, err := api.Call(payload)

    if err != nil {
        // Network error - will retry
        if isNetworkError(err) {
            return fmt.Errorf("network error (will retry): %w", err)
        }

        // Invalid data - permanent error, move to DLQ
        if resp.StatusCode == 400 {
            return fmt.Errorf("permanent error: invalid data: %w", err)
        }

        // Server error - will retry
        if resp.StatusCode >= 500 {
            return fmt.Errorf("server error (will retry): %w", err)
        }
    }

    return nil
}
```

**Log errors with context**:
```go
if err != nil {
    log.Printf("[%s] Failed to process order %s: %v",
        j.ID, orderID, err)
    return fmt.Errorf("failed to process order %s: %w", orderID, err)
}
```

### 3. Resource Management

**Use appropriate concurrency**:
```bash
# CPU-intensive: concurrency = CPU cores
WORKER_CONCURRENCY=8

# I/O-bound: concurrency = 2-3x CPU cores
WORKER_CONCURRENCY=24

# Memory-intensive: concurrency = memory / per-job-memory
WORKER_CONCURRENCY=4
```

**Set appropriate timeouts**:
```bash
# Quick jobs (API calls, emails)
JOB_TIMEOUT=30s

# Standard jobs (image processing)
JOB_TIMEOUT=5m

# Long jobs (video processing, reports)
JOB_TIMEOUT=30m
```

**Implement rate limiting**:
```go
var limiter = rate.NewLimiter(rate.Every(time.Second), 10) // 10 req/sec

func HandleAPICall(ctx context.Context, j *job.Job) error {
    // Wait for rate limit
    if err := limiter.Wait(ctx); err != nil {
        return err // Context cancelled or timed out
    }

    // Make API call
    return callExternalAPI(payload)
}
```

### 4. Monitoring

**Add structured logging**:
```go
import "go.uber.org/zap"

func HandleJob(ctx context.Context, j *job.Job) error {
    logger := zap.L().With(
        zap.String("job_id", j.ID),
        zap.String("job_name", j.Name),
        zap.String("routing_key", j.RoutingKey),
    )

    start := time.Now()
    logger.Info("Starting job processing")

    // Process job
    err := processJob(ctx, j)

    if err != nil {
        logger.Error("Job failed",
            zap.Error(err),
            zap.Duration("duration", time.Since(start)),
        )
        return err
    }

    logger.Info("Job completed",
        zap.Duration("duration", time.Since(start)),
    )

    return nil
}
```

**Implement health checks**:
```go
http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    // Check Redis connection
    if err := redisQueue.Ping(); err != nil {
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]string{
            "status": "unhealthy",
            "error":  err.Error(),
        })
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "healthy",
    })
})
```

### 5. Graceful Shutdown

**Handle signals properly**:
```go
func main() {
    pool := worker.NewPool(...)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go pool.Start(ctx)

    // Listen for shutdown signals
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    <-sigChan
    log.Println("Received shutdown signal, draining...")

    // Stop accepting new jobs
    cancel()

    // Wait for active jobs to finish (with timeout)
    shutdownCtx, shutdownCancel := context.WithTimeout(
        context.Background(),
        30*time.Second,
    )
    defer shutdownCancel()

    pool.StopGracefully(shutdownCtx)

    log.Println("Shutdown complete")
}
```

---

## Monitoring & Observability

### Metrics to Track

**Job Metrics**:
- Jobs enqueued per minute (by name, priority, routing key)
- Jobs completed per minute (by name, status)
- Jobs failed per minute (by name, error type)
- Average job duration (by name)
- Job queue depth (by priority, routing key)

**Worker Metrics**:
- Active workers (by routing key)
- Worker CPU/memory usage
- Jobs processed per worker
- Worker errors and restarts

**Queue Metrics**:
- Queue depth per priority and routing key
- Processing queue size
- Dead letter queue size
- Scheduled jobs count

### Prometheus Integration

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    jobsProcessed = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "bananas_jobs_processed_total",
            Help: "Total number of jobs processed",
        },
        []string{"job_name", "status", "routing_key"},
    )

    jobDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "bananas_job_duration_seconds",
            Help:    "Job execution duration in seconds",
            Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
        },
        []string{"job_name", "routing_key"},
    )

    queueDepth = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "bananas_queue_depth",
            Help: "Current queue depth",
        },
        []string{"priority", "routing_key"},
    )
)

func init() {
    prometheus.MustRegister(jobsProcessed)
    prometheus.MustRegister(jobDuration)
    prometheus.MustRegister(queueDepth)
}

func HandleJob(ctx context.Context, j *job.Job) error {
    start := time.Now()

    err := processJob(j)

    status := "success"
    if err != nil {
        status = "failed"
    }

    jobsProcessed.WithLabelValues(j.Name, status, j.RoutingKey).Inc()
    jobDuration.WithLabelValues(j.Name, j.RoutingKey).Observe(
        time.Since(start).Seconds(),
    )

    return err
}
```

### Grafana Dashboard

Key panels to include:
- Job throughput (jobs/sec) by status
- Job duration (p50, p95, p99) by job name
- Queue depth over time by routing key
- Worker count and CPU/memory usage
- Error rate and DLQ size
- Job processing latency

---

## Troubleshooting

### Jobs Not Processing

**Check workers are running**:
```bash
docker ps | grep worker
# or
kubectl get pods -l app=bananas-worker
# or
systemctl status bananas-worker
```

**Check Redis connectivity**:
```bash
redis-cli -u $REDIS_URL ping
# Expected: PONG
```

**Check jobs in queue**:
```bash
# Check default queues
redis-cli LLEN bananas:route:default:queue:high
redis-cli LLEN bananas:route:default:queue:normal
redis-cli LLEN bananas:route:default:queue:low

# Check routed queues
redis-cli LLEN bananas:route:gpu:queue:high
redis-cli LLEN bananas:route:email:queue:normal
```

**Check worker logs**:
```bash
docker logs bananas-worker
# or
kubectl logs -l app=bananas-worker
# or
journalctl -u bananas-worker -f
```

### Jobs Failing Repeatedly

**View failed jobs**:
```bash
redis-cli LRANGE bananas:queue:dead 0 -1
```

**Check job error**:
```bash
redis-cli HGET bananas:job:{job_id} error
redis-cli HGET bananas:job:{job_id} attempts
```

**Verify handler registration**:
```go
registry := worker.NewRegistry()
registry.Register("job_name", HandlerFunc)

// Log registered handlers
for name := range registry.Handlers() {
    log.Printf("Registered handler: %s", name)
}
```

### High Memory Usage

**Reduce worker concurrency**:
```bash
WORKER_CONCURRENCY=3 ./worker
```

**Set job timeout**:
```bash
JOB_TIMEOUT=2m ./worker
```

**Limit Redis memory**:
```bash
redis-cli CONFIG SET maxmemory 256mb
redis-cli CONFIG SET maxmemory-policy allkeys-lru
```

### Slow Job Processing

**Increase worker concurrency**:
```bash
WORKER_CONCURRENCY=20 ./worker
```

**Scale workers horizontally**:
```bash
docker-compose up -d --scale worker=5
# or
kubectl scale deployment bananas-worker --replicas=10
```

**Profile handlers**:
```go
import _ "net/http/pprof"

go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()

// View CPU profile: http://localhost:6060/debug/pprof/profile
// View memory profile: http://localhost:6060/debug/pprof/heap
```

---

## Migration Guide

### From Celery

Bananas provides 90% feature parity with Celery. Here's how to migrate:

| Celery Feature | Bananas Equivalent |
|----------------|-------------------|
| `task.delay()` | `SubmitJob()` |
| `task.apply_async(queue='gpu')` | `SubmitJobWithRoute(..., "gpu")` |
| `task.apply_async().get()` | `SubmitAndWait()` |
| `@app.task` | `registry.Register()` |
| `beat` scheduler | Cron scheduler |
| Priority queues | Built-in (high, normal, low) |
| Routing | Task routing with routing keys |
| Result backend | Built-in result backend |

**Example migration**:

```python
# Celery
@app.task
def send_email(to, subject, body):
    # ...

send_email.apply_async(
    args=['user@example.com', 'Welcome', 'Hello!'],
    queue='email',
    priority=9
)
```

```go
// Bananas
func HandleSendEmail(ctx context.Context, j *job.Job) error {
    var payload struct {
        To      string `json:"to"`
        Subject string `json:"subject"`
        Body    string `json:"body"`
    }
    json.Unmarshal(j.Payload, &payload)
    // ...
}

registry.Register("send_email", HandleSendEmail)

c.SubmitJobWithRoute(
    "send_email",
    map[string]string{
        "to": "user@example.com",
        "subject": "Welcome",
        "body": "Hello!",
    },
    job.PriorityHigh,
    "email",
)
```

### From RabbitMQ

| RabbitMQ Concept | Bananas Equivalent |
|------------------|-------------------|
| Exchange | Routing keys |
| Queue | Priority + routing key queues |
| Routing key | Routing key |
| Consumer | Worker |
| Publisher | Client |

---

## Related Documentation

- **[Architecture](docs/ARCHITECTURE.md)** - System architecture and design decisions
- **[API Reference](docs/API_REFERENCE.md)** - Complete API documentation
- **[Deployment Guide](docs/DEPLOYMENT.md)** - Production deployment patterns
- **[Task Routing](docs/TASK_ROUTING_USAGE.md)** - Detailed task routing guide
- **[Contributing](CONTRIBUTING.md)** - Developer contribution guide
- **[README](README.md)** - Project overview and quick start

---

**Happy integrating! 🍌**

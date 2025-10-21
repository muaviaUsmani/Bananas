# 🍌 Bananas Integration Guide

A comprehensive guide for integrating the Bananas distributed task queue into your existing infrastructure and projects.

## Table of Contents

1. [Overview](#overview)
2. [Integration Options](#integration-options)
3. [Quick Start](#quick-start)
4. [Using the Client Library](#using-the-client-library)
5. [Creating Custom Job Handlers](#creating-custom-job-handlers)
6. [Deployment Strategies](#deployment-strategies)
7. [Configuration Management](#configuration-management)
8. [Best Practices](#best-practices)
9. [Monitoring & Observability](#monitoring--observability)
10. [Troubleshooting](#troubleshooting)

---

## Overview

Bananas is a distributed task queue system built in Go that provides:
- ✅ **Priority-based job processing** (high, normal, low)
- ✅ **Exponential backoff retries** with configurable max attempts
- ✅ **Dead letter queue** for failed jobs
- ✅ **Concurrent worker pools** with timeout enforcement
- ✅ **Redis-backed persistence** for reliability
- ✅ **Clean SDK** for easy integration

**Use cases**:
- Background email sending
- Image/video processing
- Report generation
- Data imports/exports
- Webhook delivery
- Scheduled tasks
- Long-running computations

---

## Integration Options

You can integrate Bananas in three ways depending on your needs:

### Option 1: As a Library (Embedded)

**Best for**: Monolithic applications, tight coupling, simplest setup

Import Bananas packages directly into your Go application:

```go
import (
    "github.com/muaviaUsmani/bananas/pkg/client"
    "github.com/muaviaUsmani/bananas/internal/worker"
    "github.com/muaviaUsmani/bananas/internal/queue"
)
```

**Pros**:
- No separate services to deploy
- Lowest latency
- Shared memory and resources

**Cons**:
- Tightly coupled to your application
- Can't scale workers independently
- Restart required for handler updates

---

### Option 2: As Microservices (Recommended)

**Best for**: Distributed systems, independent scaling, production deployments

Deploy Bananas services separately using Docker Compose or Kubernetes:

```yaml
services:
  bananas-worker:    # Process jobs
  bananas-scheduler: # Manage retries
  bananas-redis:     # Job storage
  your-app:          # Submit jobs via SDK
```

**Pros**:
- Independent scaling (add more workers as needed)
- Isolation (worker crashes don't affect your app)
- Update handlers without redeploying your app
- Better resource management

**Cons**:
- More infrastructure to manage
- Network latency for job submission
- Additional deployment complexity

---

### Option 3: Hybrid Approach

**Best for**: Gradual migration, flexibility

Run workers as separate services but embed the client SDK:

```go
// In your application
client := client.NewClient()
client.SubmitJob("process_order", orderData, job.PriorityHigh)

// Workers run separately in containers
```

**Pros**:
- Best of both worlds
- Easy migration path
- Flexible deployment

---

## Quick Start

### Step 1: Install Dependencies

Add Bananas to your Go project:

```bash
go get github.com/muaviaUsmani/bananas@latest
```

### Step 2: Start Redis

Bananas requires Redis for job storage:

```bash
# Using Docker
docker run -d -p 6379:6379 redis:7-alpine

# Or use existing Redis instance
export REDIS_URL="redis://your-redis-host:6379"
```

### Step 3: Submit Your First Job

```go
package main

import (
    "log"
    
    "github.com/muaviaUsmani/bananas/pkg/client"
    "github.com/muaviaUsmani/bananas/internal/job"
)

func main() {
    // Create client
    c := client.NewClient()
    
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

### Step 4: Create a Worker

Create a separate worker application to process jobs:

```go
package main

import (
    "context"
    "log"
    
    "github.com/muaviaUsmani/bananas/internal/config"
    "github.com/muaviaUsmani/bananas/internal/job"
    "github.com/muaviaUsmani/bananas/internal/queue"
    "github.com/muaviaUsmani/bananas/internal/worker"
)

func main() {
    // Load config
    cfg, _ := config.LoadConfig()
    
    // Connect to Redis
    redisQueue, _ := queue.NewRedisQueue(cfg.RedisURL)
    defer redisQueue.Close()
    
    // Create handler registry
    registry := worker.NewRegistry()
    registry.Register("send_welcome_email", handleWelcomeEmail)
    
    // Create executor
    executor := worker.NewExecutor(registry, redisQueue, cfg.WorkerConcurrency)
    
    // Create and start worker pool
    pool := worker.NewPool(executor, redisQueue, cfg.WorkerConcurrency, cfg.JobTimeout)
    
    ctx := context.Background()
    pool.Start(ctx)
    
    // Block forever (or until SIGTERM)
    select {}
}

func handleWelcomeEmail(ctx context.Context, j *job.Job) error {
    var payload struct {
        Email string `json:"email"`
        Name  string `json:"name"`
    }
    
    if err := json.Unmarshal(j.Payload, &payload); err != nil {
        return err
    }
    
    // Send email using your email service
    log.Printf("Sending welcome email to %s (%s)", payload.Name, payload.Email)
    
    // Your email sending logic here...
    
    return nil
}
```

### Step 5: Run the System

```bash
# Terminal 1: Start worker
go run worker.go

# Terminal 2: Submit jobs
go run main.go
```

**That's it!** Your first job will be processed by the worker.

---

## Using the Client Library

### Basic Usage

```go
import (
    "github.com/muaviaUsmani/bananas/pkg/client"
    "github.com/muaviaUsmani/bananas/internal/job"
)

// Initialize once, reuse throughout your application
var jobClient = client.NewClient()

// Submit a job
jobID, err := jobClient.SubmitJob(
    "job_name",
    payload,
    job.PriorityNormal,
)
```

### Priority Levels

Choose priority based on business requirements:

```go
// Critical: User-facing, time-sensitive
jobClient.SubmitJob("send_otp", data, job.PriorityHigh)

// Standard: Most jobs should use this
jobClient.SubmitJob("process_order", data, job.PriorityNormal)

// Background: Reports, cleanup, non-urgent tasks
jobClient.SubmitJob("generate_report", data, job.PriorityLow)
```

### Complex Payloads

Any JSON-serializable data works:

```go
type OrderPayload struct {
    OrderID    string    `json:"order_id"`
    CustomerID string    `json:"customer_id"`
    Items      []Item    `json:"items"`
    Total      float64   `json:"total"`
    CreatedAt  time.Time `json:"created_at"`
}

payload := OrderPayload{
    OrderID:    "ORD-12345",
    CustomerID: "CUST-789",
    Items:      []Item{{SKU: "WIDGET", Qty: 2}},
    Total:      49.99,
    CreatedAt:  time.Now(),
}

jobID, err := jobClient.SubmitJob(
    "process_order",
    payload,
    job.PriorityHigh,
    "Process order ORD-12345",
)
```

### Checking Job Status

```go
// Get job details
j, err := jobClient.GetJob(jobID)
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

### List All Jobs

```go
allJobs := jobClient.ListJobs()

for _, j := range allJobs {
    log.Printf("Job %s: %s (status: %s)", 
        j.ID, j.Name, j.Status)
}
```

---

## Creating Custom Job Handlers

### Handler Function Signature

All handlers must implement:

```go
func HandlerName(ctx context.Context, j *job.Job) error
```

### Example: Email Handler

```go
import (
    "context"
    "encoding/json"
    "fmt"
    
    "github.com/muaviaUsmani/bananas/internal/job"
    "your-project/email" // Your email service
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
    err := email.Send(ctx, email.Message{
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
    
    // Resize (check for context cancellation)
    select {
    case <-ctx.Done():
        return ctx.Err() // Job timed out
    default:
        resized := resize(img, payload.Width, payload.Height)
        
        // Upload
        if err := uploadImage(ctx, resized); err != nil {
            return fmt.Errorf("upload failed: %w", err)
        }
    }
    
    return nil
}
```

### Handler Best Practices

#### ✅ DO:

1. **Respect context cancellation**
```go
select {
case <-ctx.Done():
    return ctx.Err()
default:
    // Continue processing
}
```

2. **Return descriptive errors**
```go
return fmt.Errorf("failed to connect to API: %w", err)
```

3. **Use structured logging**
```go
log.Printf("Processing order %s for customer %s", orderID, customerID)
```

4. **Validate input early**
```go
if payload.Amount <= 0 {
    return fmt.Errorf("invalid amount: %f", payload.Amount)
}
```

5. **Make handlers idempotent** (safe to retry)
```go
// Check if already processed
if alreadyProcessed(orderID) {
    return nil // Skip duplicate
}
```

#### ❌ DON'T:

1. **Don't update job status directly** (executor handles this)
```go
// ❌ Wrong
j.Status = job.StatusCompleted

// ✅ Correct - just return nil
return nil
```

2. **Don't panic** (return errors instead)
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

3. **Don't block forever** (respect timeouts)
```go
// ❌ Wrong - ignores context
time.Sleep(1 * time.Hour)

// ✅ Correct - respects context
select {
case <-time.After(1 * time.Hour):
case <-ctx.Done():
    return ctx.Err()
}
```

4. **Don't swallow errors**
```go
// ❌ Wrong
if err != nil {
    log.Println(err)
    return nil // Job marked as successful!
}

// ✅ Correct
if err != nil {
    return err // Job will retry
}
```

### Registering Handlers

```go
func setupWorker() {
    registry := worker.NewRegistry()
    
    // Register all your handlers
    registry.Register("send_email", HandleSendEmail)
    registry.Register("resize_image", HandleResizeImage)
    registry.Register("process_order", HandleProcessOrder)
    registry.Register("generate_report", HandleGenerateReport)
    
    log.Printf("Registered %d handlers", registry.Count())
}
```

---

## Deployment Strategies

### Strategy 1: Docker Compose (Development & Small Production)

**Best for**: Development, staging, small deployments

**Setup**:

1. Clone the Bananas repository:
```bash
git clone https://github.com/muaviaUsmani/bananas.git
cd bananas
```

2. Add your handlers to `cmd/worker/main.go`:
```go
registry.Register("your_job", YourHandler)
```

3. Start services:
```bash
# Development (with hot reload)
make dev

# Production
make up
```

**Pros**: Simple, all-in-one, easy to debug  
**Cons**: Single point of failure, limited scaling

---

### Strategy 2: Kubernetes (Production, High Scale)

**Best for**: Large deployments, microservices, cloud-native

**Deployment**:

```yaml
# worker-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bananas-worker
spec:
  replicas: 5  # Scale workers as needed
  selector:
    matchLabels:
      app: bananas-worker
  template:
    metadata:
      labels:
        app: bananas-worker
    spec:
      containers:
      - name: worker
        image: your-registry/bananas-worker:latest
        env:
        - name: REDIS_URL
          valueFrom:
            secretKeyRef:
              name: bananas-secrets
              key: redis-url
        - name: WORKER_CONCURRENCY
          value: "10"
        - name: JOB_TIMEOUT
          value: "5m"
        - name: MAX_RETRIES
          value: "3"
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
---
# scheduler-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bananas-scheduler
spec:
  replicas: 1  # Single scheduler instance
  selector:
    matchLabels:
      app: bananas-scheduler
  template:
    metadata:
      labels:
        app: bananas-scheduler
    spec:
      containers:
      - name: scheduler
        image: your-registry/bananas-scheduler:latest
        env:
        - name: REDIS_URL
          valueFrom:
            secretKeyRef:
              name: bananas-secrets
              key: redis-url
```

**Deploy**:
```bash
kubectl apply -f worker-deployment.yaml
kubectl apply -f scheduler-deployment.yaml

# Scale workers
kubectl scale deployment bananas-worker --replicas=10
```

**Pros**: Auto-scaling, self-healing, resource limits  
**Cons**: Kubernetes complexity, learning curve

---

### Strategy 3: Serverless Workers (AWS Lambda, Cloud Functions)

**Best for**: Sporadic workloads, cost optimization, auto-scaling

**Concept**: Package each handler as a separate Lambda function

**Example** (AWS Lambda):

```go
package main

import (
    "context"
    "encoding/json"
    
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/muaviaUsmani/bananas/internal/job"
)

// Lambda handler wrapper
func handler(ctx context.Context, event job.Job) error {
    // Your job logic here
    return HandleSendEmail(ctx, &event)
}

func main() {
    lambda.Start(handler)
}
```

**Trigger**: Use Redis Streams or SQS to trigger Lambda on new jobs

**Pros**: Pay-per-execution, infinite scaling, no infrastructure  
**Cons**: Cold starts, execution time limits, complex setup

---

### Strategy 4: Embedded Workers (Simplest)

**Best for**: Monolithic apps, simple deployments, getting started

**Code**:

```go
// main.go - Your existing application
package main

import (
    "context"
    
    "github.com/muaviaUsmani/bananas/internal/worker"
    "github.com/muaviaUsmani/bananas/internal/queue"
    "github.com/muaviaUsmani/bananas/internal/config"
)

func main() {
    // Your existing application setup...
    
    // Start worker pool in background goroutine
    go startWorkerPool()
    
    // Your existing application continues...
    startWebServer()
}

func startWorkerPool() {
    cfg, _ := config.LoadConfig()
    redisQueue, _ := queue.NewRedisQueue(cfg.RedisURL)
    
    registry := worker.NewRegistry()
    registry.Register("send_email", HandleSendEmail)
    
    executor := worker.NewExecutor(registry, redisQueue, 5)
    pool := worker.NewPool(executor, redisQueue, 5, cfg.JobTimeout)
    
    pool.Start(context.Background())
}
```

**Pros**: Simplest deployment, no separate services  
**Cons**: Workers restart with your app, can't scale independently

---

## Configuration Management

### Environment Variables

All services use these environment variables:

```bash
# Redis connection
REDIS_URL=redis://localhost:6379

# API server (if using)
API_PORT=8080

# Worker configuration
WORKER_CONCURRENCY=5    # Number of concurrent workers
JOB_TIMEOUT=5m          # Max time per job (e.g., 30s, 5m, 1h)
MAX_RETRIES=3           # Max retry attempts before DLQ

# Optional: Custom retry delays
# RETRY_DELAY_BASE=2    # Base for exponential backoff (default: 2)
```

### Development vs. Production

**Development** (`docker-compose.dev.yml`):
```yaml
environment:
  REDIS_URL: redis://redis:6379
  WORKER_CONCURRENCY: 2
  JOB_TIMEOUT: 30s
  MAX_RETRIES: 1
```

**Production** (`docker-compose.yml`):
```yaml
environment:
  REDIS_URL: redis://redis-cluster:6379
  WORKER_CONCURRENCY: 10
  JOB_TIMEOUT: 5m
  MAX_RETRIES: 3
```

### Configuration File (Alternative)

Create a `config.yaml`:

```yaml
redis:
  url: redis://localhost:6379
  
api:
  port: 8080
  
worker:
  concurrency: 10
  job_timeout: 5m
  max_retries: 3
  
scheduler:
  check_interval: 1s
```

Load in your application:

```go
cfg := loadYAMLConfig("config.yaml")
os.Setenv("REDIS_URL", cfg.Redis.URL)
os.Setenv("WORKER_CONCURRENCY", strconv.Itoa(cfg.Worker.Concurrency))
// ... etc
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
        return nil // Safe to retry
    }
    
    // Process payment
    return payment.Process(payload)
}
```

### 2. Error Handling

**Distinguish retriable vs. permanent errors**:
```go
func HandleAPICall(ctx context.Context, j *job.Job) error {
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
    
    return nil
}
```

### 3. Monitoring

**Add structured logging**:
```go
import "go.uber.org/zap"

func HandleJob(ctx context.Context, j *job.Job) error {
    logger := zap.L().With(
        zap.String("job_id", j.ID),
        zap.String("job_name", j.Name),
    )
    
    logger.Info("Starting job processing")
    
    // ... process job
    
    logger.Info("Job completed",
        zap.Duration("duration", time.Since(start)),
    )
    
    return nil
}
```

### 4. Rate Limiting

**Protect external APIs**:
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

### 5. Graceful Shutdown

**Handle SIGTERM properly**:
```go
func main() {
    pool := worker.NewPool(...)
    
    ctx, cancel := context.WithCancel(context.Background())
    pool.Start(ctx)
    
    // Listen for shutdown signals
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    
    <-sigChan
    log.Println("Shutting down gracefully...")
    
    cancel()        // Stop accepting new jobs
    pool.Stop()     // Wait for active jobs to finish
    
    log.Println("Shutdown complete")
}
```

---

## Monitoring & Observability

### Metrics to Track

1. **Job metrics**:
   - Jobs enqueued per minute
   - Jobs completed per minute
   - Jobs failed per minute
   - Average job duration
   - Jobs in each status (pending, processing, failed)

2. **Queue metrics**:
   - Queue depth per priority
   - Processing queue size
   - Dead letter queue size
   - Scheduled jobs count

3. **Worker metrics**:
   - Active workers
   - Worker CPU/memory usage
   - Jobs processed per worker
   - Worker errors

### Using Prometheus

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    jobsProcessed = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "bananas_jobs_processed_total",
            Help: "Total number of jobs processed",
        },
        []string{"job_name", "status"},
    )
    
    jobDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "bananas_job_duration_seconds",
            Help: "Job execution duration",
        },
        []string{"job_name"},
    )
)

func init() {
    prometheus.MustRegister(jobsProcessed)
    prometheus.MustRegister(jobDuration)
}

func HandleJob(ctx context.Context, j *job.Job) error {
    start := time.Now()
    
    err := processJob(j)
    
    status := "success"
    if err != nil {
        status = "failed"
    }
    
    jobsProcessed.WithLabelValues(j.Name, status).Inc()
    jobDuration.WithLabelValues(j.Name).Observe(time.Since(start).Seconds())
    
    return err
}
```

### Health Checks

```go
// Add to worker
func healthCheck() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
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
    }
}
```

---

## Troubleshooting

### Problem: Jobs Not Processing

**Check**:
1. Are workers running?
   ```bash
   docker ps | grep worker
   ```

2. Is Redis reachable?
   ```bash
   redis-cli -u $REDIS_URL ping
   ```

3. Are jobs in the queue?
   ```bash
   redis-cli LLEN bananas:queue:high
   redis-cli LLEN bananas:queue:normal
   redis-cli LLEN bananas:queue:low
   ```

4. Check worker logs:
   ```bash
   docker logs bananas-worker
   ```

### Problem: Jobs Failing Repeatedly

**Check**:
1. View failed job details:
   ```bash
   redis-cli LRANGE bananas:queue:dead 0 -1
   ```

2. Check error messages in job data:
   ```bash
   redis-cli HGET bananas:job:{job_id} error
   ```

3. Verify handler is registered:
   ```go
   log.Printf("Registered handlers: %d", registry.Count())
   ```

### Problem: High Memory Usage

**Solutions**:
1. Reduce worker concurrency:
   ```bash
   WORKER_CONCURRENCY=3
   ```

2. Add job timeout:
   ```bash
   JOB_TIMEOUT=2m
   ```

3. Limit Redis memory:
   ```bash
   redis-cli CONFIG SET maxmemory 256mb
   redis-cli CONFIG SET maxmemory-policy allkeys-lru
   ```

### Problem: Slow Job Processing

**Solutions**:
1. Increase worker concurrency:
   ```bash
   WORKER_CONCURRENCY=20
   ```

2. Scale workers horizontally:
   ```bash
   docker-compose up --scale worker=5
   ```

3. Optimize handlers (profile with pprof)

---

## Next Steps

1. **Read the main README**: [`README.md`](./README.md) for architecture details
2. **Explore the code**: 
   - [`cmd/`](./cmd/README.md) - Service entry points
   - [`internal/`](./internal/README.md) - Core packages
   - [`pkg/`](./pkg/README.md) - Client SDK
   - [`tests/`](./tests/README.md) - Test suite
3. **Run the tests**: `make test` to verify everything works
4. **Check examples**: Look at example handlers in `internal/worker/example_handlers.go`
5. **Deploy**: Choose a deployment strategy that fits your infrastructure

---

## Support & Community

- **Issues**: Report bugs or request features on GitHub
- **Discussions**: Ask questions in GitHub Discussions
- **Contributions**: Pull requests welcome!

---

**Happy queueing! 🍌**



# Complete Workflow Example

This example demonstrates the complete end-to-end workflow of the Bananas distributed task queue system.

## What This Example Demonstrates

- ✅ Defining custom job handlers for different job types
- ✅ Starting workers with registered handlers
- ✅ Starting the scheduler for scheduled job execution
- ✅ Submitting immediate jobs with different priorities
- ✅ Submitting scheduled jobs for future execution
- ✅ Monitoring job status and completion
- ✅ Priority-based job processing (High > Normal > Low)
- ✅ Graceful shutdown handling

## Prerequisites

**Redis must be running** before you can run this example. You can start Redis using:

```bash
# Using Docker
docker run -d -p 6379:6379 redis:latest

# Or using docker-compose from the project root
docker compose up -d redis

# Or if you have Redis installed locally
redis-server
```

## How to Run

1. **Start Redis** (see prerequisites above)

2. **Run the example:**

```bash
cd examples/complete_workflow
go run main.go
```

## What to Expect

When you run the example, you'll see:

### 1. **Initialization Phase**
```
========================================
   Bananas - Complete Workflow Example
========================================

Redis URL: redis://localhost:6379

Step 1: Registering job handlers...
Registered 3 handlers
```

### 2. **Worker & Scheduler Startup**
```
Step 2: Starting workers...
=== Starting Workers ===
Starting worker pool with 5 workers
Worker pool started successfully
Workers started with 5 concurrent workers

Step 3: Starting scheduler...
=== Starting Scheduler ===
Scheduler ready - monitoring scheduled jobs...
```

### 3. **Job Submission**
```
Step 4: Creating client and submitting jobs...
✓ Submitted user signup job: a1b2c3d4-...
✓ Submitted welcome email job: e5f6g7h8-...
✓ Submitted data processing job: i9j0k1l2-...
✓ Submitted scheduled email job: m3n4o5p6-... (scheduled for 2025-10-21T15:30:45Z)
```

### 4. **Job Execution (in worker logs)**
```
[UserSignup] Processing signup for John Doe (john.doe@example.com) - Plan: premium
[UserSignup] Account created for john.doe@example.com
Job a1b2c3d4-... completed successfully in 1.002s

[WelcomeEmail] Sending email to john.doe@example.com: Welcome to Bananas!
[WelcomeEmail] Email sent successfully to john.doe@example.com
Job e5f6g7h8-... completed successfully in 501ms

[DataProcessing] Processing dataset: user_analytics_2025
[DataProcessing] Progress: 1/5 - user_analytics_2025
[DataProcessing] Progress: 2/5 - user_analytics_2025
...
[DataProcessing] Completed processing: user_analytics_2025
Job i9j0k1l2-... completed successfully in 2.503s
```

### 5. **Scheduled Job Execution**
```
Scheduler: Moved 1 jobs to ready queues
[WelcomeEmail] Sending email to jane.smith@example.com: Reminder: Complete your profile
[WelcomeEmail] Email sent successfully to jane.smith@example.com
✓ Scheduled job status: completed
```

### 6. **Completion Summary**
```
========================================
   Workflow Completed Successfully!
========================================

What happened:
1. Workers started and began polling Redis queues
2. Scheduler started monitoring for scheduled jobs
3. Four jobs were submitted (3 immediate, 1 scheduled)
4. Jobs were processed in priority order (High > Normal > Low)
5. Scheduled job executed at the specified time

Key Features Demonstrated:
✓ Priority-based job processing
✓ Scheduled job execution
✓ Graceful worker shutdown
✓ Job status monitoring
✓ Custom job handlers

Press Ctrl+C to shutdown workers and scheduler...
```

## Job Processing Order

Jobs are processed in priority order:

1. **High Priority** - `user_signup` (processed first)
2. **Normal Priority** - `send_welcome_email` (processed second)
3. **Low Priority** - `data_processing` (processed last)
4. **Scheduled** - `send_welcome_email` (processed after scheduled time)

## Custom Job Handlers

The example defines three custom handlers:

### HandleUserSignup
Processes new user signups, including account creation and setup.

```go
type UserSignupPayload struct {
    Email string `json:"email"`
    Name  string `json:"name"`
    Plan  string `json:"plan"`
}
```

### HandleSendWelcomeEmail
Sends welcome and notification emails to users.

```go
type EmailPayload struct {
    To      string `json:"to"`
    Subject string `json:"subject"`
    Body    string `json:"body"`
}
```

### HandleDataProcessing
Processes large datasets with progress tracking.

```go
type DataProcessingPayload struct {
    DataSet string                 `json:"dataset"`
    Options map[string]interface{} `json:"options"`
}
```

## How to Modify for Your Use Case

1. **Define your payloads:**
```go
type YourPayload struct {
    Field1 string `json:"field1"`
    Field2 int    `json:"field2"`
}
```

2. **Create your handler:**
```go
func HandleYourJob(ctx context.Context, j *job.Job) error {
    var data YourPayload
    if err := json.Unmarshal(j.Payload, &data); err != nil {
        return fmt.Errorf("invalid payload: %w", err)
    }

    // Your job logic here
    log.Printf("Processing: %s", data.Field1)

    return nil
}
```

3. **Register your handler:**
```go
registry.Register("your_job_name", HandleYourJob)
```

4. **Submit jobs:**
```go
jobID, err := client.SubmitJob("your_job_name", payload, job.PriorityNormal, "Description")
```

## Configuration

You can configure Bananas using environment variables:

```bash
# Redis connection
export REDIS_URL="redis://localhost:6379"

# Worker settings
export WORKER_CONCURRENCY=5    # Number of concurrent workers
export JOB_TIMEOUT=5m          # Maximum time per job

# Retry settings
export MAX_RETRIES=3           # Maximum retry attempts for failed jobs
```

## Troubleshooting

### Jobs Not Processing
- **Check Redis is running:** `redis-cli ping` should return `PONG`
- **Check worker logs:** Look for connection errors or handler panics
- **Verify handlers are registered:** Check the "Registered X handlers" message

### Jobs Stuck in Queue
- **Check worker status:** Are workers running and polling?
- **Check handler errors:** Look for error messages in worker logs
- **Verify job name:** Handler name must match the job name exactly

### High Latency
- **Check queue depth:** Use Redis CLI to inspect queue sizes
- **Add more workers:** Increase `WORKER_CONCURRENCY`
- **Check job duration:** Profile your handlers for performance

### Memory Issues
- **Reduce worker concurrency:** Lower `WORKER_CONCURRENCY`
- **Check for memory leaks:** Profile your handlers
- **Limit payload size:** Keep job payloads small (<1MB recommended)

## Next Steps

After running this example, you can:

1. **Explore the codebase:** See how workers, scheduler, and client SDK work
2. **Add your own handlers:** Define handlers for your specific use cases
3. **Run integration tests:** See `tests/integration_test.go` for more examples
4. **Deploy to production:** See `docs/DEPLOYMENT.md` (future) for production setup

## Additional Resources

- **Architecture Documentation:** See `docs/ARCHITECTURE.md` (future)
- **API Reference:** See `docs/API_REFERENCE.md` (future)
- **Performance Benchmarks:** See `docs/PERFORMANCE.md` (future)
- **Integration Guide:** See `docs/INTEGRATION.md`

## License

This example is part of the Bananas project and is provided as-is for demonstration purposes.

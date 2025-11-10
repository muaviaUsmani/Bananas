# Cron Scheduler Example

This example demonstrates how to use the Bananas cron scheduler to run periodic tasks.

## What This Example Does

The example registers several periodic schedules with different cron expressions:

1. **every-minute**: Runs a "ping" job every minute
2. **every-5-minutes**: Runs a "cleanup" job every 5 minutes
3. **hourly-report**: Generates a report at the top of every hour
4. **daily-summary**: Sends a daily summary email at 9 AM EST
5. **weekly-backup**: Performs a database backup every Monday at midnight
6. **monthly-invoice**: Generates invoices on the 1st of each month
7. **disabled-job**: Example of a disabled schedule (won't run)

## Prerequisites

1. Redis server running on `localhost:6379` (or set `REDIS_URL` environment variable)
2. Go 1.23 or later

## Running the Example

```bash
# Start from the project root
cd examples/cron_scheduler

# Run the example
go run main.go

# With custom Redis URL
REDIS_URL=redis://localhost:6379 go run main.go
```

## Expected Output

```
Connected to Redis
Registered 7 schedules
Schedule: every-minute | Cron: * * * * * | Job: ping | Next Run: 2025-11-10T03:01:00Z | Enabled: true
Schedule: every-5-minutes | Cron: */5 * * * * | Job: cleanup | Next Run: 2025-11-10T03:05:00Z | Enabled: true
Schedule: hourly-report | Cron: 0 * * * * | Job: generate_report | Next Run: 2025-11-10T04:00:00Z | Enabled: true
Schedule: daily-summary | Cron: 0 9 * * * | Job: send_summary_email | Next Run: 2025-11-10T14:00:00Z | Enabled: true
Schedule: weekly-backup | Cron: 0 0 * * 1 | Job: database_backup | Next Run: 2025-11-11T00:00:00Z | Enabled: true
Schedule: monthly-invoice | Cron: 0 0 1 * * | Job: generate_invoices | Next Run: 2025-12-01T00:00:00Z | Enabled: true
Schedule: disabled-job | Cron: * * * * * | Job: test_job | Next Run: 2025-11-10T03:01:00Z | Enabled: false
Cron scheduler started. Press Ctrl+C to stop.
```

## How It Works

### 1. Schedule Registration

Schedules are registered with the `Registry` using `MustRegister()`:

```go
registry.MustRegister(&scheduler.Schedule{
    ID:          "my-schedule",      // Unique identifier
    Cron:        "0 * * * *",         // Cron expression
    Job:         "my_job_name",       // Job handler name
    Payload:     []byte(`{...}`),     // JSON payload
    Priority:    job.PriorityNormal,  // high/normal/low
    Timezone:    "UTC",               // IANA timezone
    Enabled:     true,                // Enable/disable
    Description: "Job description",   // Human-readable description
})
```

### 2. Cron Expression Format

Uses standard 5-field cron syntax:

```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6) (Sunday to Saturday)
│ │ │ │ │
* * * * *
```

**Common Examples:**
- `* * * * *` - Every minute
- `*/5 * * * *` - Every 5 minutes
- `0 * * * *` - Every hour at minute 0
- `0 9 * * *` - Daily at 9:00 AM
- `0 9 * * 1` - Every Monday at 9:00 AM
- `0 0 1 * *` - First day of every month at midnight
- `0 9 * * 1-5` - Weekdays at 9:00 AM
- `0 0,12 * * *` - Daily at midnight and noon

### 3. Timezone Support

Each schedule can have its own timezone:

```go
Timezone: "America/New_York"  // EST/EDT
Timezone: "Europe/London"     // GMT/BST
Timezone: "Asia/Tokyo"        // JST
Timezone: "UTC"               // Default
```

### 4. Priority Levels

Jobs are enqueued with priorities:
- `job.PriorityHigh` - Processed first
- `job.PriorityNormal` - Default priority
- `job.PriorityLow` - Processed after normal/high

### 5. Distributed Execution

The scheduler uses Redis-based distributed locking to ensure that each schedule only runs once, even when multiple scheduler instances are running. This makes it safe to run multiple scheduler processes for high availability.

### 6. State Persistence

Schedule state is stored in Redis:
- `last_run` - Last execution time
- `next_run` - Next scheduled time
- `run_count` - Total number of executions
- `last_success` - Last successful execution
- `last_error` - Last error (if any)

### 7. Monitoring State

You can check schedule state from Redis:

```bash
redis-cli HGETALL bananas:schedules:every-minute
```

## Monitoring Jobs

Use the Bananas API or Redis to monitor enqueued jobs:

```bash
# Check pending jobs
redis-cli LRANGE bananas:queue:normal 0 -1

# Check scheduled job state
redis-cli HGETALL bananas:schedules:hourly-report
```

## Integration with Workers

The enqueued jobs will be processed by Bananas workers. Make sure you have:

1. Workers running with handlers for your job names:
   - `ping`
   - `cleanup`
   - `generate_report`
   - `send_summary_email`
   - `database_backup`
   - `generate_invoices`

2. Workers configured to process jobs from the appropriate priority queues.

See the worker examples for more details on implementing job handlers.

## Production Deployment

For production use:

1. **Enable in scheduler binary**: Uncomment the schedule registration in `cmd/scheduler/main.go`
2. **Set environment variables**:
   - `CRON_SCHEDULER_ENABLED=true` (default)
   - `CRON_SCHEDULER_INTERVAL=1s` (default)
   - `REDIS_URL=redis://your-redis-host:6379`
3. **High Availability**: Run multiple scheduler instances - distributed locking ensures each schedule runs only once
4. **Monitoring**: Monitor Redis keys `bananas:schedules:*` for execution state

## Disabling Schedules

To temporarily disable a schedule without removing it:

```go
schedule.Enabled = false
```

Or update the `enabled` field directly in your schedule definition.

## Advanced Usage

### Dynamic Schedule Updates

Schedules are loaded at startup. To update schedules dynamically, you would need to:
1. Store schedules in a database
2. Reload the registry periodically
3. Implement a schedule management API

This is left as an exercise for production systems.

### Custom Tick Interval

The scheduler checks for due schedules every second by default. You can adjust this:

```go
// Check every 5 seconds instead
cronScheduler := scheduler.NewCronScheduler(registry, redisQueue, redisClient, 5*time.Second)
```

**Note**: A shorter interval means more frequent checks but higher Redis load. 1 second is recommended for most use cases.

### Lock TTL

The default lock TTL is 60 seconds. For long-running job enqueue operations, you may need to adjust:

```go
cronScheduler.SetLockTTL(120 * time.Second)
```

## Troubleshooting

### Schedules Not Running

1. **Check schedule is enabled**: `Enabled: true`
2. **Verify cron expression**: Use a cron expression tester
3. **Check Redis connection**: Ensure Redis is accessible
4. **Verify timezone**: Ensure timezone string is valid IANA timezone
5. **Check logs**: Look for errors in scheduler logs

### Jobs Enqueued Multiple Times

This should not happen due to distributed locking. If it does:
1. Check Redis connectivity
2. Verify multiple schedulers aren't bypassing locks
3. Check for time synchronization issues between servers

### Schedule State Not Updating

1. Check Redis connection
2. Verify Redis permissions (write access required)
3. Check for Redis errors in logs

## Next Steps

- Implement job handlers in workers (see worker examples)
- Set up monitoring and alerting for schedule execution
- Create a schedule management UI (optional)
- Implement schedule persistence in database (for dynamic schedules)

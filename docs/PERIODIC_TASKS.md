# Periodic Tasks (Cron Scheduler)

**Last Updated:** 2025-11-11
**Status:** Complete

Bananas includes a cron-style periodic task scheduler for running jobs on a regular schedule. This is similar to Celery Beat in Python but designed for Go applications.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Scheduling Jobs](#scheduling-jobs)
- [Cron Expressions](#cron-expressions)
- [Timezone Support](#timezone-support)
- [Distributed Execution](#distributed-execution)
- [Monitoring](#monitoring)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Overview

The periodic task scheduler allows you to:

- **Schedule jobs using cron expressions** - Standard 5-field cron syntax
- **Timezone-aware scheduling** - Each schedule can have its own timezone
- **Distributed execution** - Safe to run multiple scheduler instances
- **Priority support** - Schedule jobs with high/normal/low priority
- **State persistence** - Track execution history in Redis
- **Enable/disable schedules** - Turn schedules on/off without deleting them
- **Task routing integration** - Scheduled jobs support routing keys

### Architecture

The scheduler consists of three main components:

1. **Registry** - Stores and validates schedule definitions
2. **CronScheduler** - Checks schedules and enqueues jobs when due
3. **DistributedLock** - Ensures single execution across multiple instances

Jobs are enqueued to the same Redis-backed queue system used for regular jobs (including routing-aware queues), so they're processed by your existing workers.

```
┌──────────────────────────────────────────────────────┐
│               Application Code                        │
│  scheduler.Register("cleanup", Schedule{             │
│    Cron: "0 * * * *",  // Every hour                 │
│    Job:  "cleanup_old_data",                         │
│    Payload: []byte(`{"max_age_days": 30}`),          │
│  })                                                  │
└──────────────────────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────┐
│          Schedule Registry (In-Memory)                │
│  - Stores schedule definitions                       │
│  - Validates cron expressions                        │
│  - Calculates next run times                         │
└──────────────────────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────┐
│            Cron Scheduler Service                     │
│  - Every 1 second: check for due schedules           │
│  - Acquire distributed lock per schedule             │
│  - Enqueue job to routing-aware queue                │
└──────────────────────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────┐
│                Redis Queue System                     │
│  bananas:route:{routing_key}:queue:{priority}        │
└──────────────────────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────┐
│                   Worker Pool                         │
│  - Dequeue and execute jobs                          │
│  - Respects routing keys                             │
└──────────────────────────────────────────────────────┘
```

## Quick Start

### 1. Enable the Scheduler

The cron scheduler is enabled by default in the `scheduler` binary. To disable it:

```bash
export CRON_SCHEDULER_ENABLED=false
```

### 2. Register a Schedule

Edit `cmd/scheduler/main.go` to register your schedules:

```go
// In main() after creating the registry
registry.MustRegister(&scheduler.Schedule{
    ID:          "daily-report",
    Cron:        "0 0 * * *",           // Daily at midnight
    Job:         "generate_daily_report", // Job handler name
    Payload:     []byte(`{"type": "sales"}`),
    Priority:    job.PriorityNormal,
    Timezone:    "America/New_York",
    Enabled:     true,
    Description: "Generate daily sales report",
})
```

### 3. Implement the Job Handler

In your worker, implement the job handler:

```go
// In worker registration
jobHandlers := map[string]worker.JobHandler{
    "generate_daily_report": handleDailyReport,
    // ... other handlers
}

func handleDailyReport(ctx context.Context, j *job.Job) error {
    var payload struct {
        Type string `json:"type"`
    }
    if err := json.Unmarshal(j.Payload, &payload); err != nil {
        return err
    }

    // Generate the report
    log.Printf("Generating %s report...", payload.Type)
    // ... report generation logic ...

    return nil
}
```

### 4. Start the Scheduler

```bash
./scheduler
```

The scheduler will check for due schedules every second (configurable) and enqueue jobs as needed.

## Configuration

Configure the scheduler via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `CRON_SCHEDULER_ENABLED` | `true` | Enable/disable the cron scheduler |
| `CRON_SCHEDULER_INTERVAL` | `1s` | How often to check for due schedules |
| `REDIS_URL` | `redis://localhost:6379` | Redis connection URL |

Example:

```bash
export CRON_SCHEDULER_ENABLED=true
export CRON_SCHEDULER_INTERVAL=1s
export REDIS_URL=redis://prod-redis:6379
./scheduler
```

## Scheduling Jobs

### Schedule Structure

```go
type Schedule struct {
    ID          string          // Unique identifier (alphanumeric, _, -)
    Cron        string          // Cron expression (5-field format)
    Job         string          // Job handler name
    Payload     []byte          // JSON payload for the job
    Priority    job.JobPriority // high, normal, or low
    Timezone    string          // IANA timezone (e.g., "America/New_York")
    RoutingKey  string          // Optional: route to specific workers
    Enabled     bool            // Whether the schedule is active
    Description string          // Human-readable description
}
```

### Scheduled Jobs with Routing Keys

Periodic tasks support task routing for specialized worker pools:

```go
// Schedule GPU-intensive job for GPU workers
registry.MustRegister(&scheduler.Schedule{
    ID:         "nightly-model-training",
    Cron:       "0 2 * * *",
    Job:        "train_ml_model",
    RoutingKey: "gpu",  // Route to GPU workers
    Priority:   job.PriorityNormal,
    Enabled:    true,
})

// Schedule email job for email workers
registry.MustRegister(&scheduler.Schedule{
    ID:         "daily-digest",
    Cron:       "0 8 * * 1-5",  // Weekdays at 8 AM
    Job:        "send_daily_digest",
    RoutingKey: "email",  // Route to email workers
    Priority:   job.PriorityHigh,
    Enabled:    true,
})
```

### Registration Methods

**MustRegister** - Panics on error (use for startup registration):

```go
registry.MustRegister(&scheduler.Schedule{
    ID:   "my-schedule",
    Cron: "0 * * * *",
    Job:  "my_job",
    // ... other fields
})
```

**Register** - Returns error (use for dynamic registration):

```go
err := registry.Register(&scheduler.Schedule{
    ID:   "my-schedule",
    Cron: "0 * * * *",
    Job:  "my_job",
    // ... other fields
})
if err != nil {
    log.Printf("Failed to register schedule: %v", err)
}
```

### Validation

Schedules are validated on registration:

- **ID**: Must be alphanumeric with `_` or `-` only
- **Cron**: Must be valid 5-field cron expression
- **Job**: Cannot be empty
- **Timezone**: Must be valid IANA timezone
- **Priority**: Must be `high`, `normal`, or `low` (if specified)
- **RoutingKey**: Must be valid routing key format (if specified)

## Cron Expressions

### Format

Bananas uses standard 5-field cron syntax:

```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6) (Sunday to Saturday)
│ │ │ │ │
* * * * *
```

### Common Examples

| Expression | Description | Example Time |
|------------|-------------|--------------|
| `* * * * *` | Every minute | Every minute |
| `*/5 * * * *` | Every 5 minutes | 00:05, 00:10, 00:15, ... |
| `0 * * * *` | Every hour | 01:00, 02:00, 03:00, ... |
| `0 0 * * *` | Daily at midnight | 00:00 |
| `0 9 * * *` | Daily at 9 AM | 09:00 |
| `0 9 * * 1` | Every Monday at 9 AM | Monday 09:00 |
| `0 9 * * 1-5` | Weekdays at 9 AM | Mon-Fri 09:00 |
| `0 0,12 * * *` | Twice daily | 00:00, 12:00 |
| `0 0 1 * *` | First of month | 1st 00:00 |
| `0 0 1 1 *` | Yearly on Jan 1 | Jan 1 00:00 |

### Supported Syntax

- **Wildcards**: `*` matches any value
- **Ranges**: `1-5` matches 1, 2, 3, 4, 5
- **Steps**: `*/5` matches every 5th value
- **Lists**: `1,2,3` matches 1, 2, and 3
- **Combinations**: `1-5,10,*/15` combines patterns

### Testing Cron Expressions

Use online tools like [crontab.guru](https://crontab.guru/) to test your expressions.

Note: Bananas uses 5-field format (minute-based), not 6-field (second-based).

## Timezone Support

### Setting Timezone

Each schedule can have its own timezone:

```go
registry.MustRegister(&scheduler.Schedule{
    ID:       "ny-morning-job",
    Cron:     "0 9 * * *",
    Job:      "morning_routine",
    Timezone: "America/New_York", // EST/EDT
    // ...
})

registry.MustRegister(&scheduler.Schedule{
    ID:       "london-evening-job",
    Cron:     "0 18 * * *",
    Job:      "evening_routine",
    Timezone: "Europe/London", // GMT/BST
    // ...
})
```

### Timezone List

Use IANA timezone names:

- **US**: `America/New_York`, `America/Chicago`, `America/Denver`, `America/Los_Angeles`
- **Europe**: `Europe/London`, `Europe/Paris`, `Europe/Berlin`
- **Asia**: `Asia/Tokyo`, `Asia/Shanghai`, `Asia/Dubai`
- **UTC**: `UTC` (default if not specified)

Full list: [IANA Time Zone Database](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones)

### Daylight Saving Time

Timezones automatically handle DST transitions:

```go
// This will run at 9 AM local time year-round:
// - 9:00 AM EST (UTC-5) in winter
// - 9:00 AM EDT (UTC-4) in summer
Timezone: "America/New_York"
Cron:     "0 9 * * *"
```

## Distributed Execution

### How It Works

The scheduler uses Redis-based distributed locking to ensure each schedule runs exactly once, even with multiple scheduler instances:

1. When a schedule is due, the scheduler attempts to acquire a lock
2. Lock key: `bananas:schedule_lock:{schedule_id}`
3. Lock uses a unique UUID token for ownership verification
4. Lock TTL: 60 seconds (prevents deadlock if scheduler crashes)
5. Only the instance that acquires the lock enqueues the job
6. Other instances skip execution (lock already held)

### High Availability

Run multiple scheduler instances for redundancy:

```bash
# Instance 1
./scheduler

# Instance 2 (on different server)
./scheduler

# Instance 3 (on different server)
./scheduler
```

All instances connect to the same Redis. Only one will execute each schedule.

### Lock Safety

Locks are released immediately after job enqueueing. If a scheduler crashes while holding a lock, the lock will expire after 60 seconds, allowing another instance to take over.

## Monitoring

### Schedule State

Each schedule maintains state in Redis:

```bash
# View schedule state
redis-cli HGETALL bananas:schedules:daily-report
```

State fields:
- `last_run` - Last execution time (RFC3339)
- `next_run` - Next scheduled time (RFC3339)
- `run_count` - Total executions
- `last_success` - Last successful execution
- `last_error` - Last error message (if any)

Example output:

```
1) "last_run"
2) "2025-11-10T00:00:00Z"
3) "next_run"
4) "2025-11-11T00:00:00Z"
5) "run_count"
6) "42"
7) "last_success"
8) "2025-11-10T00:00:00Z"
```

### Checking Enqueued Jobs

Jobs are enqueued to routing-aware priority queues:

```bash
# Check default queues
redis-cli LRANGE bananas:route:default:queue:high 0 -1
redis-cli LRANGE bananas:route:default:queue:normal 0 -1
redis-cli LRANGE bananas:route:default:queue:low 0 -1

# Check routed queues (e.g., GPU)
redis-cli LRANGE bananas:route:gpu:queue:high 0 -1
redis-cli LRANGE bananas:route:gpu:queue:normal 0 -1
```

### Logs

The scheduler logs execution events:

```
INFO Cron scheduler started interval=1s schedules=5
INFO Scheduled job enqueued schedule_id=daily-report job_name=generate_report job_id=abc123 priority=normal routing_key=default
ERROR Failed to enqueue scheduled job schedule_id=backup job_name=backup error="queue full"
```

Use `LOG_LEVEL=debug` for verbose output.

### Metrics

Monitor these Redis keys:

- `bananas:schedules:*` - Schedule states
- `bananas:schedule_lock:*` - Active locks
- Queue lengths by priority and routing key

## Best Practices

### 1. Use Descriptive IDs

```go
// Good
ID: "daily-sales-report"
ID: "hourly-metrics-sync"

// Bad
ID: "job1"
ID: "task_a"
```

### 2. Add Descriptions

```go
Description: "Generate daily sales report and email to management"
```

### 3. Choose Appropriate Priorities

- **High**: Time-critical jobs (monitoring, alerts)
- **Normal**: Regular jobs (reports, syncs)
- **Low**: Background tasks (cleanup, archival)

### 4. Use Task Routing for Resource-Specific Jobs

```go
// GPU-intensive periodic jobs
RoutingKey: "gpu"

// Email-intensive periodic jobs
RoutingKey: "email"

// Region-specific periodic jobs
RoutingKey: "us-east-1"
```

### 5. Set Realistic Intervals

Don't schedule jobs more frequently than they can complete:

```go
// If a job takes 2 minutes to run, don't schedule it every minute
Cron: "*/5 * * * *"  // Every 5 minutes - allows buffer
```

### 6. Use Timezone-Aware Schedules

For user-facing jobs, use the user's timezone:

```go
Timezone: "America/New_York"  // Not UTC
```

### 7. Enable/Disable vs Delete

Use `Enabled: false` to temporarily disable schedules:

```go
// Temporarily disabled for maintenance
Enabled: false
```

This preserves the schedule definition and execution history.

### 8. Test Cron Expressions

Verify your cron expressions using the `NextRun` method:

```go
schedule := &scheduler.Schedule{
    ID:   "test",
    Cron: "0 9 * * 1",
}
nextRun, err := registry.NextRun(schedule, time.Now())
fmt.Printf("Next run: %s\n", nextRun.Format(time.RFC3339))
```

### 9. Monitor for Failures

Check `last_error` field in schedule state:

```go
state, err := cronScheduler.GetState(ctx, "daily-report")
if state.LastError != "" {
    // Alert on error
    log.Printf("Schedule %s failed: %s", scheduleID, state.LastError)
}
```

### 10. Plan for Scale

If you have many schedules:
- Use appropriate `CRON_SCHEDULER_INTERVAL` (1s is fine for <1000 schedules)
- Monitor Redis performance
- Consider sharding schedules across multiple Redis instances

### 11. Document Your Schedules

Keep a registry of all schedules with business purpose:

```go
// Production schedules:
// - daily-report: Sales report for management (9 AM EST daily)
// - hourly-sync: Sync inventory with warehouse (top of hour)
// - weekly-backup: Full database backup (Monday midnight)
```

## Troubleshooting

### Schedule Not Running

**Check if schedule is enabled:**

```bash
redis-cli HGET bananas:schedules:my-schedule enabled
```

**Verify cron expression:**

```go
nextRun, err := registry.NextRun(schedule, time.Now())
if err != nil {
    log.Printf("Invalid cron: %v", err)
}
```

**Check scheduler logs:**

```bash
grep "my-schedule" scheduler.log
```

### Jobs Enqueued Multiple Times

This should not happen with distributed locking. If it does:

1. **Check Redis connectivity** - Ensure all schedulers connect to same Redis
2. **Verify time sync** - Ensure server clocks are synchronized (use NTP)
3. **Check lock TTL** - Default 60s should be sufficient

### Schedule State Not Updating

**Check Redis permissions:**

```bash
redis-cli SET test value
redis-cli GET test
redis-cli DEL test
```

**Check for Redis errors in logs:**

```
ERROR Failed to update state schedule_id=my-schedule error="connection refused"
```

### Wrong Timezone

**Verify timezone name:**

```go
_, err := time.LoadLocation("America/New_York")
if err != nil {
    log.Printf("Invalid timezone: %v", err)
}
```

**Check server timezone data:**

```bash
ls /usr/share/zoneinfo/America/
```

### High Redis Load

If Redis is overloaded:

1. **Increase interval**: `CRON_SCHEDULER_INTERVAL=5s`
2. **Reduce schedule count**: Consolidate similar schedules
3. **Scale Redis**: Use Redis Cluster or separate instance

### Lock Contention

If seeing many "already locked" debug messages:

1. **Normal behavior** when running multiple schedulers
2. **Check interval**: Ensure interval isn't too short
3. **Verify schedule timing**: Schedules running too frequently may queue up

---

## Related Documentation

- **[PERIODIC_TASKS_DESIGN.md](PERIODIC_TASKS_DESIGN.md)** - Architecture and design details
- **[Integration Guide](../INTEGRATION_GUIDE.md)** - Integration patterns and examples
- **[Task Routing](TASK_ROUTING_USAGE.md)** - Route scheduled jobs to specific workers
- **[Crontab Guru](https://crontab.guru/)** - Test cron expressions online
- **[IANA Time Zones](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones)** - Timezone reference

---

**Next:** See [Task Routing Usage Guide](TASK_ROUTING_USAGE.md) for routing periodic jobs to specialized workers.

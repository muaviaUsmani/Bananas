# Periodic Tasks - Design Document

## Overview

This document describes the design and implementation of periodic (cron-style) task scheduling for Bananas, providing a Celery Beat equivalent for Go.

## Goals

- **Cron-like Scheduling**: Support standard cron syntax for recurring tasks
- **Code-based Registration**: Define schedules in application code
- **Distributed Execution**: Only one instance executes each scheduled task (via distributed locking)
- **Persistent Storage**: Schedules survive restarts
- **Timezone Support**: Handle timezone-aware scheduling
- **Production Ready**: Reliable, observable, and well-tested

## Non-Goals

- **Web UI for schedule management** (future work)
- **Dynamic schedule modification at runtime** (schedules are code-defined)
- **Database-backed storage** (Redis-only for now)

---

## Architecture

### High-Level Design

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
│  - Enqueue job if lock acquired                      │
│  - Update last run time in Redis                     │
└──────────────────────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────┐
│              Redis (Persistent State)                 │
│  - schedules:{id} → last_run, next_run               │
│  - schedule_lock:{id} → lock token (TTL 60s)         │
└──────────────────────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────┐
│              Job Queue (Existing)                     │
│  - Scheduled job enqueued with priority              │
│  - Workers execute job normally                      │
└──────────────────────────────────────────────────────┘
```

---

## Components

### 1. Schedule Definition

**Location**: `internal/scheduler/schedule.go`

```go
package scheduler

import (
    "time"
    "github.com/muaviaUsmani/bananas/internal/job"
)

// Schedule represents a periodic task schedule
type Schedule struct {
    // ID is a unique identifier for the schedule
    ID string

    // Cron expression (standard 5-field: minute hour day month weekday)
    // Examples:
    //   "0 * * * *"     - Every hour at minute 0
    //   "*/15 * * * *"  - Every 15 minutes
    //   "0 9 * * 1"     - Every Monday at 9:00 AM
    //   "0 0 1 * *"     - First day of every month at midnight
    Cron string

    // Job name (must be registered with worker)
    Job string

    // Job payload (JSON bytes)
    Payload []byte

    // Priority for the enqueued job
    Priority job.JobPriority

    // Timezone for cron evaluation (default: UTC)
    Timezone string

    // Enabled flag (allows disabling without removing)
    Enabled bool

    // Metadata for logging/monitoring
    Description string
}

// ScheduleState represents the runtime state of a schedule
type ScheduleState struct {
    ID          string
    LastRun     time.Time
    NextRun     time.Time
    RunCount    int64
    LastError   string
    LastSuccess time.Time
}
```

**Validation Rules:**
- `ID` must be unique, non-empty, alphanumeric + underscores/hyphens
- `Cron` must be valid cron expression (validated using robfig/cron library)
- `Job` must be non-empty
- `Timezone` must be valid IANA timezone (e.g., "America/New_York", "UTC")

---

### 2. Schedule Registry

**Location**: `internal/scheduler/registry.go`

```go
package scheduler

import (
    "fmt"
    "sync"
    "github.com/robfig/cron/v3"
)

// Registry stores and manages periodic schedules
type Registry struct {
    mu        sync.RWMutex
    schedules map[string]*Schedule
    parser    cron.Parser
}

// NewRegistry creates a new schedule registry
func NewRegistry() *Registry {
    return &Registry{
        schedules: make(map[string]*Schedule),
        parser:    cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
    }
}

// Register adds a schedule to the registry
func (r *Registry) Register(schedule *Schedule) error {
    // Validate schedule
    if err := r.validate(schedule); err != nil {
        return fmt.Errorf("invalid schedule: %w", err)
    }

    r.mu.Lock()
    defer r.mu.Unlock()

    // Check for duplicate ID
    if _, exists := r.schedules[schedule.ID]; exists {
        return fmt.Errorf("schedule with ID %s already exists", schedule.ID)
    }

    r.schedules[schedule.ID] = schedule
    return nil
}

// MustRegister registers a schedule, panicking on error
// Useful for initialization-time schedule registration
func (r *Registry) MustRegister(schedule *Schedule) {
    if err := r.Register(schedule); err != nil {
        panic(fmt.Sprintf("failed to register schedule: %v", err))
    }
}

// Get retrieves a schedule by ID
func (r *Registry) Get(id string) (*Schedule, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    s, exists := r.schedules[id]
    return s, exists
}

// List returns all registered schedules
func (r *Registry) List() []*Schedule {
    r.mu.RLock()
    defer r.mu.RUnlock()

    schedules := make([]*Schedule, 0, len(r.schedules))
    for _, s := range r.schedules {
        schedules = append(schedules, s)
    }
    return schedules
}

// NextRun calculates the next run time for a schedule
func (r *Registry) NextRun(schedule *Schedule, after time.Time) (time.Time, error) {
    cronSchedule, err := r.parser.Parse(schedule.Cron)
    if err != nil {
        return time.Time{}, err
    }

    // Load timezone
    loc := time.UTC
    if schedule.Timezone != "" && schedule.Timezone != "UTC" {
        loc, err = time.LoadLocation(schedule.Timezone)
        if err != nil {
            return time.Time{}, fmt.Errorf("invalid timezone %s: %w", schedule.Timezone, err)
        }
    }

    // Calculate next run in the schedule's timezone
    afterInTz := after.In(loc)
    next := cronSchedule.Next(afterInTz)
    return next, nil
}
```

---

### 3. Distributed Lock

**Location**: `internal/scheduler/lock.go`

```go
package scheduler

import (
    "context"
    "fmt"
    "time"
    "github.com/google/uuid"
    "github.com/redis/go-redis/v9"
)

// DistributedLock provides Redis-based distributed locking
type DistributedLock struct {
    client *redis.Client
    key    string
    token  string
    ttl    time.Duration
}

// AcquireLock attempts to acquire a distributed lock
// Returns lock if successful, nil if already locked
func AcquireLock(ctx context.Context, client *redis.Client, key string, ttl time.Duration) (*DistributedLock, error) {
    token := uuid.New().String()

    // SETNX: Set if not exists
    acquired, err := client.SetNX(ctx, key, token, ttl).Result()
    if err != nil {
        return nil, fmt.Errorf("failed to acquire lock: %w", err)
    }

    if !acquired {
        return nil, nil // Lock already held by another instance
    }

    return &DistributedLock{
        client: client,
        key:    key,
        token:  token,
        ttl:    ttl,
    }, nil
}

// Release releases the lock (only if we still own it)
func (l *DistributedLock) Release(ctx context.Context) error {
    // Use Lua script to ensure we only delete our own lock
    script := `
        if redis.call("get", KEYS[1]) == ARGV[1] then
            return redis.call("del", KEYS[1])
        else
            return 0
        end
    `

    _, err := l.client.Eval(ctx, script, []string{l.key}, l.token).Result()
    return err
}

// Extend extends the lock TTL (for long-running operations)
func (l *DistributedLock) Extend(ctx context.Context, ttl time.Duration) error {
    // Use Lua script to extend only if we still own the lock
    script := `
        if redis.call("get", KEYS[1]) == ARGV[1] then
            return redis.call("pexpire", KEYS[1], ARGV[2])
        else
            return 0
        end
    `

    _, err := l.client.Eval(ctx, script, []string{l.key}, l.token, ttl.Milliseconds()).Result()
    return err
}
```

---

### 4. Cron Scheduler Service

**Location**: `internal/scheduler/cron_scheduler.go`

```go
package scheduler

import (
    "context"
    "fmt"
    "time"

    "github.com/muaviaUsmani/bananas/internal/job"
    "github.com/muaviaUsmani/bananas/internal/logger"
    "github.com/muaviaUsmani/bananas/internal/queue"
    "github.com/redis/go-redis/v9"
)

// CronScheduler manages periodic task execution
type CronScheduler struct {
    registry *Registry
    queue    queue.Queue
    client   *redis.Client
    interval time.Duration
    log      *logger.Logger
}

// NewCronScheduler creates a new cron scheduler
func NewCronScheduler(registry *Registry, queue queue.Queue, client *redis.Client, interval time.Duration) *CronScheduler {
    return &CronScheduler{
        registry: registry,
        queue:    queue,
        client:   client,
        interval: interval,
        log:      logger.Default().WithComponent(logger.ComponentScheduler),
    }
}

// Start begins the cron scheduler loop
func (cs *CronScheduler) Start(ctx context.Context) {
    cs.log.Info("Cron scheduler started", "interval", cs.interval)

    ticker := time.NewTicker(cs.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            cs.log.Info("Cron scheduler stopping")
            return
        case <-ticker.C:
            cs.tick(ctx)
        }
    }
}

// tick checks all schedules and enqueues due jobs
func (cs *CronScheduler) tick(ctx context.Context) {
    now := time.Now()
    schedules := cs.registry.List()

    for _, schedule := range schedules {
        if !schedule.Enabled {
            continue
        }

        // Check if schedule is due
        if cs.isDue(ctx, schedule, now) {
            cs.executeSchedule(ctx, schedule, now)
        }
    }
}

// isDue checks if a schedule should run now
func (cs *CronScheduler) isDue(ctx context.Context, schedule *Schedule, now time.Time) bool {
    // Get last run time from Redis
    state, err := cs.getState(ctx, schedule.ID)
    if err != nil {
        cs.log.Error("Failed to get schedule state", "schedule_id", schedule.ID, "error", err)
        return false
    }

    // Calculate next run time
    nextRun, err := cs.registry.NextRun(schedule, state.LastRun)
    if err != nil {
        cs.log.Error("Failed to calculate next run", "schedule_id", schedule.ID, "error", err)
        return false
    }

    // Due if next run time is in the past
    return now.After(nextRun) || now.Equal(nextRun)
}

// executeSchedule attempts to execute a schedule
func (cs *CronScheduler) executeSchedule(ctx context.Context, schedule *Schedule, now time.Time) {
    lockKey := fmt.Sprintf("bananas:schedule_lock:%s", schedule.ID)

    // Try to acquire distributed lock
    lock, err := AcquireLock(ctx, cs.client, lockKey, 60*time.Second)
    if err != nil {
        cs.log.Error("Failed to acquire schedule lock", "schedule_id", schedule.ID, "error", err)
        return
    }

    if lock == nil {
        // Another instance is already running this schedule
        cs.log.Debug("Schedule already locked by another instance", "schedule_id", schedule.ID)
        return
    }

    defer lock.Release(ctx)

    // Create and enqueue job
    j := &job.Job{
        Name:     schedule.Job,
        Payload:  schedule.Payload,
        Priority: schedule.Priority,
        Metadata: map[string]string{
            "schedule_id":   schedule.ID,
            "scheduled_at":  now.Format(time.RFC3339),
            "trigger_type":  "cron",
        },
    }

    if err := cs.queue.Enqueue(ctx, j); err != nil {
        cs.log.Error("Failed to enqueue scheduled job",
            "schedule_id", schedule.ID,
            "job_name", schedule.Job,
            "error", err)
        cs.updateState(ctx, schedule.ID, &ScheduleState{
            ID:        schedule.ID,
            LastRun:   now,
            LastError: err.Error(),
        })
        return
    }

    cs.log.Info("Scheduled job enqueued",
        "schedule_id", schedule.ID,
        "job_name", schedule.Job,
        "job_id", j.ID,
        "priority", schedule.Priority)

    // Update state
    nextRun, _ := cs.registry.NextRun(schedule, now)
    cs.updateState(ctx, schedule.ID, &ScheduleState{
        ID:          schedule.ID,
        LastRun:     now,
        NextRun:     nextRun,
        LastSuccess: now,
        RunCount:    cs.incrementRunCount(ctx, schedule.ID),
    })
}

// getState retrieves the current state of a schedule from Redis
func (cs *CronScheduler) getState(ctx context.Context, scheduleID string) (*ScheduleState, error) {
    key := fmt.Sprintf("bananas:schedules:%s", scheduleID)

    result, err := cs.client.HGetAll(ctx, key).Result()
    if err != nil {
        return nil, err
    }

    // Return default state if not found
    if len(result) == 0 {
        return &ScheduleState{
            ID:      scheduleID,
            LastRun: time.Time{}, // Zero time = never run
        }, nil
    }

    // Parse state from Redis hash
    state := &ScheduleState{ID: scheduleID}

    if lastRun, exists := result["last_run"]; exists {
        state.LastRun, _ = time.Parse(time.RFC3339, lastRun)
    }
    if nextRun, exists := result["next_run"]; exists {
        state.NextRun, _ = time.Parse(time.RFC3339, nextRun)
    }
    if lastSuccess, exists := result["last_success"]; exists {
        state.LastSuccess, _ = time.Parse(time.RFC3339, lastSuccess)
    }

    return state, nil
}

// updateState updates the schedule state in Redis
func (cs *CronScheduler) updateState(ctx context.Context, scheduleID string, state *ScheduleState) error {
    key := fmt.Sprintf("bananas:schedules:%s", scheduleID)

    fields := map[string]interface{}{
        "last_run": state.LastRun.Format(time.RFC3339),
    }

    if !state.NextRun.IsZero() {
        fields["next_run"] = state.NextRun.Format(time.RFC3339)
    }
    if !state.LastSuccess.IsZero() {
        fields["last_success"] = state.LastSuccess.Format(time.RFC3339)
    }
    if state.LastError != "" {
        fields["last_error"] = state.LastError
    }

    return cs.client.HSet(ctx, key, fields).Err()
}

// incrementRunCount increments and returns the run count
func (cs *CronScheduler) incrementRunCount(ctx context.Context, scheduleID string) int64 {
    key := fmt.Sprintf("bananas:schedules:%s", scheduleID)
    count, _ := cs.client.HIncrBy(ctx, key, "run_count", 1).Result()
    return count
}
```

---

## Usage Examples

### Basic Usage

```go
package main

import (
    "github.com/muaviaUsmani/bananas/internal/job"
    "github.com/muaviaUsmani/bananas/internal/scheduler"
)

func main() {
    // Create registry
    registry := scheduler.NewRegistry()

    // Register schedules
    registry.MustRegister(&scheduler.Schedule{
        ID:          "cleanup_hourly",
        Cron:        "0 * * * *",  // Every hour
        Job:         "cleanup_old_data",
        Payload:     []byte(`{"max_age_days": 30}`),
        Priority:    job.PriorityNormal,
        Timezone:    "UTC",
        Enabled:     true,
        Description: "Clean up data older than 30 days",
    })

    registry.MustRegister(&scheduler.Schedule{
        ID:          "weekly_report",
        Cron:        "0 9 * * 1",  // Every Monday at 9 AM
        Job:         "generate_weekly_report",
        Payload:     []byte(`{}`),
        Priority:    job.PriorityHigh,
        Timezone:    "America/New_York",
        Enabled:     true,
        Description: "Generate weekly analytics report",
    })

    // Create and start cron scheduler
    cronScheduler := scheduler.NewCronScheduler(
        registry,
        queue,
        redisClient,
        1*time.Second,  // Check every second
    )

    go cronScheduler.Start(ctx)
}
```

### Advanced: Timezone-Aware Scheduling

```go
// Daily backup at 2 AM Eastern Time
registry.MustRegister(&scheduler.Schedule{
    ID:          "daily_backup",
    Cron:        "0 2 * * *",
    Job:         "backup_database",
    Priority:    job.PriorityHigh,
    Timezone:    "America/New_York",  // Respects DST
    Enabled:     true,
})
```

---

## Redis Data Model

### Schedule State

**Key**: `bananas:schedules:{schedule_id}`
**Type**: Hash

**Fields**:
- `last_run` - RFC3339 timestamp of last execution
- `next_run` - RFC3339 timestamp of next scheduled execution
- `run_count` - Total number of executions
- `last_success` - RFC3339 timestamp of last successful execution
- `last_error` - Last error message (if any)

**Example**:
```
HGETALL bananas:schedules:cleanup_hourly
1) "last_run"
2) "2025-11-10T15:00:00Z"
3) "next_run"
4) "2025-11-10T16:00:00Z"
5) "run_count"
6) "240"
7) "last_success"
8) "2025-11-10T15:00:05Z"
```

### Distributed Lock

**Key**: `bananas:schedule_lock:{schedule_id}`
**Type**: String (lock token)
**TTL**: 60 seconds

**Example**:
```
SET bananas:schedule_lock:cleanup_hourly "uuid-token-12345" EX 60 NX
```

---

## Cron Expression Syntax

Standard 5-field cron format:

```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6) (Sunday=0)
│ │ │ │ │
* * * * *
```

**Examples**:
- `0 * * * *` - Every hour at minute 0
- `*/15 * * * *` - Every 15 minutes
- `0 9 * * 1` - Every Monday at 9:00 AM
- `0 0 1 * *` - First day of every month at midnight
- `0 0 * * 0` - Every Sunday at midnight
- `30 2 * * 1-5` - 2:30 AM Monday through Friday

**Special Tokens**:
- `@hourly` = `0 * * * *`
- `@daily` = `0 0 * * *`
- `@weekly` = `0 0 * * 0`
- `@monthly` = `0 0 1 * *`
- `@yearly` = `0 0 1 1 *`

---

## Error Handling

### Lock Acquisition Failure
- **Scenario**: Redis connection error when acquiring lock
- **Behavior**: Log error, skip this tick, retry next tick
- **Logging**: ERROR level with schedule_id and error details

### Schedule Already Locked
- **Scenario**: Another instance holds the lock
- **Behavior**: Skip silently (expected in distributed setup)
- **Logging**: DEBUG level

### Job Enqueue Failure
- **Scenario**: Queue.Enqueue() returns error
- **Behavior**: Log error, update state with error, retry next tick
- **Logging**: ERROR level with schedule_id, job_name, error

### Invalid Cron Expression
- **Scenario**: Malformed cron expression during registration
- **Behavior**: Return validation error, panic if using MustRegister
- **Logging**: Not logged (validation happens at startup)

### Timezone Error
- **Scenario**: Invalid timezone string
- **Behavior**: Return error during NextRun calculation
- **Logging**: ERROR level, skip schedule

---

## Monitoring & Observability

### Metrics

```go
// Schedule execution metrics
scheduler_runs_total{schedule_id, status}          // Counter: total runs
scheduler_run_duration_seconds{schedule_id}        // Histogram: execution time
scheduler_last_run_timestamp{schedule_id}          // Gauge: last run time
scheduler_next_run_timestamp{schedule_id}          // Gauge: next run time
scheduler_lock_failures_total{schedule_id}         // Counter: lock acquisition failures
```

### Logs

```json
{
  "level": "INFO",
  "msg": "Scheduled job enqueued",
  "component": "scheduler",
  "schedule_id": "cleanup_hourly",
  "job_name": "cleanup_old_data",
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "priority": "normal",
  "next_run": "2025-11-10T16:00:00Z"
}
```

---

## Testing Strategy

### Unit Tests

1. **Schedule Validation**:
   - Valid/invalid cron expressions
   - Timezone validation
   - Required fields

2. **Next Run Calculation**:
   - Various cron patterns
   - Timezone conversions
   - Edge cases (month boundaries, DST transitions)

3. **Distributed Lock**:
   - Acquire/release
   - Lock expiration
   - Concurrent acquisition attempts

### Integration Tests

1. **Cron Scheduler**:
   - Schedule execution
   - State persistence
   - Multiple schedules
   - Distributed locking (multiple scheduler instances)

2. **End-to-End**:
   - Register schedule → Execute → Job in queue
   - Timezone-aware scheduling
   - Error recovery

### Benchmark Tests

1. Lock acquisition performance
2. State read/write performance
3. Tick performance with 100+ schedules

---

## Migration Path

### Phase 1: Core Implementation (Current)
- ✅ Schedule registration API
- ✅ Cron parsing with robfig/cron
- ✅ Distributed locking
- ✅ Basic cron scheduler

### Phase 2: Enhanced Features (Future)
- [ ] Dynamic schedule updates (without restart)
- [ ] Schedule pause/resume API
- [ ] Schedule execution history
- [ ] Web UI for schedule management
- [ ] Alerting on schedule failures

---

## Dependencies

### New Dependencies

```go
require (
    github.com/robfig/cron/v3 v3.0.1      // Cron expression parsing
    github.com/google/uuid v1.6.0         // Lock token generation
)
```

### Existing Dependencies
- `github.com/redis/go-redis/v9` - Redis client (already in use)
- Internal packages: job, queue, logger, metrics

---

## Security Considerations

1. **Schedule Poisoning**: Schedules are code-defined only (not user input)
2. **Lock Hijacking**: Locks use unique tokens, verified on release
3. **Redis Access**: Shared Redis instance, no additional security needed
4. **Timezone Injection**: Timezone strings validated via time.LoadLocation

---

## Performance Considerations

### Lock TTL
- **Default**: 60 seconds
- **Rationale**: Covers job enqueue time + buffer
- **Trade-off**: Longer TTL = slower recovery from crashed scheduler

### Tick Interval
- **Default**: 1 second
- **Rationale**: 1-second granularity is sufficient for cron (minute-level)
- **Scaling**: 100 schedules = 100 checks/second = negligible load

### Redis Overhead
- **Per Tick**: N × HGETALL + N × SETNX (where N = enabled schedules)
- **Per Execution**: 1 × HSET + 1 × HINCRBY + 1 × Enqueue
- **Estimated**: <10ms for 100 schedules

---

**Last Updated**: 2025-11-10
**Status**: Design Complete, Ready for Implementation

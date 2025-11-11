// Package scheduler provides cron-based job scheduling functionality.
package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/muaviaUsmani/bananas/internal/logger"
	"github.com/redis/go-redis/v9"
)

// Queue defines the interface for enqueueing jobs
type Queue interface {
	Enqueue(ctx context.Context, j *job.Job) error
}

// CronScheduler manages periodic task execution
type CronScheduler struct {
	registry *Registry
	queue    Queue
	client   *redis.Client
	interval time.Duration
	lockTTL  time.Duration
	log      logger.Logger
}

// NewCronScheduler creates a new cron scheduler
func NewCronScheduler(registry *Registry, queue Queue, client *redis.Client, interval time.Duration) *CronScheduler {
	return &CronScheduler{
		registry: registry,
		queue:    queue,
		client:   client,
		interval: interval,
		lockTTL:  60 * time.Second, // Default: 60s lock TTL
		log:      logger.Default().WithComponent(logger.ComponentScheduler),
	}
}

// SetLockTTL sets the distributed lock TTL (for testing or tuning)
func (cs *CronScheduler) SetLockTTL(ttl time.Duration) {
	cs.lockTTL = ttl
}

// Start begins the cron scheduler loop
func (cs *CronScheduler) Start(ctx context.Context) {
	cs.log.Info("Cron scheduler started",
		"interval", cs.interval,
		"schedules", cs.registry.Count())

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
		cs.log.Error("Failed to get schedule state",
			"schedule_id", schedule.ID,
			"error", err)
		return false
	}

	// Calculate next run time
	nextRun, err := cs.registry.NextRun(schedule, state.LastRun)
	if err != nil {
		cs.log.Error("Failed to calculate next run",
			"schedule_id", schedule.ID,
			"error", err)
		return false
	}

	// Due if next run time is in the past or equal to now
	// Use 1-second buffer to account for tick timing
	return now.After(nextRun.Add(-1*time.Second)) || now.Equal(nextRun)
}

// executeSchedule attempts to execute a schedule
func (cs *CronScheduler) executeSchedule(ctx context.Context, schedule *Schedule, now time.Time) {
	lockKey := fmt.Sprintf("bananas:schedule_lock:%s", schedule.ID)

	// Try to acquire distributed lock
	lock, err := AcquireLock(ctx, cs.client, lockKey, cs.lockTTL)
	if err != nil {
		cs.log.Error("Failed to acquire schedule lock",
			"schedule_id", schedule.ID,
			"error", err)
		return
	}

	if lock == nil {
		// Another instance is already running this schedule
		cs.log.Debug("Schedule already locked by another instance",
			"schedule_id", schedule.ID)
		return
	}

	defer func() {
		if err := lock.Release(ctx); err != nil {
			cs.log.Error("Failed to release schedule lock",
				"schedule_id", schedule.ID,
				"error", err)
		}
	}()

	// Create and enqueue job
	description := schedule.Description
	if description == "" {
		description = fmt.Sprintf("Scheduled job: %s (schedule: %s)", schedule.Job, schedule.ID)
	}

	j := &job.Job{
		Name:        schedule.Job,
		Payload:     schedule.Payload,
		Priority:    schedule.Priority,
		Description: description,
	}

	// Default to normal priority if not specified
	if j.Priority == "" {
		j.Priority = job.PriorityNormal
	}

	if err := cs.queue.Enqueue(ctx, j); err != nil {
		cs.log.Error("Failed to enqueue scheduled job",
			"schedule_id", schedule.ID,
			"job_name", schedule.Job,
			"error", err)

		// Update state with error - log if update fails but don't fail the operation
		if updateErr := cs.updateState(ctx, schedule.ID, &ScheduleState{
			ID:        schedule.ID,
			LastRun:   now,
			LastError: err.Error(),
		}); updateErr != nil {
			cs.log.Warn("Failed to update schedule state", "schedule_id", schedule.ID, "error", updateErr)
		}
		return
	}

	cs.log.Info("Scheduled job enqueued",
		"schedule_id", schedule.ID,
		"job_name", schedule.Job,
		"job_id", j.ID,
		"priority", schedule.Priority,
		"description", schedule.Description)

	// Calculate next run time
	nextRun, err := cs.registry.NextRun(schedule, now)
	if err != nil {
		cs.log.Error("Failed to calculate next run time",
			"schedule_id", schedule.ID,
			"error", err)
		nextRun = time.Time{} // Zero time
	}

	// Update state - log if update fails but don't fail the operation
	runCount := cs.incrementRunCount(ctx, schedule.ID)
	if updateErr := cs.updateState(ctx, schedule.ID, &ScheduleState{
		ID:          schedule.ID,
		LastRun:     now,
		NextRun:     nextRun,
		LastSuccess: now,
		RunCount:    runCount,
		LastError:   "", // Clear error on success
	}); updateErr != nil {
		cs.log.Warn("Failed to update schedule state", "schedule_id", schedule.ID, "error", updateErr)
	}

	cs.log.Debug("Schedule state updated",
		"schedule_id", schedule.ID,
		"next_run", nextRun.Format(time.RFC3339),
		"run_count", runCount)
}

// getState retrieves the current state of a schedule from Redis
func (cs *CronScheduler) getState(ctx context.Context, scheduleID string) (*ScheduleState, error) {
	key := fmt.Sprintf("bananas:schedules:%s", scheduleID)

	result, err := cs.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get schedule state: %w", err)
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

	if lastRun, exists := result["last_run"]; exists && lastRun != "" {
		parsed, err := time.Parse(time.RFC3339, lastRun)
		if err == nil {
			state.LastRun = parsed
		}
	}

	if nextRun, exists := result["next_run"]; exists && nextRun != "" {
		parsed, err := time.Parse(time.RFC3339, nextRun)
		if err == nil {
			state.NextRun = parsed
		}
	}

	if lastSuccess, exists := result["last_success"]; exists && lastSuccess != "" {
		parsed, err := time.Parse(time.RFC3339, lastSuccess)
		if err == nil {
			state.LastSuccess = parsed
		}
	}

	if lastError, exists := result["last_error"]; exists {
		state.LastError = lastError
	}

	if runCount, exists := result["run_count"]; exists && runCount != "" {
		var count int64
		if _, err := fmt.Sscanf(runCount, "%d", &count); err == nil {
			state.RunCount = count
		}
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
	} else {
		// Clear error field on success
		cs.client.HDel(ctx, key, "last_error")
	}

	return cs.client.HSet(ctx, key, fields).Err()
}

// incrementRunCount increments and returns the run count
func (cs *CronScheduler) incrementRunCount(ctx context.Context, scheduleID string) int64 {
	key := fmt.Sprintf("bananas:schedules:%s", scheduleID)
	count, err := cs.client.HIncrBy(ctx, key, "run_count", 1).Result()
	if err != nil {
		cs.log.Error("Failed to increment run count",
			"schedule_id", scheduleID,
			"error", err)
		return 0
	}
	return count
}

// GetState retrieves the current state of a schedule (public method for monitoring)
func (cs *CronScheduler) GetState(ctx context.Context, scheduleID string) (*ScheduleState, error) {
	return cs.getState(ctx, scheduleID)
}

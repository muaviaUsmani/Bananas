package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/redis/go-redis/v9"
)

// mockQueue for testing
type mockQueue struct {
	enqueued []*job.Job
	errors   map[string]error
}

func (mq *mockQueue) Enqueue(ctx context.Context, j *job.Job) error {
	if err, exists := mq.errors[j.Name]; exists {
		return err
	}
	mq.enqueued = append(mq.enqueued, j)
	return nil
}

func setupCronScheduler(t *testing.T) (*CronScheduler, *Registry, *mockQueue, *redis.Client, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	registry := NewRegistry()
	q := &mockQueue{
		enqueued: make([]*job.Job, 0),
		errors:   make(map[string]error),
	}

	scheduler := NewCronScheduler(registry, q, client, 100*time.Millisecond)
	scheduler.SetLockTTL(5 * time.Second)

	return scheduler, registry, q, client, mr
}

func TestNewCronScheduler(t *testing.T) {
	scheduler, _, _, client, mr := setupCronScheduler(t)
	defer mr.Close()
	defer client.Close()

	if scheduler == nil {
		t.Fatal("Expected non-nil scheduler")
	}

	if scheduler.interval != 100*time.Millisecond {
		t.Errorf("Interval mismatch: got %v, want 100ms", scheduler.interval)
	}

	if scheduler.lockTTL != 5*time.Second {
		t.Errorf("Lock TTL mismatch: got %v, want 5s", scheduler.lockTTL)
	}
}

func TestCronScheduler_ExecuteSchedule(t *testing.T) {
	scheduler, registry, q, client, mr := setupCronScheduler(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Register a schedule
	schedule := &Schedule{
		ID:       "test_schedule",
		Cron:     "* * * * *", // Every minute
		Job:      "test_job",
		Payload:  []byte(`{"key":"value"}`),
		Priority: job.PriorityHigh,
		Enabled:  true,
	}

	registry.MustRegister(schedule)

	// Execute the schedule
	now := time.Now()
	scheduler.executeSchedule(ctx, schedule, now)

	// Check job was enqueued
	if len(q.enqueued) != 1 {
		t.Fatalf("Expected 1 enqueued job, got %d", len(q.enqueued))
	}

	enqueuedJob := q.enqueued[0]
	if enqueuedJob.Name != "test_job" {
		t.Errorf("Job name mismatch: got %s, want test_job", enqueuedJob.Name)
	}

	if enqueuedJob.Priority != job.PriorityHigh {
		t.Errorf("Job priority mismatch: got %s, want high", enqueuedJob.Priority)
	}

	if string(enqueuedJob.Payload) != `{"key":"value"}` {
		t.Errorf("Job payload mismatch: got %s", enqueuedJob.Payload)
	}

	// Check description contains schedule reference
	if enqueuedJob.Description == "" {
		t.Error("Expected non-empty description")
	}

	// Check state was updated
	state, err := scheduler.GetState(ctx, "test_schedule")
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	if state.LastRun.IsZero() {
		t.Error("LastRun was not updated")
	}

	if state.LastSuccess.IsZero() {
		t.Error("LastSuccess was not updated")
	}

	if state.RunCount != 1 {
		t.Errorf("RunCount mismatch: got %d, want 1", state.RunCount)
	}

	if state.NextRun.IsZero() {
		t.Error("NextRun was not calculated")
	}
}

func TestCronScheduler_DefaultPriority(t *testing.T) {
	scheduler, registry, q, client, mr := setupCronScheduler(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Register schedule without priority
	schedule := &Schedule{
		ID:      "test_schedule",
		Cron:    "* * * * *",
		Job:     "test_job",
		Enabled: true,
	}

	registry.MustRegister(schedule)

	// Execute
	scheduler.executeSchedule(ctx, schedule, time.Now())

	// Should default to normal priority
	if len(q.enqueued) != 1 {
		t.Fatalf("Expected 1 enqueued job, got %d", len(q.enqueued))
	}

	if q.enqueued[0].Priority != job.PriorityNormal {
		t.Errorf("Expected default priority normal, got %s", q.enqueued[0].Priority)
	}
}

func TestCronScheduler_EnqueueError(t *testing.T) {
	scheduler, registry, q, client, mr := setupCronScheduler(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Set up queue to return error
	q.errors["failing_job"] = errors.New("queue full")

	schedule := &Schedule{
		ID:      "test_schedule",
		Cron:    "* * * * *",
		Job:     "failing_job",
		Enabled: true,
	}

	registry.MustRegister(schedule)

	// Execute
	scheduler.executeSchedule(ctx, schedule, time.Now())

	// Job should not be enqueued
	if len(q.enqueued) != 0 {
		t.Errorf("Expected 0 enqueued jobs (error), got %d", len(q.enqueued))
	}

	// State should have error
	state, err := scheduler.GetState(ctx, "test_schedule")
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	if state.LastError == "" {
		t.Error("Expected error in state, got empty string")
	}

	// LastSuccess should be zero
	if !state.LastSuccess.IsZero() {
		t.Error("Expected zero LastSuccess on error")
	}
}

func TestCronScheduler_DistributedLocking(t *testing.T) {
	// Create two schedulers sharing the same Redis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer client.Close()

	registry := NewRegistry()
	q1 := &mockQueue{enqueued: make([]*job.Job, 0)}
	q2 := &mockQueue{enqueued: make([]*job.Job, 0)}

	scheduler1 := NewCronScheduler(registry, q1, client, 100*time.Millisecond)
	scheduler2 := NewCronScheduler(registry, q2, client, 100*time.Millisecond)

	ctx := context.Background()

	schedule := &Schedule{
		ID:      "test_schedule",
		Cron:    "* * * * *",
		Job:     "test_job",
		Enabled: true,
	}

	registry.MustRegister(schedule)

	// Execute on both schedulers simultaneously
	done := make(chan bool, 2)

	go func() {
		scheduler1.executeSchedule(ctx, schedule, time.Now())
		done <- true
	}()

	go func() {
		scheduler2.executeSchedule(ctx, schedule, time.Now())
		done <- true
	}()

	// Wait for both to finish
	<-done
	<-done

	// Only one should have succeeded
	totalEnqueued := len(q1.enqueued) + len(q2.enqueued)
	if totalEnqueued != 1 {
		t.Errorf("Expected exactly 1 job enqueued (distributed lock), got %d", totalEnqueued)
	}
}

func TestCronScheduler_IsDue_NeverRun(t *testing.T) {
	scheduler, registry, _, client, mr := setupCronScheduler(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Schedule that runs every minute
	schedule := &Schedule{
		ID:      "test_schedule",
		Cron:    "* * * * *",
		Job:     "test_job",
		Enabled: true,
	}

	registry.MustRegister(schedule)

	// First check should be due (never run before)
	now := time.Now()
	isDue := scheduler.isDue(ctx, schedule, now)

	if !isDue {
		t.Error("Expected schedule to be due on first check")
	}
}

func TestCronScheduler_IsDue_RecentlyRun(t *testing.T) {
	scheduler, registry, _, client, mr := setupCronScheduler(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	schedule := &Schedule{
		ID:      "test_schedule",
		Cron:    "0 * * * *", // Every hour
		Job:     "test_job",
		Enabled: true,
	}

	registry.MustRegister(schedule)

	// Set last run to 30 minutes ago
	lastRun := time.Now().Add(-30 * time.Minute)
	client.HSet(ctx, "bananas:schedules:test_schedule", "last_run", lastRun.Format(time.RFC3339))

	// Should not be due yet
	now := time.Now()
	isDue := scheduler.isDue(ctx, schedule, now)

	if isDue {
		t.Error("Expected schedule not to be due (last run was 30 min ago, runs hourly)")
	}
}

func TestCronScheduler_IsDue_PastDue(t *testing.T) {
	scheduler, registry, _, client, mr := setupCronScheduler(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	schedule := &Schedule{
		ID:      "test_schedule",
		Cron:    "0 * * * *", // Every hour
		Job:     "test_job",
		Enabled: true,
	}

	registry.MustRegister(schedule)

	// Set last run to 2 hours ago
	lastRun := time.Now().Add(-2 * time.Hour)
	client.HSet(ctx, "bananas:schedules:test_schedule", "last_run", lastRun.Format(time.RFC3339))

	// Should be due now
	now := time.Now()
	isDue := scheduler.isDue(ctx, schedule, now)

	if !isDue {
		t.Error("Expected schedule to be due (last run was 2 hours ago)")
	}
}

func TestCronScheduler_Tick_DisabledSchedule(t *testing.T) {
	scheduler, registry, q, client, mr := setupCronScheduler(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	schedule := &Schedule{
		ID:      "test_schedule",
		Cron:    "* * * * *",
		Job:     "test_job",
		Enabled: false, // Disabled
	}

	registry.MustRegister(schedule)

	// Run tick
	scheduler.tick(ctx)

	// No job should be enqueued
	if len(q.enqueued) != 0 {
		t.Errorf("Expected 0 jobs for disabled schedule, got %d", len(q.enqueued))
	}
}

func TestCronScheduler_Tick_MultipleSchedules(t *testing.T) {
	scheduler, registry, q, client, mr := setupCronScheduler(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Register multiple schedules
	schedule1 := &Schedule{
		ID:      "schedule1",
		Cron:    "* * * * *",
		Job:     "job1",
		Enabled: true,
	}

	schedule2 := &Schedule{
		ID:      "schedule2",
		Cron:    "* * * * *",
		Job:     "job2",
		Enabled: true,
	}

	schedule3 := &Schedule{
		ID:      "schedule3",
		Cron:    "* * * * *",
		Job:     "job3",
		Enabled: false, // Disabled
	}

	registry.MustRegister(schedule1)
	registry.MustRegister(schedule2)
	registry.MustRegister(schedule3)

	// Run tick
	scheduler.tick(ctx)

	// Should enqueue 2 jobs (schedule3 is disabled)
	if len(q.enqueued) != 2 {
		t.Errorf("Expected 2 enqueued jobs, got %d", len(q.enqueued))
	}

	// Verify job names
	jobNames := make(map[string]bool)
	for _, j := range q.enqueued {
		jobNames[j.Name] = true
	}

	if !jobNames["job1"] || !jobNames["job2"] {
		t.Error("Expected job1 and job2 to be enqueued")
	}

	if jobNames["job3"] {
		t.Error("job3 should not be enqueued (disabled schedule)")
	}
}

func TestCronScheduler_StateUpdate_ClearsError(t *testing.T) {
	scheduler, registry, _, client, mr := setupCronScheduler(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	schedule := &Schedule{
		ID:      "test_schedule",
		Cron:    "* * * * *",
		Job:     "test_job",
		Enabled: true,
	}

	registry.MustRegister(schedule)

	// First, set an error in state
	scheduler.updateState(ctx, "test_schedule", &ScheduleState{
		ID:        "test_schedule",
		LastRun:   time.Now(),
		LastError: "previous error",
	})

	// Verify error exists
	state, _ := scheduler.GetState(ctx, "test_schedule")
	if state.LastError != "previous error" {
		t.Error("Expected error to be set")
	}

	// Now execute successfully
	scheduler.executeSchedule(ctx, schedule, time.Now())

	// Error should be cleared
	state, err := scheduler.GetState(ctx, "test_schedule")
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	if state.LastError != "" {
		t.Errorf("Expected error to be cleared, got %s", state.LastError)
	}
}

func TestCronScheduler_RunCount_Increment(t *testing.T) {
	scheduler, registry, _, client, mr := setupCronScheduler(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	schedule := &Schedule{
		ID:      "test_schedule",
		Cron:    "* * * * *",
		Job:     "test_job",
		Enabled: true,
	}

	registry.MustRegister(schedule)

	// Execute multiple times
	for i := 1; i <= 5; i++ {
		scheduler.executeSchedule(ctx, schedule, time.Now())

		state, err := scheduler.GetState(ctx, "test_schedule")
		if err != nil {
			t.Fatalf("Failed to get state: %v", err)
		}

		if state.RunCount != int64(i) {
			t.Errorf("Run %d: expected run_count %d, got %d", i, i, state.RunCount)
		}
	}
}

func TestCronScheduler_Start_Stop(t *testing.T) {
	scheduler, _, _, client, mr := setupCronScheduler(t)
	defer mr.Close()
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Start scheduler in background
	done := make(chan bool)
	go func() {
		scheduler.Start(ctx)
		done <- true
	}()

	// Let it run for a bit
	time.Sleep(300 * time.Millisecond)

	// Stop scheduler
	cancel()

	// Wait for it to finish
	select {
	case <-done:
		// Good, stopped cleanly
	case <-time.After(2 * time.Second):
		t.Error("Scheduler did not stop within timeout")
	}
}

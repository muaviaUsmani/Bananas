package queue

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/muaviaUsmani/bananas/internal/job"
)

func setupTestRedis(t *testing.T) (*RedisQueue, *miniredis.Miniredis) {
	// Create miniredis server
	mr := miniredis.RunT(t)

	// Create queue
	queue, err := NewRedisQueue("redis://" + mr.Addr())
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}

	return queue, mr
}

func TestNewRedisQueue_Success(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	queue, err := NewRedisQueue("redis://" + mr.Addr())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if queue == nil {
		t.Fatal("expected queue to be created")
	}
	defer queue.Close()

	if queue.keyPrefix != "bananas:" {
		t.Errorf("expected keyPrefix 'bananas:', got '%s'", queue.keyPrefix)
	}
}

func TestNewRedisQueue_InvalidURL(t *testing.T) {
	_, err := NewRedisQueue("invalid://url")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestNewRedisQueue_ConnectionFailure(t *testing.T) {
	_, err := NewRedisQueue("redis://localhost:9999")
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestEnqueue_Success(t *testing.T) {
	queue, mr := setupTestRedis(t)
	defer mr.Close()
	defer queue.Close()

	ctx := context.Background()
	j := job.NewJob("test_job", []byte(`{"test":"data"}`), job.PriorityNormal)

	err := queue.Enqueue(ctx, j)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify job was stored
	jobKey := queue.jobKey(j.ID)
	if !mr.Exists(jobKey) {
		t.Error("job data not stored in Redis")
	}

	// Verify job ID was added to queue
	queueKey := queue.queueKey(job.PriorityNormal)
	length, _ := queue.client.LLen(context.Background(), queueKey).Result()
	if length != 1 {
		t.Errorf("expected queue length 1, got %d", length)
	}
}

func TestDequeue_Success(t *testing.T) {
	queue, mr := setupTestRedis(t)
	defer mr.Close()
	defer queue.Close()

	ctx := context.Background()

	// Enqueue a job
	j := job.NewJob("test_job", []byte(`{"test":"data"}`), job.PriorityHigh)
	if err := queue.Enqueue(ctx, j); err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	// Dequeue the job
	priorities := []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow}
	dequeuedJob, err := queue.Dequeue(ctx, priorities)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dequeuedJob == nil {
		t.Fatal("expected job to be dequeued")
	}
	if dequeuedJob.ID != j.ID {
		t.Errorf("expected job ID %s, got %s", j.ID, dequeuedJob.ID)
	}

	// Verify job moved to processing queue
	processingKey := queue.processingQueueKey()
	length, _ := queue.client.LLen(context.Background(), processingKey).Result()
	if length != 1 {
		t.Errorf("expected processing queue length 1, got %d", length)
	}
}

func TestDequeue_EmptyQueue(t *testing.T) {
	queue, mr := setupTestRedis(t)
	defer mr.Close()
	defer queue.Close()

	ctx := context.Background()
	priorities := []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow}

	dequeuedJob, err := queue.Dequeue(ctx, priorities)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dequeuedJob != nil {
		t.Error("expected nil job from empty queue")
	}
}

func TestDequeue_PriorityOrdering(t *testing.T) {
	queue, mr := setupTestRedis(t)
	defer mr.Close()
	defer queue.Close()

	ctx := context.Background()

	// Enqueue jobs with different priorities
	lowJob := job.NewJob("low_job", []byte(`{}`), job.PriorityLow)
	normalJob := job.NewJob("normal_job", []byte(`{}`), job.PriorityNormal)
	highJob := job.NewJob("high_job", []byte(`{}`), job.PriorityHigh)

	queue.Enqueue(ctx, lowJob)
	queue.Enqueue(ctx, normalJob)
	queue.Enqueue(ctx, highJob)

	// Dequeue with priority order
	priorities := []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow}

	// Should get high priority first
	j1, _ := queue.Dequeue(ctx, priorities)
	if j1.ID != highJob.ID {
		t.Errorf("expected high priority job first, got %s", j1.Name)
	}

	// Should get normal priority second
	j2, _ := queue.Dequeue(ctx, priorities)
	if j2.ID != normalJob.ID {
		t.Errorf("expected normal priority job second, got %s", j2.Name)
	}

	// Should get low priority last
	j3, _ := queue.Dequeue(ctx, priorities)
	if j3.ID != lowJob.ID {
		t.Errorf("expected low priority job last, got %s", j3.Name)
	}
}

func TestComplete_Success(t *testing.T) {
	queue, mr := setupTestRedis(t)
	defer mr.Close()
	defer queue.Close()

	ctx := context.Background()

	// Enqueue and dequeue a job
	j := job.NewJob("test_job", []byte(`{}`), job.PriorityNormal)
	queue.Enqueue(ctx, j)

	priorities := []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow}
	dequeuedJob, _ := queue.Dequeue(ctx, priorities)

	// Complete the job
	err := queue.Complete(ctx, dequeuedJob.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify job removed from processing queue
	processingKey := queue.processingQueueKey()
	length, _ := queue.client.LLen(context.Background(), processingKey).Result()
	if length != 0 {
		t.Errorf("expected processing queue empty, got length %d", length)
	}

	// Verify job status updated
	completedJob, err := queue.GetJob(ctx, dequeuedJob.ID)
	if err != nil {
		t.Fatalf("failed to get job: %v", err)
	}
	if completedJob.Status != job.StatusCompleted {
		t.Errorf("expected status %s, got %s", job.StatusCompleted, completedJob.Status)
	}
}

func TestFail_WithRetry(t *testing.T) {
	queue, mr := setupTestRedis(t)
	defer mr.Close()
	defer queue.Close()

	ctx := context.Background()

	// Create job with max retries
	j := job.NewJob("test_job", []byte(`{}`), job.PriorityNormal)
	j.MaxRetries = 3
	queue.Enqueue(ctx, j)

	priorities := []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow}
	dequeuedJob, _ := queue.Dequeue(ctx, priorities)

	// Fail the job (should schedule for retry, not re-enqueue)
	err := queue.Fail(ctx, dequeuedJob, "test error")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify job NOT in priority queue (should be in scheduled set instead)
	queueKey := queue.queueKey(job.PriorityNormal)
	length, _ := queue.client.LLen(context.Background(), queueKey).Result()
	if length != 0 {
		t.Errorf("expected priority queue empty (job should be in scheduled set), got length %d", length)
	}

	// Verify job IS in scheduled set
	scheduledKey := queue.getScheduledSetKey()
	scheduledMembers, _ := queue.client.ZRange(context.Background(), scheduledKey, 0, -1).Result()
	if len(scheduledMembers) != 1 {
		t.Errorf("expected job in scheduled set, got %d members", len(scheduledMembers))
	}
	if len(scheduledMembers) > 0 && scheduledMembers[0] != j.ID {
		t.Errorf("expected job ID %s in scheduled set, got %s", j.ID, scheduledMembers[0])
	}

	// Verify job removed from processing queue
	processingKey := queue.processingQueueKey()
	processLength, _ := queue.client.LLen(context.Background(), processingKey).Result()
	if processLength != 0 {
		t.Errorf("expected processing queue empty, got length %d", processLength)
	}

	// Verify attempts incremented and ScheduledFor set
	retriedJob, _ := queue.GetJob(ctx, j.ID)
	if retriedJob.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", retriedJob.Attempts)
	}
	if retriedJob.Error != "test error" {
		t.Errorf("expected error message set, got '%s'", retriedJob.Error)
	}
	if retriedJob.ScheduledFor == nil {
		t.Error("expected ScheduledFor to be set")
	}
	if retriedJob.Status != job.StatusPending {
		t.Errorf("expected status %s, got %s", job.StatusPending, retriedJob.Status)
	}
}

func TestFail_MaxRetriesExceeded(t *testing.T) {
	queue, mr := setupTestRedis(t)
	defer mr.Close()
	defer queue.Close()

	ctx := context.Background()

	// Create job and set to max attempts
	j := job.NewJob("test_job", []byte(`{}`), job.PriorityNormal)
	j.MaxRetries = 3
	j.Attempts = 2 // One more attempt will hit max
	queue.Enqueue(ctx, j)

	priorities := []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow}
	dequeuedJob, _ := queue.Dequeue(ctx, priorities)

	// Fail the job (should go to dead letter queue)
	err := queue.Fail(ctx, dequeuedJob, "fatal error")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify job in dead letter queue
	deadLetterKey := queue.deadLetterQueueKey()
	length, _ := queue.client.LLen(context.Background(), deadLetterKey).Result()
	if length != 1 {
		t.Errorf("expected job in dead letter queue, got length %d", length)
	}

	// Verify job not in priority queue
	queueKey := queue.queueKey(job.PriorityNormal)
	queueLength, _ := queue.client.LLen(context.Background(), queueKey).Result()
	if queueLength != 0 {
		t.Errorf("expected priority queue empty, got length %d", queueLength)
	}

	// Verify job status is Failed
	failedJob, _ := queue.GetJob(ctx, j.ID)
	if failedJob.Status != job.StatusFailed {
		t.Errorf("expected status %s, got %s", job.StatusFailed, failedJob.Status)
	}
}

func TestGetJob_Success(t *testing.T) {
	queue, mr := setupTestRedis(t)
	defer mr.Close()
	defer queue.Close()

	ctx := context.Background()

	// Enqueue a job
	j := job.NewJob("test_job", []byte(`{"test":"data"}`), job.PriorityNormal, "Test job")
	queue.Enqueue(ctx, j)

	// Get the job
	retrievedJob, err := queue.GetJob(ctx, j.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if retrievedJob == nil {
		t.Fatal("expected job to be retrieved")
	}
	if retrievedJob.ID != j.ID {
		t.Errorf("expected job ID %s, got %s", j.ID, retrievedJob.ID)
	}
	if retrievedJob.Description != "Test job" {
		t.Errorf("expected description 'Test job', got '%s'", retrievedJob.Description)
	}
}

func TestGetJob_NotFound(t *testing.T) {
	queue, mr := setupTestRedis(t)
	defer mr.Close()
	defer queue.Close()

	ctx := context.Background()

	_, err := queue.GetJob(ctx, "non-existent-id")
	if err == nil {
		t.Fatal("expected error for non-existent job")
	}
}

func TestKeyGeneration(t *testing.T) {
	prefix := "bananas:"
	queue := &RedisQueue{
		keyPrefix:       prefix,
		queueHighKey:    prefix + "queue:high",
		queueNormalKey:  prefix + "queue:normal",
		queueLowKey:     prefix + "queue:low",
		processingKey:   prefix + "queue:processing",
		deadLetterKey:   prefix + "queue:dead",
		scheduledSetKey: prefix + "queue:scheduled",
	}

	tests := []struct {
		name     string
		method   func() string
		expected string
	}{
		{"jobKey", func() string { return queue.jobKey("123") }, "bananas:job:123"},
		{"queueKey high", func() string { return queue.queueKey(job.PriorityHigh) }, "bananas:queue:high"},
		{"queueKey normal", func() string { return queue.queueKey(job.PriorityNormal) }, "bananas:queue:normal"},
		{"queueKey low", func() string { return queue.queueKey(job.PriorityLow) }, "bananas:queue:low"},
		{"processingQueueKey", func() string { return queue.processingQueueKey() }, "bananas:queue:processing"},
		{"deadLetterQueueKey", func() string { return queue.deadLetterQueueKey() }, "bananas:queue:dead"},
		{"scheduledSetKey", func() string { return queue.getScheduledSetKey() }, "bananas:queue:scheduled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method()
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestEnqueueDequeue_MultipleJobs(t *testing.T) {
	queue, mr := setupTestRedis(t)
	defer mr.Close()
	defer queue.Close()

	ctx := context.Background()

	// Enqueue multiple jobs
	job1 := job.NewJob("job1", []byte(`{}`), job.PriorityNormal)
	job2 := job.NewJob("job2", []byte(`{}`), job.PriorityNormal)
	job3 := job.NewJob("job3", []byte(`{}`), job.PriorityNormal)

	queue.Enqueue(ctx, job1)
	queue.Enqueue(ctx, job2)
	queue.Enqueue(ctx, job3)

	// Dequeue all jobs
	priorities := []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow}

	retrieved := make(map[string]bool)
	for i := 0; i < 3; i++ {
		j, err := queue.Dequeue(ctx, priorities)
		if err != nil || j == nil {
			t.Fatalf("failed to dequeue job %d: %v", i, err)
		}
		retrieved[j.ID] = true
	}

	// Verify all jobs were retrieved
	if !retrieved[job1.ID] || !retrieved[job2.ID] || !retrieved[job3.ID] {
		t.Error("not all jobs were dequeued")
	}

	// Verify queue is now empty
	j, err := queue.Dequeue(ctx, priorities)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if j != nil {
		t.Error("expected empty queue")
	}
}

func TestClose(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	queue, err := NewRedisQueue("redis://" + mr.Addr())
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}

	err = queue.Close()
	if err != nil {
		t.Errorf("expected no error on close, got %v", err)
	}
}

func TestJobTimestampUpdates(t *testing.T) {
	queue, mr := setupTestRedis(t)
	defer mr.Close()
	defer queue.Close()

	ctx := context.Background()

	// Create and enqueue job
	j := job.NewJob("test_job", []byte(`{}`), job.PriorityNormal)
	initialTime := j.UpdatedAt
	queue.Enqueue(ctx, j)

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Retrieve and complete job
	priorities := []job.JobPriority{job.PriorityNormal}
	dequeuedJob, _ := queue.Dequeue(ctx, priorities)
	queue.Complete(ctx, dequeuedJob.ID)

	// Get completed job
	completedJob, _ := queue.GetJob(ctx, j.ID)

	// Verify timestamp was updated
	if !completedJob.UpdatedAt.After(initialTime) {
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestMoveScheduledToReady_Success(t *testing.T) {
	queue, mr := setupTestRedis(t)
	defer mr.Close()
	defer queue.Close()

	ctx := context.Background()

	// Create and enqueue job
	j := job.NewJob("test_job", []byte(`{}`), job.PriorityHigh)
	j.MaxRetries = 3
	queue.Enqueue(ctx, j)

	// Dequeue and fail it (should go to scheduled set)
	priorities := []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow}
	dequeuedJob, _ := queue.Dequeue(ctx, priorities)
	queue.Fail(ctx, dequeuedJob, "temporary error")

	// Verify job is in scheduled set
	scheduledKey := queue.getScheduledSetKey()
	scheduledMembers, _ := queue.client.ZRange(context.Background(), scheduledKey, 0, -1).Result()
	if len(scheduledMembers) != 1 {
		t.Fatal("expected job in scheduled set")
	}

	// Manually update the scheduled set score to be in the past
	// (miniredis FastForward doesn't work with sorted sets)
	pastTime := time.Now().Add(-10 * time.Second).Unix()
	queue.client.ZAdd(ctx, scheduledKey, redis.Z{
		Score:  float64(pastTime),
		Member: j.ID,
	})

	// Move scheduled jobs to ready
	count, err := queue.MoveScheduledToReady(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 job moved, got %d", count)
	}

	// Verify job moved to priority queue
	queueKey := queue.queueKey(job.PriorityHigh)
	queueLength, _ := queue.client.LLen(context.Background(), queueKey).Result()
	if queueLength != 1 {
		t.Errorf("expected job in priority queue, got length %d", queueLength)
	}

	// Verify job removed from scheduled set
	scheduledMembers2, _ := queue.client.ZRange(context.Background(), scheduledKey, 0, -1).Result()
	if len(scheduledMembers2) != 0 {
		t.Error("expected scheduled set to be empty")
	}

	// Verify ScheduledFor cleared
	movedJob, _ := queue.GetJob(ctx, j.ID)
	if movedJob.ScheduledFor != nil {
		t.Error("expected ScheduledFor to be cleared")
	}
}

func TestMoveScheduledToReady_NoJobs(t *testing.T) {
	queue, mr := setupTestRedis(t)
	defer mr.Close()
	defer queue.Close()

	ctx := context.Background()

	// Call with empty scheduled set
	count, err := queue.MoveScheduledToReady(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 jobs moved, got %d", count)
	}
}

func TestMoveScheduledToReady_FutureJobs(t *testing.T) {
	queue, mr := setupTestRedis(t)
	defer mr.Close()
	defer queue.Close()

	ctx := context.Background()

	// Create and fail job (will be scheduled for future)
	j := job.NewJob("test_job", []byte(`{}`), job.PriorityNormal)
	j.MaxRetries = 3
	queue.Enqueue(ctx, j)

	priorities := []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow}
	dequeuedJob, _ := queue.Dequeue(ctx, priorities)
	queue.Fail(ctx, dequeuedJob, "error")

	// Try to move scheduled jobs (but it's not time yet)
	count, err := queue.MoveScheduledToReady(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 jobs moved (not ready yet), got %d", count)
	}

	// Verify job still in scheduled set
	scheduledKey := queue.getScheduledSetKey()
	scheduledMembers, _ := queue.client.ZRange(context.Background(), scheduledKey, 0, -1).Result()
	if len(scheduledMembers) != 1 {
		t.Error("expected job still in scheduled set")
	}

	// Verify job NOT in priority queue
	queueKey := queue.queueKey(job.PriorityNormal)
	queueLength, _ := queue.client.LLen(context.Background(), queueKey).Result()
	if queueLength != 0 {
		t.Error("expected priority queue to be empty")
	}
}

func TestExponentialBackoff_Calculation(t *testing.T) {
	queue, mr := setupTestRedis(t)
	defer mr.Close()
	defer queue.Close()

	ctx := context.Background()

	tests := []struct {
		attempt      int
		expectedSecs int
	}{
		{1, 2},  // 2^1 = 2 seconds
		{2, 4},  // 2^2 = 4 seconds
		{3, 8},  // 2^3 = 8 seconds
		{4, 16}, // 2^4 = 16 seconds
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			// Create job
			j := job.NewJob("test_job", []byte(`{}`), job.PriorityNormal)
			j.MaxRetries = 10
			j.Attempts = tt.attempt - 1 // Set to one less so next fail increments to tt.attempt
			queue.Enqueue(ctx, j)

			// Dequeue and fail
			priorities := []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow}
			dequeuedJob, _ := queue.Dequeue(ctx, priorities)
			
			beforeFail := time.Now()
			queue.Fail(ctx, dequeuedJob, "error")
			
			// Retrieve job and check scheduled time
			failedJob, _ := queue.GetJob(ctx, j.ID)
			
			if failedJob.ScheduledFor == nil {
				t.Fatal("expected ScheduledFor to be set")
			}
			
			// Calculate actual delay
			actualDelay := failedJob.ScheduledFor.Sub(beforeFail)
			expectedDelay := time.Duration(tt.expectedSecs) * time.Second
			
			// Allow 1 second tolerance for test execution time
			if actualDelay < expectedDelay-time.Second || actualDelay > expectedDelay+time.Second {
				t.Errorf("expected delay ~%v, got %v", expectedDelay, actualDelay)
			}
			
			// Cleanup for next iteration
			mr.FlushDB()
		})
	}
}

func TestMoveScheduledToReady_MultipleJobs(t *testing.T) {
	queue, mr := setupTestRedis(t)
	defer mr.Close()
	defer queue.Close()

	ctx := context.Background()

	// Create and fail multiple jobs
	job1 := job.NewJob("job1", []byte(`{}`), job.PriorityHigh)
	job1.MaxRetries = 3
	job2 := job.NewJob("job2", []byte(`{}`), job.PriorityNormal)
	job2.MaxRetries = 3
	job3 := job.NewJob("job3", []byte(`{}`), job.PriorityLow)
	job3.MaxRetries = 3

	queue.Enqueue(ctx, job1)
	queue.Enqueue(ctx, job2)
	queue.Enqueue(ctx, job3)

	priorities := []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow}
	
	// Dequeue and fail all jobs
	dj1, _ := queue.Dequeue(ctx, priorities)
	queue.Fail(ctx, dj1, "error")
	dj2, _ := queue.Dequeue(ctx, priorities)
	queue.Fail(ctx, dj2, "error")
	dj3, _ := queue.Dequeue(ctx, priorities)
	queue.Fail(ctx, dj3, "error")

	// Manually update all scheduled jobs to be in the past
	// (miniredis FastForward doesn't work with sorted sets)
	scheduledKey := queue.getScheduledSetKey()
	pastTime := time.Now().Add(-10 * time.Second).Unix()
	queue.client.ZAdd(ctx, scheduledKey, redis.Z{Score: float64(pastTime), Member: job1.ID})
	queue.client.ZAdd(ctx, scheduledKey, redis.Z{Score: float64(pastTime), Member: job2.ID})
	queue.client.ZAdd(ctx, scheduledKey, redis.Z{Score: float64(pastTime), Member: job3.ID})

	// Move all scheduled jobs
	count, err := queue.MoveScheduledToReady(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 jobs moved, got %d", count)
	}

	// Verify all queues have their jobs back
	highLen, _ := queue.client.LLen(context.Background(), queue.queueKey(job.PriorityHigh)).Result()
	normalLen, _ := queue.client.LLen(context.Background(), queue.queueKey(job.PriorityNormal)).Result()
	lowLen, _ := queue.client.LLen(context.Background(), queue.queueKey(job.PriorityLow)).Result()

	if highLen != 1 || normalLen != 1 || lowLen != 1 {
		t.Errorf("expected 1 job in each queue, got high=%d normal=%d low=%d", highLen, normalLen, lowLen)
	}
}


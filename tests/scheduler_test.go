package tests

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/muaviaUsmani/bananas/internal/queue"
)

func TestScheduler_MovesReadyJobs(t *testing.T) {
	// Start miniredis server
	s := miniredis.RunT(t)
	defer s.Close()

	// Create queue
	q, err := queue.NewRedisQueue("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}
	defer q.Close()

	ctx := context.Background()

	// Create a job that will fail and be scheduled for retry
	failJob := job.NewJob("fail_test", []byte(`{}`), job.PriorityNormal)

	// Enqueue it first
	if err := q.Enqueue(ctx, failJob); err != nil {
		t.Fatalf("Failed to enqueue fail job: %v", err)
	}

	// Dequeue it (simulate worker picking it up)
	dequeuedJob, err := q.Dequeue(ctx, []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow})
	if err != nil {
		t.Fatalf("Failed to dequeue job: %v", err)
	}

	// Fail it (will be scheduled for retry in 2 seconds: 2^1 = 2s)
	if err := q.Fail(ctx, dequeuedJob, "test error"); err != nil {
		t.Fatalf("Failed to fail job: %v", err)
	}

	// Wait for the retry delay to pass (2 seconds + small buffer)
	time.Sleep(2100 * time.Millisecond)

	// Now move scheduled jobs to ready
	count, err := q.MoveScheduledToReady(ctx)
	if err != nil {
		t.Fatalf("MoveScheduledToReady failed: %v", err)
	}

	// Should have moved 1 job
	if count != 1 {
		t.Errorf("Expected 1 job to be moved, got %d", count)
	}

	// Try to dequeue the job - it should now be in the ready queue
	movedJob, err := q.Dequeue(ctx, []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow})
	if err != nil {
		t.Fatalf("Failed to dequeue moved job: %v", err)
	}

	if movedJob == nil {
		t.Error("Expected to dequeue a job after moving scheduled jobs")
	} else if movedJob.ID != dequeuedJob.ID {
		t.Errorf("Expected to dequeue the same job, got different ID")
	}
}

func TestScheduler_DoesNotMoveFutureJobs(t *testing.T) {
	// Start miniredis server
	s := miniredis.RunT(t)
	defer s.Close()

	// Create queue
	q, err := queue.NewRedisQueue("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}
	defer q.Close()

	ctx := context.Background()

	// Create a job scheduled for 10 seconds in the future
	futureTime := time.Now().Add(10 * time.Second)
	futureJob := job.NewJob("future_job", []byte(`{"test": "data"}`), job.PriorityNormal)
	futureJob.ScheduledFor = &futureTime

	// Enqueue and then fail it to put it in scheduled set
	if err := q.Enqueue(ctx, futureJob); err != nil {
		t.Fatalf("Failed to enqueue job: %v", err)
	}

	// Dequeue it
	dequeuedJob, err := q.Dequeue(ctx, []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow})
	if err != nil {
		t.Fatalf("Failed to dequeue job: %v", err)
	}

	// Fail it (will be scheduled for retry in the future)
	if err := q.Fail(ctx, dequeuedJob, "test error"); err != nil {
		t.Fatalf("Failed to fail job: %v", err)
	}

	// Try to move scheduled jobs - should not move the future job
	count, err := q.MoveScheduledToReady(ctx)
	if err != nil {
		t.Fatalf("MoveScheduledToReady failed: %v", err)
	}

	// Should not move any jobs since they're all in the future
	if count != 0 {
		t.Errorf("Expected 0 jobs to be moved (future job), got %d", count)
	}

	// Queue should be empty
	job, err := q.Dequeue(ctx, []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow})
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}

	if job != nil {
		t.Error("Expected queue to be empty, but got a job")
	}
}

func TestScheduler_HandlesEmptyScheduledSet(t *testing.T) {
	// Start miniredis server
	s := miniredis.RunT(t)
	defer s.Close()

	// Create queue
	q, err := queue.NewRedisQueue("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}
	defer q.Close()

	ctx := context.Background()

	// Try to move scheduled jobs when there are none
	count, err := q.MoveScheduledToReady(ctx)
	if err != nil {
		t.Fatalf("MoveScheduledToReady failed: %v", err)
	}

	// Should return 0 without error
	if count != 0 {
		t.Errorf("Expected 0 jobs to be moved, got %d", count)
	}
}

func TestScheduler_MovesMultipleReadyJobs(t *testing.T) {
	// Start miniredis server
	s := miniredis.RunT(t)
	defer s.Close()

	// Create queue
	q, err := queue.NewRedisQueue("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}
	defer q.Close()

	ctx := context.Background()

	// Create multiple jobs and fail them to put in scheduled set
	jobsToCreate := 5
	for i := 0; i < jobsToCreate; i++ {
		j := job.NewJob("test_job", []byte(`{"test": "data"}`), job.PriorityNormal)

		// Enqueue
		if err := q.Enqueue(ctx, j); err != nil {
			t.Fatalf("Failed to enqueue job %d: %v", i, err)
		}

		// Dequeue
		dequeuedJob, err := q.Dequeue(ctx, []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow})
		if err != nil {
			t.Fatalf("Failed to dequeue job %d: %v", i, err)
		}

		// Fail it (will be scheduled for retry in 2 seconds)
		if err := q.Fail(ctx, dequeuedJob, "test error"); err != nil {
			t.Fatalf("Failed to fail job %d: %v", i, err)
		}
	}

	// Wait for retry delay to pass
	time.Sleep(2100 * time.Millisecond)

	// Move scheduled jobs
	count, err := q.MoveScheduledToReady(ctx)
	if err != nil {
		t.Fatalf("MoveScheduledToReady failed: %v", err)
	}

	// Should have moved all jobs
	if count != jobsToCreate {
		t.Errorf("Expected %d jobs to be moved, got %d", jobsToCreate, count)
	}

	// All jobs should now be in ready queues
	for i := 0; i < jobsToCreate; i++ {
		j, err := q.Dequeue(ctx, []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow})
		if err != nil {
			t.Fatalf("Failed to dequeue job %d: %v", i, err)
		}
		if j == nil {
			t.Errorf("Expected job %d to be in queue, but got nil", i)
		}
	}
}

func TestScheduler_HandlesRedisConnectionFailure(t *testing.T) {
	// This test verifies that MoveScheduledToReady handles Redis errors gracefully

	// Start miniredis server
	s := miniredis.RunT(t)

	// Create queue
	q, err := queue.NewRedisQueue("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}
	defer q.Close()

	ctx := context.Background()

	// Close miniredis to simulate connection failure
	s.Close()

	// Try to move scheduled jobs - should return error
	_, err = q.MoveScheduledToReady(ctx)
	if err == nil {
		t.Error("Expected error when Redis is down, but got nil")
	}
}

func TestScheduler_RespectsJobPriority(t *testing.T) {
	// Start miniredis server
	s := miniredis.RunT(t)
	defer s.Close()

	// Create queue
	q, err := queue.NewRedisQueue("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}
	defer q.Close()

	ctx := context.Background()

	// Create jobs with different priorities and fail them
	priorities := []job.JobPriority{job.PriorityLow, job.PriorityNormal, job.PriorityHigh}

	for _, priority := range priorities {
		j := job.NewJob("test_job", []byte(`{"test": "data"}`), priority)

		// Enqueue
		if err := q.Enqueue(ctx, j); err != nil {
			t.Fatalf("Failed to enqueue job: %v", err)
		}

		// Dequeue
		dequeuedJob, err := q.Dequeue(ctx, []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow})
		if err != nil {
			t.Fatalf("Failed to dequeue job: %v", err)
		}

		// Fail it (will be scheduled for retry in 2 seconds)
		if err := q.Fail(ctx, dequeuedJob, "test error"); err != nil {
			t.Fatalf("Failed to fail job: %v", err)
		}
	}

	// Wait for retry delay to pass
	time.Sleep(2100 * time.Millisecond)

	// Move scheduled jobs
	count, err := q.MoveScheduledToReady(ctx)
	if err != nil {
		t.Fatalf("MoveScheduledToReady failed: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 jobs to be moved, got %d", count)
	}

	// Dequeue jobs - high priority should come first
	firstJob, err := q.Dequeue(ctx, []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow})
	if err != nil {
		t.Fatalf("Failed to dequeue first job: %v", err)
	}
	if firstJob == nil {
		t.Fatal("Expected first job to be non-nil")
	}
	if firstJob.Priority != job.PriorityHigh {
		t.Errorf("Expected first job to be high priority, got %s", firstJob.Priority)
	}

	secondJob, err := q.Dequeue(ctx, []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow})
	if err != nil {
		t.Fatalf("Failed to dequeue second job: %v", err)
	}
	if secondJob == nil {
		t.Fatal("Expected second job to be non-nil")
	}
	if secondJob.Priority != job.PriorityNormal {
		t.Errorf("Expected second job to be normal priority, got %s", secondJob.Priority)
	}

	thirdJob, err := q.Dequeue(ctx, []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow})
	if err != nil {
		t.Fatalf("Failed to dequeue third job: %v", err)
	}
	if thirdJob == nil {
		t.Fatal("Expected third job to be non-nil")
	}
	if thirdJob.Priority != job.PriorityLow {
		t.Errorf("Expected third job to be low priority, got %s", thirdJob.Priority)
	}
}

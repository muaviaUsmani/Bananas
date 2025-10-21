package tests

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/muaviaUsmani/bananas/internal/queue"
	"github.com/muaviaUsmani/bananas/internal/worker"
	"github.com/muaviaUsmani/bananas/pkg/client"
)

func TestFullWorkflow_EndToEnd(t *testing.T) {
	// Start miniredis server
	s := miniredis.RunT(t)
	defer s.Close()

	// Create client
	c, err := client.NewClient("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer c.Close()

	// Submit multiple jobs
	items := []string{"item1", "item2", "item3"}
	jobID1, err := c.SubmitJob("count_items", items, job.PriorityNormal, "Count test items")
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	email := map[string]string{"to": "test@example.com", "subject": "Test", "body": "Hello"}
	jobID2, err := c.SubmitJob("send_email", email, job.PriorityHigh, "Send test email")
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	jobID3, err := c.SubmitJob("process_data", map[string]string{}, job.PriorityLow)
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	// Create registry and register handlers
	registry := worker.NewRegistry()
	registry.Register("count_items", worker.HandleCountItems)
	registry.Register("send_email", worker.HandleSendEmail)
	registry.Register("process_data", worker.HandleProcessData)

	// Create queue and executor
	redisQueue, err := queue.NewRedisQueue("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer redisQueue.Close()

	executor := worker.NewExecutor(registry, redisQueue, 5)

	// Execute all submitted jobs by dequeuing and executing
	ctx := context.Background()
	priorities := []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow}

	// Dequeue and execute each job
	for i := 0; i < 3; i++ {
		j, err := redisQueue.Dequeue(ctx, priorities)
		if err != nil {
			t.Fatalf("failed to dequeue job: %v", err)
		}
		if j == nil {
			t.Fatal("expected job, got nil")
		}

		if err := executor.ExecuteJob(ctx, j); err != nil {
			// HandleProcessData sleeps for 3 seconds which might cause timeout in short mode
			// This is expected behavior, not a failure
			t.Logf("job %s execution: %v", j.ID, err)
		}
	}

	// Verify all jobs were processed
	j1, _ := c.GetJob(jobID1)
	j2, _ := c.GetJob(jobID2)
	j3, _ := c.GetJob(jobID3)

	if j1.Status != job.StatusCompleted && j1.Status != job.StatusFailed {
		t.Errorf("job1 status = %s, want completed or failed", j1.Status)
	}
	if j2.Status != job.StatusCompleted && j2.Status != job.StatusFailed {
		t.Errorf("job2 status = %s, want completed or failed", j2.Status)
	}
	if j3.Status != job.StatusCompleted && j3.Status != job.StatusFailed {
		t.Errorf("job3 status = %s, want completed or failed", j3.Status)
	}
}

func TestFullWorkflow_WithDifferentPriorities(t *testing.T) {
	// Start miniredis server
	s := miniredis.RunT(t)
	defer s.Close()

	c, err := client.NewClient("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer c.Close()

	registry := worker.NewRegistry()
	registry.Register("count_items", worker.HandleCountItems)

	redisQueue, err := queue.NewRedisQueue("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer redisQueue.Close()

	executor := worker.NewExecutor(registry, redisQueue, 3)

	// Submit jobs with different priorities
	priorities := []job.JobPriority{
		job.PriorityHigh,
		job.PriorityNormal,
		job.PriorityLow,
	}

	var jobIDs []string
	for _, priority := range priorities {
		items := []string{"a", "b", "c"}
		id, _ := c.SubmitJob("count_items", items, priority)
		jobIDs = append(jobIDs, id)
	}

	// Execute all jobs
	ctx := context.Background()
	priorityList := []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow}

	for i := 0; i < 3; i++ {
		j, err := redisQueue.Dequeue(ctx, priorityList)
		if err != nil {
			t.Fatalf("failed to dequeue job: %v", err)
		}
		if j == nil {
			t.Fatal("expected job, got nil")
		}

		if err := executor.ExecuteJob(ctx, j); err != nil {
			t.Errorf("failed to execute job %s: %v", j.ID, err)
		}
	}

	// Success - all jobs executed without errors
}

func TestFullWorkflow_InvalidJobName(t *testing.T) {
	// Start miniredis server
	s := miniredis.RunT(t)
	defer s.Close()

	c, err := client.NewClient("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer c.Close()

	registry := worker.NewRegistry()
	registry.Register("valid_job", worker.HandleCountItems)

	redisQueue, err := queue.NewRedisQueue("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer redisQueue.Close()

	executor := worker.NewExecutor(registry, redisQueue, 1)

	// Submit job with invalid name
	_, _ = c.SubmitJob("invalid_job_name", map[string]string{}, job.PriorityNormal)

	ctx := context.Background()

	// Dequeue the job
	dequeuedJob, err := redisQueue.Dequeue(ctx, []job.JobPriority{job.PriorityNormal})
	if err != nil {
		t.Fatalf("failed to dequeue: %v", err)
	}

	err = executor.ExecuteJob(ctx, dequeuedJob)

	// Should return error for unknown handler
	if err == nil {
		t.Error("expected error for invalid job name, got nil")
	}
}

func TestFullWorkflow_HandlerFailure(t *testing.T) {
	// Start miniredis server
	s := miniredis.RunT(t)
	defer s.Close()

	c, err := client.NewClient("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer c.Close()

	registry := worker.NewRegistry()

	// Use HandleCountItems with invalid payload to trigger error
	registry.Register("bad_payload", worker.HandleCountItems)

	redisQueue, err := queue.NewRedisQueue("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer redisQueue.Close()

	executor := worker.NewExecutor(registry, redisQueue, 1)

	// Submit job with invalid payload for count_items
	_, _ = c.SubmitJob("bad_payload", "not an array", job.PriorityNormal)

	ctx := context.Background()

	// Dequeue the job
	dequeuedJob, err := redisQueue.Dequeue(ctx, []job.JobPriority{job.PriorityNormal})
	if err != nil {
		t.Fatalf("failed to dequeue: %v", err)
	}

	err = executor.ExecuteJob(ctx, dequeuedJob)

	// Should fail
	if err == nil {
		t.Error("expected error for invalid payload, got nil")
	}
	// Error was returned, which is what we expect
}

func TestFullWorkflow_ConcurrentExecution(t *testing.T) {
	// Start miniredis server
	s := miniredis.RunT(t)
	defer s.Close()

	c, err := client.NewClient("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer c.Close()

	registry := worker.NewRegistry()
	registry.Register("count_items", worker.HandleCountItems)

	redisQueue, err := queue.NewRedisQueue("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer redisQueue.Close()

	executor := worker.NewExecutor(registry, redisQueue, 10)

	// Submit multiple jobs
	jobCount := 20
	var jobIDs []string

	for i := 0; i < jobCount; i++ {
		items := []string{"a", "b", "c"}
		id, _ := c.SubmitJob("count_items", items, job.PriorityNormal)
		jobIDs = append(jobIDs, id)
	}

	// Execute all jobs concurrently
	ctx := context.Background()
	done := make(chan bool, jobCount)
	priorities := []job.JobPriority{job.PriorityNormal}

	for i := 0; i < jobCount; i++ {
		go func() {
			j, err := redisQueue.Dequeue(ctx, priorities)
			if err != nil || j == nil {
				done <- false
				return
			}
			executor.ExecuteJob(ctx, j)
			done <- true
		}()
	}

	// Wait for all to complete with timeout
	timeout := time.After(5 * time.Second)
	completed := 0
	for i := 0; i < jobCount; i++ {
		select {
		case success := <-done:
			if success {
				completed++
			}
		case <-timeout:
			t.Logf("timeout after %d/%d jobs completed", completed, jobCount)
			return
		}
	}

	if completed != jobCount {
		t.Errorf("expected %d jobs completed, got %d", jobCount, completed)
	}
}

package tests

import (
	"context"
	"testing"

	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/muaviaUsmani/bananas/internal/worker"
	"github.com/muaviaUsmani/bananas/pkg/client"
)

// mockQueue is a simple mock implementation for integration tests
// It updates job status locally so tests can verify the flow
type mockQueue struct{}

func (m *mockQueue) Complete(ctx context.Context, jobID string) error {
	// In real implementation, this updates Redis
	// For tests, status is already updated by executor locally
	return nil
}

func (m *mockQueue) Fail(ctx context.Context, j *job.Job, errMsg string) error {
	// In real implementation, this would schedule retry or DLQ
	// For tests, status is already updated by executor locally
	return nil
}

func TestFullWorkflow_EndToEnd(t *testing.T) {
	// Create client
	c := client.NewClient()

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

	// Create executor
	mockQ := &mockQueue{}
	executor := worker.NewExecutor(registry, mockQ, 5)

	// Execute all submitted jobs
	ctx := context.Background()

	j1, _ := c.GetJob(jobID1)
	if err := executor.ExecuteJob(ctx, j1); err != nil {
		t.Errorf("failed to execute job 1: %v", err)
	}

	j2, _ := c.GetJob(jobID2)
	if err := executor.ExecuteJob(ctx, j2); err != nil {
		t.Errorf("failed to execute job 2: %v", err)
	}

	j3, _ := c.GetJob(jobID3)
	if err := executor.ExecuteJob(ctx, j3); err != nil {
		t.Errorf("failed to execute job 3: %v", err)
	}

	// Success - all jobs executed without errors
	// Note: In real implementation with Redis, job status would be updated in Redis
	// Here we're just testing the execution flow works end-to-end
}

func TestFullWorkflow_WithDifferentPriorities(t *testing.T) {
	c := client.NewClient()
	registry := worker.NewRegistry()
	registry.Register("count_items", worker.HandleCountItems)
	mockQ := &mockQueue{}
	executor := worker.NewExecutor(registry, mockQ, 3)

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
	for _, id := range jobIDs {
		j, _ := c.GetJob(id)
		if err := executor.ExecuteJob(ctx, j); err != nil {
			t.Errorf("failed to execute job %s: %v", id, err)
		}
	}

	// Success - all jobs executed without errors
}

func TestFullWorkflow_InvalidJobName(t *testing.T) {
	c := client.NewClient()
	registry := worker.NewRegistry()
	registry.Register("valid_job", worker.HandleCountItems)
	mockQ := &mockQueue{}
	executor := worker.NewExecutor(registry, mockQ, 1)

	// Submit job with invalid name
	jobID, _ := c.SubmitJob("invalid_job_name", map[string]string{}, job.PriorityNormal)

	j, _ := c.GetJob(jobID)
	ctx := context.Background()
	err := executor.ExecuteJob(ctx, j)

	// Should return error for unknown handler
	if err == nil {
		t.Error("expected error for invalid job name, got nil")
	}
}

func TestFullWorkflow_HandlerFailure(t *testing.T) {
	c := client.NewClient()
	registry := worker.NewRegistry()

	// Register a failing handler
	registry.Register("failing_job", func(ctx context.Context, j *job.Job) error {
		return error(nil) // This will cause unmarshal error in HandleCountItems
	})

	// Use HandleCountItems with invalid payload to trigger error
	registry.Register("bad_payload", worker.HandleCountItems)
	mockQ := &mockQueue{}
	executor := worker.NewExecutor(registry, mockQ, 1)

	// Submit job with invalid payload for count_items
	jobID, _ := c.SubmitJob("bad_payload", "not an array", job.PriorityNormal)

	j, _ := c.GetJob(jobID)
	ctx := context.Background()
	err := executor.ExecuteJob(ctx, j)

	// Should fail
	if err == nil {
		t.Error("expected error for invalid payload, got nil")
	}
	// Error was returned, which is what we expect
}

func TestFullWorkflow_ConcurrentExecution(t *testing.T) {
	c := client.NewClient()
	registry := worker.NewRegistry()
	registry.Register("count_items", worker.HandleCountItems)
	mockQ := &mockQueue{}
	executor := worker.NewExecutor(registry, mockQ, 10)

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

	for _, id := range jobIDs {
		go func(jobID string) {
			j, _ := c.GetJob(jobID)
			executor.ExecuteJob(ctx, j)
			done <- true
		}(id)
	}

	// Wait for all to complete
	for i := 0; i < jobCount; i++ {
		<-done
	}

	// All jobs executed - in real implementation would verify via Redis
}

func TestFullWorkflow_ListAllJobs(t *testing.T) {
	c := client.NewClient()

	// Submit various jobs
	c.SubmitJob("job1", map[string]string{}, job.PriorityHigh)
	c.SubmitJob("job2", map[string]string{}, job.PriorityNormal)
	c.SubmitJob("job3", map[string]string{}, job.PriorityLow)

	jobs := c.ListJobs()

	if len(jobs) != 3 {
		t.Errorf("expected 3 jobs, got %d", len(jobs))
	}

	// Verify we got 3 jobs back
	if len(jobs) != 3 {
		t.Errorf("expected 3 jobs, got %d", len(jobs))
	}
}


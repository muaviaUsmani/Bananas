package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
)

// mockQueue is a mock implementation of the Queue interface for testing
type mockQueue struct {
	completeCalled bool
	failCalled     bool
	lastError      string
	lastJobID      string
	completeErr    error
	failErr        error
}

func (m *mockQueue) Complete(ctx context.Context, jobID string) error {
	m.completeCalled = true
	m.lastJobID = jobID
	return m.completeErr
}

func (m *mockQueue) Fail(ctx context.Context, j *job.Job, errMsg string) error {
	m.failCalled = true
	m.lastError = errMsg
	m.lastJobID = j.ID
	return m.failErr
}

func TestNewExecutor(t *testing.T) {
	registry := NewRegistry()
	queue := &mockQueue{}
	concurrency := 5

	executor := NewExecutor(registry, queue, concurrency)

	if executor == nil {
		t.Fatal("expected executor to be created, got nil")
	}
	if executor.registry != registry {
		t.Error("expected executor registry to match provided registry")
	}
	if executor.queue != queue {
		t.Error("expected executor queue to match provided queue")
	}
	if executor.concurrency != concurrency {
		t.Errorf("expected concurrency %d, got %d", concurrency, executor.concurrency)
	}
}

func TestExecuteJob_ValidHandler(t *testing.T) {
	registry := NewRegistry()
	registry.Register("count_items", HandleCountItems)

	mockQ := &mockQueue{}
	executor := NewExecutor(registry, mockQ, 1)

	// Create test job
	payload, _ := json.Marshal([]string{"item1", "item2", "item3"})
	j := job.NewJob("count_items", payload, job.PriorityNormal)

	ctx := context.Background()
	err := executor.ExecuteJob(ctx, j)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !mockQ.completeCalled {
		t.Error("expected Complete to be called on queue")
	}
	if mockQ.lastJobID != j.ID {
		t.Errorf("expected job ID %s, got %s", j.ID, mockQ.lastJobID)
	}
}

func TestExecuteJob_UnknownHandler(t *testing.T) {
	registry := NewRegistry()
	mockQ := &mockQueue{}
	executor := NewExecutor(registry, mockQ, 1)

	// Create job with unknown handler
	j := job.NewJob("unknown_job", []byte("{}"), job.PriorityNormal)

	ctx := context.Background()
	err := executor.ExecuteJob(ctx, j)

	if err == nil {
		t.Fatal("expected error for unknown handler, got nil")
	}
	if !mockQ.failCalled {
		t.Error("expected Fail to be called on queue")
	}
}

func TestExecuteJob_StatusUpdates(t *testing.T) {
	registry := NewRegistry()
	registry.Register("process_data", HandleProcessData)

	mockQ := &mockQueue{}
	executor := NewExecutor(registry, mockQ, 1)

	j := job.NewJob("process_data", []byte("{}"), job.PriorityHigh)
	initialStatus := j.Status

	if initialStatus != job.StatusPending {
		t.Errorf("expected initial status %s, got %s", job.StatusPending, initialStatus)
	}

	ctx := context.Background()
	err := executor.ExecuteJob(ctx, j)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !mockQ.completeCalled {
		t.Error("expected Complete to be called on queue")
	}
}

func TestExecuteJob_HandlerError(t *testing.T) {
	registry := NewRegistry()

	// Register a handler that returns an error
	registry.Register("failing_job", func(ctx context.Context, j *job.Job) error {
		return errors.New("simulated failure")
	})

	mockQ := &mockQueue{}
	executor := NewExecutor(registry, mockQ, 1)
	j := job.NewJob("failing_job", []byte("{}"), job.PriorityNormal)

	ctx := context.Background()
	err := executor.ExecuteJob(ctx, j)

	if err == nil {
		t.Fatal("expected error from failing handler, got nil")
	}
	if !mockQ.failCalled {
		t.Error("expected Fail to be called on queue")
	}
	if mockQ.lastError != "simulated failure" {
		t.Errorf("expected error message 'simulated failure', got '%s'", mockQ.lastError)
	}
}

func TestExecuteJob_ContextCancellation(t *testing.T) {
	registry := NewRegistry()

	// Register a handler that checks context
	registry.Register("slow_job", func(ctx context.Context, j *job.Job) error {
		select {
		case <-time.After(5 * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	mockQ := &mockQueue{}
	executor := NewExecutor(registry, mockQ, 1)
	j := job.NewJob("slow_job", []byte("{}"), job.PriorityNormal)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := executor.ExecuteJob(ctx, j)

	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !mockQ.failCalled {
		t.Error("expected Fail to be called on queue")
	}
}


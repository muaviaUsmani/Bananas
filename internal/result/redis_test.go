package result

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return client, mr
}

func TestNewRedisBackend(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	backend := NewRedisBackend(client, time.Hour, 24*time.Hour)
	if backend == nil {
		t.Fatal("NewRedisBackend() returned nil")
	}

	if backend.successTTL != time.Hour {
		t.Errorf("successTTL = %v, want %v", backend.successTTL, time.Hour)
	}

	if backend.failureTTL != 24*time.Hour {
		t.Errorf("failureTTL = %v, want %v", backend.failureTTL, 24*time.Hour)
	}
}

func TestRedisBackend_StoreAndGetResult_Success(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	backend := NewRedisBackend(client, time.Hour, 24*time.Hour)
	ctx := context.Background()

	result := &job.JobResult{
		JobID:       "job123",
		Status:      job.StatusCompleted,
		Result:      []byte(`{"count":42}`),
		CompletedAt: time.Now().Truncate(time.Second),
		Duration:    5 * time.Second,
	}

	// Store result
	err := backend.StoreResult(ctx, result)
	if err != nil {
		t.Fatalf("StoreResult() error = %v", err)
	}

	// Retrieve result
	retrieved, err := backend.GetResult(ctx, "job123")
	if err != nil {
		t.Fatalf("GetResult() error = %v", err)
	}

	if retrieved == nil {
		t.Fatal("GetResult() returned nil")
	}

	if retrieved.JobID != result.JobID {
		t.Errorf("JobID = %v, want %v", retrieved.JobID, result.JobID)
	}

	if retrieved.Status != result.Status {
		t.Errorf("Status = %v, want %v", retrieved.Status, result.Status)
	}

	if string(retrieved.Result) != string(result.Result) {
		t.Errorf("Result = %v, want %v", string(retrieved.Result), string(result.Result))
	}

	if retrieved.Duration != result.Duration {
		t.Errorf("Duration = %v, want %v", retrieved.Duration, result.Duration)
	}
}

func TestRedisBackend_StoreAndGetResult_Failure(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	backend := NewRedisBackend(client, time.Hour, 24*time.Hour)
	ctx := context.Background()

	result := &job.JobResult{
		JobID:       "job456",
		Status:      job.StatusFailed,
		Error:       "something went wrong",
		CompletedAt: time.Now().Truncate(time.Second),
		Duration:    2 * time.Second,
	}

	// Store result
	err := backend.StoreResult(ctx, result)
	if err != nil {
		t.Fatalf("StoreResult() error = %v", err)
	}

	// Retrieve result
	retrieved, err := backend.GetResult(ctx, "job456")
	if err != nil {
		t.Fatalf("GetResult() error = %v", err)
	}

	if retrieved == nil {
		t.Fatal("GetResult() returned nil")
	}

	if retrieved.Status != job.StatusFailed {
		t.Errorf("Status = %v, want %v", retrieved.Status, job.StatusFailed)
	}

	if retrieved.Error != result.Error {
		t.Errorf("Error = %v, want %v", retrieved.Error, result.Error)
	}
}

func TestRedisBackend_GetResult_NotFound(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	backend := NewRedisBackend(client, time.Hour, 24*time.Hour)
	ctx := context.Background()

	// Try to get non-existent result
	result, err := backend.GetResult(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetResult() error = %v", err)
	}

	if result != nil {
		t.Errorf("GetResult() = %v, want nil", result)
	}
}

func TestRedisBackend_WaitForResult_AlreadyExists(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	backend := NewRedisBackend(client, time.Hour, 24*time.Hour)
	ctx := context.Background()

	// Store a result first
	result := &job.JobResult{
		JobID:       "job789",
		Status:      job.StatusCompleted,
		CompletedAt: time.Now(),
		Duration:    time.Second,
	}

	err := backend.StoreResult(ctx, result)
	if err != nil {
		t.Fatalf("StoreResult() error = %v", err)
	}

	// Wait for result (should return immediately)
	retrieved, err := backend.WaitForResult(ctx, "job789", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForResult() error = %v", err)
	}

	if retrieved == nil {
		t.Fatal("WaitForResult() returned nil")
	}

	if retrieved.JobID != "job789" {
		t.Errorf("JobID = %v, want job789", retrieved.JobID)
	}
}

func TestRedisBackend_WaitForResult_Timeout(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	backend := NewRedisBackend(client, time.Hour, 24*time.Hour)
	ctx := context.Background()

	// Wait for non-existent result with short timeout
	start := time.Now()
	result, err := backend.WaitForResult(ctx, "never-exists", 500*time.Millisecond)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("WaitForResult() error = %v", err)
	}

	if result != nil {
		t.Errorf("WaitForResult() = %v, want nil", result)
	}

	// Check that it actually waited
	if duration < 400*time.Millisecond {
		t.Errorf("WaitForResult() duration = %v, expected ~500ms", duration)
	}
}

func TestRedisBackend_WaitForResult_Notified(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	backend := NewRedisBackend(client, time.Hour, 24*time.Hour)
	ctx := context.Background()

	jobID := "job-notify"

	// Start waiting in a goroutine
	resultChan := make(chan *job.JobResult)
	errChan := make(chan error)

	go func() {
		result, err := backend.WaitForResult(ctx, jobID, 5*time.Second)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- result
	}()

	// Give the subscriber time to set up
	time.Sleep(100 * time.Millisecond)

	// Store result (this will trigger notification)
	result := &job.JobResult{
		JobID:       jobID,
		Status:      job.StatusCompleted,
		CompletedAt: time.Now(),
		Duration:    time.Second,
	}

	err := backend.StoreResult(ctx, result)
	if err != nil {
		t.Fatalf("StoreResult() error = %v", err)
	}

	// Wait for result
	select {
	case err := <-errChan:
		t.Fatalf("WaitForResult() error = %v", err)
	case retrieved := <-resultChan:
		if retrieved == nil {
			t.Fatal("WaitForResult() returned nil")
		}
		if retrieved.JobID != jobID {
			t.Errorf("JobID = %v, want %v", retrieved.JobID, jobID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForResult() timed out")
	}
}

func TestRedisBackend_DeleteResult(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	backend := NewRedisBackend(client, time.Hour, 24*time.Hour)
	ctx := context.Background()

	// Store a result
	result := &job.JobResult{
		JobID:       "job-delete",
		Status:      job.StatusCompleted,
		CompletedAt: time.Now(),
		Duration:    time.Second,
	}

	err := backend.StoreResult(ctx, result)
	if err != nil {
		t.Fatalf("StoreResult() error = %v", err)
	}

	// Verify it exists
	retrieved, err := backend.GetResult(ctx, "job-delete")
	if err != nil {
		t.Fatalf("GetResult() error = %v", err)
	}
	if retrieved == nil {
		t.Fatal("Result should exist before deletion")
	}

	// Delete it
	err = backend.DeleteResult(ctx, "job-delete")
	if err != nil {
		t.Fatalf("DeleteResult() error = %v", err)
	}

	// Verify it's gone
	retrieved, err = backend.GetResult(ctx, "job-delete")
	if err != nil {
		t.Fatalf("GetResult() after delete error = %v", err)
	}
	if retrieved != nil {
		t.Error("Result should not exist after deletion")
	}
}

func TestRedisBackend_DeleteResult_NotFound(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	backend := NewRedisBackend(client, time.Hour, 24*time.Hour)
	ctx := context.Background()

	// Delete non-existent result (should not error)
	err := backend.DeleteResult(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("DeleteResult() error = %v", err)
	}
}

func TestRedisBackend_TTL(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()

	successTTL := 2 * time.Second
	failureTTL := 5 * time.Second

	backend := NewRedisBackend(client, successTTL, failureTTL)
	ctx := context.Background()

	t.Run("Success TTL", func(t *testing.T) {
		result := &job.JobResult{
			JobID:       "job-ttl-success",
			Status:      job.StatusCompleted,
			CompletedAt: time.Now(),
			Duration:    time.Second,
		}

		err := backend.StoreResult(ctx, result)
		if err != nil {
			t.Fatalf("StoreResult() error = %v", err)
		}

		// Check TTL in Redis
		key := "bananas:result:job-ttl-success"
		ttl := mr.TTL(key)
		if ttl <= 0 || ttl > successTTL {
			t.Errorf("TTL = %v, want <= %v and > 0", ttl, successTTL)
		}
	})

	t.Run("Failure TTL", func(t *testing.T) {
		result := &job.JobResult{
			JobID:       "job-ttl-failure",
			Status:      job.StatusFailed,
			Error:       "failed",
			CompletedAt: time.Now(),
			Duration:    time.Second,
		}

		err := backend.StoreResult(ctx, result)
		if err != nil {
			t.Fatalf("StoreResult() error = %v", err)
		}

		// Check TTL in Redis
		key := "bananas:result:job-ttl-failure"
		ttl := mr.TTL(key)
		if ttl <= 0 || ttl > failureTTL {
			t.Errorf("TTL = %v, want <= %v and > 0", ttl, failureTTL)
		}
	})
}

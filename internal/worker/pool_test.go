package worker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
)

// mockQueueReader is a mock implementation for testing the pool
type mockQueueReader struct {
	jobs   []*job.Job
	mu     sync.Mutex
	called int
}

func (m *mockQueueReader) Dequeue(ctx context.Context, priorities []job.JobPriority) (*job.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.called++

	if len(m.jobs) == 0 {
		return nil, nil
	}

	j := m.jobs[0]
	m.jobs = m.jobs[1:]
	return j, nil
}

func TestNewPool(t *testing.T) {
	registry := NewRegistry()
	mockQ := &mockQueue{}
	executor := NewExecutor(registry, mockQ, 5)
	reader := &mockQueueReader{}

	pool := NewPool(executor, reader, 5, 10*time.Second)

	if pool == nil {
		t.Fatal("expected pool to be created")
	}
	if pool.concurrency != 5 {
		t.Errorf("expected concurrency 5, got %d", pool.concurrency)
	}
	if pool.jobTimeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", pool.jobTimeout)
	}
}

func TestPool_StartStop(t *testing.T) {
	registry := NewRegistry()
	mockQ := &mockQueue{}
	executor := NewExecutor(registry, mockQ, 2)
	reader := &mockQueueReader{}

	pool := NewPool(executor, reader, 2, 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the pool
	pool.Start(ctx)

	// Give workers time to start
	time.Sleep(100 * time.Millisecond)

	// Stop the pool
	pool.Stop()

	// Verify dequeue was called (workers were running)
	if reader.called == 0 {
		t.Error("expected Dequeue to be called at least once")
	}
}

func TestPool_ProcessesJobs(t *testing.T) {
	registry := NewRegistry()

	// Track processed jobs
	var processed []string
	var mu sync.Mutex

	registry.Register("test_job", func(ctx context.Context, j *job.Job) error {
		mu.Lock()
		processed = append(processed, j.ID)
		mu.Unlock()
		return nil
	})

	mockQ := &mockQueue{}
	executor := NewExecutor(registry, mockQ, 2)

	// Create test jobs
	job1 := job.NewJob("test_job", []byte("{}"), job.PriorityNormal)
	job2 := job.NewJob("test_job", []byte("{}"), job.PriorityNormal)
	job3 := job.NewJob("test_job", []byte("{}"), job.PriorityHigh)

	reader := &mockQueueReader{
		jobs: []*job.Job{job1, job2, job3},
	}

	pool := NewPool(executor, reader, 2, 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)

	// Wait for jobs to be processed
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		count := len(processed)
		mu.Unlock()

		if count >= 3 {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for jobs to be processed")
		}

		time.Sleep(50 * time.Millisecond)
	}

	pool.Stop()

	// Verify all jobs were processed
	mu.Lock()
	if len(processed) != 3 {
		t.Errorf("expected 3 jobs processed, got %d", len(processed))
	}
	mu.Unlock()
}

func TestPool_ConcurrencyLimit(t *testing.T) {
	registry := NewRegistry()

	// Track concurrent executions
	var concurrent int
	var maxConcurrent int
	var mu sync.Mutex

	registry.Register("slow_job", func(ctx context.Context, j *job.Job) error {
		mu.Lock()
		concurrent++
		if concurrent > maxConcurrent {
			maxConcurrent = concurrent
		}
		mu.Unlock()

		// Simulate work
		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		concurrent--
		mu.Unlock()

		return nil
	})

	mockQ := &mockQueue{}
	executor := NewExecutor(registry, mockQ, 3)

	// Create many jobs
	var jobs []*job.Job
	for i := 0; i < 10; i++ {
		jobs = append(jobs, job.NewJob("slow_job", []byte("{}"), job.PriorityNormal))
	}

	reader := &mockQueueReader{jobs: jobs}
	pool := NewPool(executor, reader, 3, 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)

	// Wait for some jobs to process
	time.Sleep(500 * time.Millisecond)

	pool.Stop()

	// Verify concurrency was limited
	mu.Lock()
	if maxConcurrent > 3 {
		t.Errorf("expected max concurrency 3, got %d", maxConcurrent)
	}
	mu.Unlock()
}

func TestPool_RespectsJobTimeout(t *testing.T) {
	registry := NewRegistry()

	// Handler that takes longer than timeout
	registry.Register("long_job", func(ctx context.Context, j *job.Job) error {
		select {
		case <-time.After(2 * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	mockQ := &mockQueue{}
	executor := NewExecutor(registry, mockQ, 1)

	j := job.NewJob("long_job", []byte("{}"), job.PriorityNormal)
	reader := &mockQueueReader{jobs: []*job.Job{j}}

	// Set short timeout
	pool := NewPool(executor, reader, 1, 100*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)

	// Wait for job to timeout
	time.Sleep(500 * time.Millisecond)

	pool.Stop()

	// Verify Fail was called (job timed out)
	if !mockQ.failCalled {
		t.Error("expected Fail to be called when job times out")
	}
}


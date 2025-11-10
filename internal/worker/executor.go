package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/muaviaUsmani/bananas/internal/metrics"
	"github.com/muaviaUsmani/bananas/internal/result"
)

// Queue interface defines the methods needed for job queue operations
type Queue interface {
	Complete(ctx context.Context, jobID string) error
	Fail(ctx context.Context, j *job.Job, errMsg string) error
}

// Executor manages job execution with concurrency control
type Executor struct {
	registry      *Registry
	queue         Queue
	resultBackend result.Backend
	concurrency   int
}

// NewExecutor creates a new job executor with Redis queue integration
func NewExecutor(registry *Registry, queue Queue, concurrency int) *Executor {
	return &Executor{
		registry:    registry,
		queue:       queue,
		concurrency: concurrency,
	}
}

// SetResultBackend sets the result backend for storing job results
// This is optional - if not set, results won't be stored
func (e *Executor) SetResultBackend(backend result.Backend) {
	e.resultBackend = backend
}

// ExecuteJob executes a single job using the registered handler and updates Redis queue
func (e *Executor) ExecuteJob(ctx context.Context, j *job.Job) error {
	// Look up handler
	handler, exists := e.registry.Get(j.Name)
	if !exists {
		err := fmt.Errorf("no handler registered for job: %s", j.Name)
		// Mark as failed in queue (will go to dead letter queue after max retries)
		if queueErr := e.queue.Fail(ctx, j, err.Error()); queueErr != nil {
			log.Printf("Failed to mark job %s as failed in queue: %v", j.ID, queueErr)
		}
		return err
	}

	// Update status to Processing (already done by queue.Dequeue, but update locally)
	j.UpdateStatus(job.StatusProcessing)
	log.Printf("Executing job %s (name: %s, priority: %s)", j.ID, j.Name, j.Priority)

	// Record job started in metrics
	metrics.Default().RecordJobStarted(j.Priority)

	// Execute handler with context
	startTime := time.Now()
	err := handler(ctx, j)
	duration := time.Since(startTime)

	// Update job based on result
	if err != nil {
		// Check if error was due to context cancellation
		if ctx.Err() != nil {
			log.Printf("Job %s cancelled: %v", j.ID, ctx.Err())
			errMsg := fmt.Sprintf("context cancelled: %v", ctx.Err())

			// Record job failure in metrics
			metrics.Default().RecordJobFailed(j.Priority, duration)

			// Store result if backend is configured
			e.storeResult(ctx, j.ID, job.StatusFailed, nil, errMsg, duration)

			// Mark as failed in queue (will trigger exponential backoff retry)
			if queueErr := e.queue.Fail(ctx, j, errMsg); queueErr != nil {
				log.Printf("Failed to update job %s in queue after cancellation: %v", j.ID, queueErr)
			}
			return fmt.Errorf("job cancelled: %w", ctx.Err())
		}

		// Handler returned an error
		log.Printf("Job %s failed after %v: %v", j.ID, duration, err)

		// Record job failure in metrics
		metrics.Default().RecordJobFailed(j.Priority, duration)

		// Store result if backend is configured
		e.storeResult(ctx, j.ID, job.StatusFailed, nil, err.Error(), duration)

		// Mark as failed in queue (will trigger exponential backoff retry)
		if queueErr := e.queue.Fail(ctx, j, err.Error()); queueErr != nil {
			log.Printf("Failed to update job %s in queue after failure: %v", j.ID, queueErr)
		}
		return err
	}

	// Success - mark as completed in queue
	log.Printf("Job %s completed successfully in %v", j.ID, duration)

	// Record job completion in metrics
	metrics.Default().RecordJobCompleted(j.Priority, duration)

	// Store result if backend is configured
	e.storeResult(ctx, j.ID, job.StatusCompleted, nil, "", duration)

	if err := e.queue.Complete(ctx, j.ID); err != nil {
		log.Printf("Failed to mark job %s as completed in queue: %v", j.ID, err)
		return fmt.Errorf("job succeeded but failed to update queue: %w", err)
	}

	return nil
}

// storeResult stores the job result in the backend if configured
// This is a best-effort operation - failures are logged but don't fail the job
func (e *Executor) storeResult(ctx context.Context, jobID string, status job.JobStatus, resultData []byte, errorMsg string, duration time.Duration) {
	if e.resultBackend == nil {
		return // Result backend not configured
	}

	result := &job.JobResult{
		JobID:       jobID,
		Status:      status,
		Result:      resultData,
		Error:       errorMsg,
		CompletedAt: time.Now(),
		Duration:    duration,
	}

	if err := e.resultBackend.StoreResult(ctx, result); err != nil {
		log.Printf("Failed to store result for job %s: %v", jobID, err)
	}
}


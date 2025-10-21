package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
)

// Queue interface defines the methods needed for job queue operations
type Queue interface {
	Complete(ctx context.Context, jobID string) error
	Fail(ctx context.Context, j *job.Job, errMsg string) error
}

// Executor manages job execution with concurrency control
type Executor struct {
	registry    *Registry
	queue       Queue
	concurrency int
}

// NewExecutor creates a new job executor with Redis queue integration
func NewExecutor(registry *Registry, queue Queue, concurrency int) *Executor {
	return &Executor{
		registry:    registry,
		queue:       queue,
		concurrency: concurrency,
	}
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
			
			// Mark as failed in queue (will trigger exponential backoff retry)
			if queueErr := e.queue.Fail(ctx, j, errMsg); queueErr != nil {
				log.Printf("Failed to update job %s in queue after cancellation: %v", j.ID, queueErr)
			}
			return fmt.Errorf("job cancelled: %w", ctx.Err())
		}

		// Handler returned an error
		log.Printf("Job %s failed after %v: %v", j.ID, duration, err)
		
		// Mark as failed in queue (will trigger exponential backoff retry)
		if queueErr := e.queue.Fail(ctx, j, err.Error()); queueErr != nil {
			log.Printf("Failed to update job %s in queue after failure: %v", j.ID, queueErr)
		}
		return err
	}

	// Success - mark as completed in queue
	log.Printf("Job %s completed successfully in %v", j.ID, duration)
	if err := e.queue.Complete(ctx, j.ID); err != nil {
		log.Printf("Failed to mark job %s as completed in queue: %v", j.ID, err)
		return fmt.Errorf("job succeeded but failed to update queue: %w", err)
	}

	return nil
}


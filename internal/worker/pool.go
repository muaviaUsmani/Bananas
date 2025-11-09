package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/muaviaUsmani/bananas/internal/logger"
)

// QueueReader defines the interface for dequeuing jobs from the queue
type QueueReader interface {
	Dequeue(ctx context.Context, priorities []job.JobPriority) (*job.Job, error)
}

// Pool manages a pool of workers that process jobs from the queue
type Pool struct {
	executor    *Executor
	queue       QueueReader
	concurrency int
	jobTimeout  time.Duration
	priorities  []job.JobPriority
	wg          sync.WaitGroup
	stopChan    chan struct{}
}

// NewPool creates a new worker pool
func NewPool(executor *Executor, queue QueueReader, concurrency int, jobTimeout time.Duration) *Pool {
	return &Pool{
		executor:    executor,
		queue:       queue,
		concurrency: concurrency,
		jobTimeout:  jobTimeout,
		priorities: []job.JobPriority{
			job.PriorityHigh,
			job.PriorityNormal,
			job.PriorityLow,
		},
		stopChan: make(chan struct{}),
	}
}

// Start begins processing jobs from the queue with the configured concurrency
func (p *Pool) Start(ctx context.Context) {
	logger.Info("Starting worker pool", "workers", p.concurrency)

	// Start worker goroutines
	for i := 0; i < p.concurrency; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i+1)
	}

	logger.Info("Worker pool started successfully")
}

// Stop gracefully shuts down the worker pool with a 30-second timeout
func (p *Pool) Stop() {
	logger.Info("Stopping worker pool")
	close(p.stopChan)

	// Wait for workers with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("Worker pool stopped gracefully")
	case <-time.After(30 * time.Second):
		logger.Warn("Worker pool shutdown timed out", "timeout", "30s")
	}
}

// worker is the main loop for each worker goroutine
func (p *Pool) worker(ctx context.Context, workerID int) {
	defer p.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Worker recovered from panic", "worker_id", workerID, "panic", r)
		}
	}()

	// Create worker-specific context with worker_id
	workerCtx := context.WithValue(ctx, "worker_id", fmt.Sprintf("worker-%d", workerID))

	logger.Info("Worker started", "worker_id", workerID)

	for {
		select {
		case <-p.stopChan:
			logger.Info("Worker stopping", "worker_id", workerID)
			return
		case <-workerCtx.Done():
			logger.Info("Worker stopping due to context cancellation", "worker_id", workerID)
			return
		default:
			// Try to dequeue a job (uses blocking operations internally)
			j, err := p.queue.Dequeue(workerCtx, p.priorities)
			if err != nil {
				// Check if context was cancelled
				if workerCtx.Err() != nil {
					logger.Info("Worker stopping due to context cancellation", "worker_id", workerID)
					return
				}
				logger.Error("Error dequeuing job", "worker_id", workerID, "error", err)
				// Wait a bit before retrying to avoid tight loop on persistent errors
				time.Sleep(time.Second)
				continue
			}

			// No job available (all queues empty after timeout)
			if j == nil {
				// Dequeue uses blocking operations with timeouts, so no sleep needed
				// Just loop back and try again
				continue
			}

			// Execute the job with timeout
			p.executeWithTimeout(workerCtx, workerID, j)
		}
	}
}

// executeWithTimeout executes a job with the configured timeout
func (p *Pool) executeWithTimeout(ctx context.Context, workerID int, j *job.Job) {
	// Recover from panics during job execution
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Job panicked", "worker_id", workerID, "job_id", j.ID, "panic", r)
		}
	}()

	// Add job_id to context
	jobCtx := context.WithValue(ctx, "job_id", j.ID)

	// Create context with timeout for job execution
	jobCtx, cancel := context.WithTimeout(jobCtx, p.jobTimeout)
	defer cancel()

	// Use job-specific logger
	jobLogger := logger.Default().WithSource(logger.LogSourceJob)
	jobLogger.InfoContext(jobCtx, "Processing job", "worker_id", workerID, "job_id", j.ID, "job_name", j.Name, "priority", j.Priority)

	// Execute the job
	if err := p.executor.ExecuteJob(jobCtx, j); err != nil {
		jobLogger.ErrorContext(jobCtx, "Job failed", "worker_id", workerID, "job_id", j.ID, "error", err)
	} else {
		jobLogger.InfoContext(jobCtx, "Job completed", "worker_id", workerID, "job_id", j.ID)
	}
}


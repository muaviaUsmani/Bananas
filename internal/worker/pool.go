package worker

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/muaviaUsmani/bananas/internal/logger"
	"github.com/muaviaUsmani/bananas/internal/metrics"
)

// QueueReader defines the interface for dequeuing jobs from the queue
type QueueReader interface {
	Dequeue(ctx context.Context, priorities []job.JobPriority) (*job.Job, error)
	Fail(ctx context.Context, j *job.Job, errMsg string) error
}

// Pool manages a pool of workers that process jobs from the queue
type Pool struct {
	executor          *Executor
	queue             QueueReader
	concurrency       int
	jobTimeout        time.Duration
	priorities        []job.JobPriority
	wg                sync.WaitGroup
	stopChan          chan struct{}
	activeWorkers     atomic.Int64
	redisRetryBackoff time.Duration // Current backoff for Redis connection errors
	maxRetryBackoff   time.Duration // Maximum backoff duration (default 30s)
}

// NewPool creates a new worker pool
func NewPool(executor *Executor, queue QueueReader, concurrency int, jobTimeout time.Duration) *Pool {
	return &Pool{
		executor:          executor,
		queue:             queue,
		concurrency:       concurrency,
		jobTimeout:        jobTimeout,
		redisRetryBackoff: time.Second,      // Initial backoff: 1 second
		maxRetryBackoff:   30 * time.Second, // Max backoff: 30 seconds
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
			stackTrace := string(debug.Stack())
			logger.Error("Worker recovered from panic - worker will be terminated",
				"worker_id", workerID,
				"panic_value", r,
				"stack_trace", stackTrace)
		}
	}()

	// Create worker-specific context with worker_id
	workerCtx := context.WithValue(ctx, "worker_id", fmt.Sprintf("worker-%d", workerID))

	logger.Info("Worker started", "worker_id", workerID)

	// Track consecutive Redis failures for exponential backoff
	consecutiveFailures := 0
	currentBackoff := time.Second

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

				// Redis connection error - use exponential backoff
				consecutiveFailures++

				// Calculate backoff: min(2^failures * 1s, maxBackoff)
				currentBackoff = time.Duration(1<<uint(consecutiveFailures)) * time.Second
				if currentBackoff > p.maxRetryBackoff {
					currentBackoff = p.maxRetryBackoff
				}

				// Log with different severity based on failure count
				if consecutiveFailures <= 3 {
					logger.Warn("Redis connection error - retrying with backoff",
						"worker_id", workerID,
						"error", err,
						"consecutive_failures", consecutiveFailures,
						"backoff", currentBackoff)
				} else if consecutiveFailures%10 == 0 {
					// Log every 10th failure after the first 3 to avoid log spam
					logger.Error("Persistent Redis connection errors",
						"worker_id", workerID,
						"error", err,
						"consecutive_failures", consecutiveFailures,
						"backoff", currentBackoff)
				}

				// Wait before retrying
				time.Sleep(currentBackoff)
				continue
			}

			// Successfully dequeued (or queue empty) - reset failure counter
			if consecutiveFailures > 0 {
				logger.Info("Redis connection recovered", "worker_id", workerID, "after_failures", consecutiveFailures)
				consecutiveFailures = 0
				currentBackoff = time.Second
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
	// Mark worker as active
	active := p.activeWorkers.Add(1)
	defer func() {
		active = p.activeWorkers.Add(-1)
		// Update metrics with current worker utilization
		metrics.Default().RecordWorkerActivity(active, int64(p.concurrency))
	}()

	// Update metrics with current worker utilization
	metrics.Default().RecordWorkerActivity(active, int64(p.concurrency))

	// Add job_id to context
	jobCtx := context.WithValue(ctx, "job_id", j.ID)

	// Create context with timeout for job execution
	jobCtx, cancel := context.WithTimeout(jobCtx, p.jobTimeout)
	defer cancel()

	// Use job-specific logger
	jobLogger := logger.Default().WithSource(logger.LogSourceJob)

	// Recover from panics during job execution and mark job as failed
	defer func() {
		if r := recover(); r != nil {
			// Capture stack trace
			stackTrace := string(debug.Stack())

			// Format panic error message with stack trace
			panicMsg := fmt.Sprintf("PANIC: %v\n\nStack Trace:\n%s", r, stackTrace)

			// Log the panic with full details
			jobLogger.ErrorContext(jobCtx, "Job panicked - marking as failed",
				"worker_id", workerID,
				"job_id", j.ID,
				"job_name", j.Name,
				"panic_value", r,
				"stack_trace", stackTrace)

			// Mark job as failed in queue (will retry or move to dead letter based on attempts)
			if err := p.queue.Fail(ctx, j, panicMsg); err != nil {
				logger.Error("Failed to mark panicked job as failed",
					"worker_id", workerID,
					"job_id", j.ID,
					"error", err)
			}

			// Record failure in metrics
			metrics.Default().RecordJobFailed(j.Priority, 0) // Duration is 0 since job panicked
		}
	}()

	jobLogger.InfoContext(jobCtx, "Processing job", "worker_id", workerID, "job_id", j.ID, "job_name", j.Name, "priority", j.Priority)

	// Execute the job
	if err := p.executor.ExecuteJob(jobCtx, j); err != nil {
		jobLogger.ErrorContext(jobCtx, "Job failed", "worker_id", workerID, "job_id", j.ID, "error", err)
	} else {
		jobLogger.InfoContext(jobCtx, "Job completed", "worker_id", workerID, "job_id", j.ID)
	}
}


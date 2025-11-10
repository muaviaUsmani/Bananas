package worker

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/muaviaUsmani/bananas/internal/config"
	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/muaviaUsmani/bananas/internal/logger"
	"github.com/muaviaUsmani/bananas/internal/metrics"
)

// QueueReader defines the interface for dequeuing jobs from the queue
type QueueReader interface {
	Dequeue(ctx context.Context, priorities []job.JobPriority) (*job.Job, error)
	DequeueWithRouting(ctx context.Context, routingKeys []string) (*job.Job, error)
	Fail(ctx context.Context, j *job.Job, errMsg string) error
}

// Pool manages a pool of workers that process jobs from the queue
type Pool struct {
	executor          *Executor
	queue             QueueReader
	workerConfig      *config.WorkerConfig
	jobTimeout        time.Duration
	wg                sync.WaitGroup
	stopChan          chan struct{}
	activeWorkers     atomic.Int64
	redisRetryBackoff time.Duration // Current backoff for Redis connection errors
	maxRetryBackoff   time.Duration // Maximum backoff duration (default 30s)
}

// NewPool creates a new worker pool
// Deprecated: Use NewPoolWithConfig instead
func NewPool(executor *Executor, queue QueueReader, concurrency int, jobTimeout time.Duration) *Pool {
	// Create a default worker config for backward compatibility
	workerConfig := &config.WorkerConfig{
		Mode:              config.WorkerModeDefault,
		Concurrency:       concurrency,
		Priorities:        []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow},
		RoutingKeys:       []string{"default"}, // Default routing key
		JobTypes:          nil,                  // All job types
		SchedulerInterval: 1 * time.Second,
		EnableScheduler:   true,
	}

	return NewPoolWithConfig(executor, queue, workerConfig, jobTimeout)
}

// NewPoolWithConfig creates a new worker pool with explicit configuration
func NewPoolWithConfig(executor *Executor, queue QueueReader, workerConfig *config.WorkerConfig, jobTimeout time.Duration) *Pool {
	return &Pool{
		executor:          executor,
		queue:             queue,
		workerConfig:      workerConfig,
		jobTimeout:        jobTimeout,
		redisRetryBackoff: time.Second,      // Initial backoff: 1 second
		maxRetryBackoff:   30 * time.Second, // Max backoff: 30 seconds
		stopChan:          make(chan struct{}),
	}
}

// Start begins processing jobs from the queue with the configured concurrency
func (p *Pool) Start(ctx context.Context) {
	logger.Info("Starting worker pool",
		"mode", p.workerConfig.Mode,
		"workers", p.workerConfig.Concurrency,
		"priorities", len(p.workerConfig.Priorities),
		"scheduler_enabled", p.workerConfig.EnableScheduler)

	// Log worker configuration details
	logger.Info("Worker configuration", "config", p.workerConfig.String())

	// Start worker goroutines (unless scheduler-only mode)
	if p.workerConfig.Mode != config.WorkerModeSchedulerOnly {
		for i := 0; i < p.workerConfig.Concurrency; i++ {
			p.wg.Add(1)
			go p.worker(ctx, i+1)
		}
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
			// Use routing-aware dequeue if routing keys are configured
			var j *job.Job
			var err error
			if len(p.workerConfig.RoutingKeys) > 0 {
				j, err = p.queue.DequeueWithRouting(workerCtx, p.workerConfig.RoutingKeys)
			} else {
				// Fall back to priority-based dequeue for backward compatibility
				j, err = p.queue.Dequeue(workerCtx, p.workerConfig.Priorities)
			}
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

			// Check if this worker should process this job (job-type filtering)
			if !p.workerConfig.ShouldProcessJob(j) {
				// Job doesn't match our filters (wrong job type for job-specialized mode)
				// Put it back in the queue by marking it as failed with zero attempts
				// Actually, we should skip it - another worker will pick it up
				// For now, we'll process it anyway since we already dequeued it
				// TODO: Implement proper job re-enqueue mechanism
				logger.Debug("Skipping job due to job-type filter",
					"worker_id", workerID,
					"job_id", j.ID,
					"job_name", j.Name,
					"allowed_types", p.workerConfig.JobTypes)
				// For now, just continue - in practice this shouldn't happen often
				// because queue separation should prevent this
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
		metrics.Default().RecordWorkerActivity(active, int64(p.workerConfig.Concurrency))
	}()

	// Update metrics with current worker utilization
	metrics.Default().RecordWorkerActivity(active, int64(p.workerConfig.Concurrency))

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


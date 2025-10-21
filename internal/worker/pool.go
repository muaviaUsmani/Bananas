package worker

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
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
	log.Printf("Starting worker pool with %d workers", p.concurrency)

	// Start worker goroutines
	for i := 0; i < p.concurrency; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i+1)
	}

	log.Printf("Worker pool started successfully")
}

// Stop gracefully shuts down the worker pool with a 30-second timeout
func (p *Pool) Stop() {
	log.Println("Stopping worker pool...")
	close(p.stopChan)

	// Wait for workers with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Worker pool stopped gracefully")
	case <-time.After(30 * time.Second):
		log.Println("Warning: Worker pool shutdown timed out after 30 seconds")
	}
}

// worker is the main loop for each worker goroutine
func (p *Pool) worker(ctx context.Context, workerID int) {
	defer p.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Worker %d recovered from panic: %v", workerID, r)
		}
	}()

	log.Printf("Worker %d started", workerID)

	for {
		select {
		case <-p.stopChan:
			log.Printf("Worker %d stopping...", workerID)
			return
		case <-ctx.Done():
			log.Printf("Worker %d stopping due to context cancellation", workerID)
			return
		default:
			// Try to dequeue a job
			j, err := p.queue.Dequeue(ctx, p.priorities)
			if err != nil {
				log.Printf("Worker %d: error dequeuing job: %v", workerID, err)
				// Wait a bit before retrying to avoid tight loop on persistent errors
				time.Sleep(time.Second)
				continue
			}

			// No job available
			if j == nil {
				// Sleep briefly to avoid tight polling loop
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Execute the job with timeout
			p.executeWithTimeout(ctx, workerID, j)
		}
	}
}

// executeWithTimeout executes a job with the configured timeout
func (p *Pool) executeWithTimeout(ctx context.Context, workerID int, j *job.Job) {
	// Recover from panics during job execution
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Worker %d: job %s panicked: %v", workerID, j.ID, r)
		}
	}()

	// Create context with timeout for job execution
	jobCtx, cancel := context.WithTimeout(ctx, p.jobTimeout)
	defer cancel()

	log.Printf("Worker %d: processing job %s (name: %s)", workerID, j.ID, j.Name)

	// Execute the job
	if err := p.executor.ExecuteJob(jobCtx, j); err != nil {
		log.Printf("Worker %d: job %s failed: %v", workerID, j.ID, err)
	} else {
		log.Printf("Worker %d: job %s completed", workerID, j.ID)
	}
}


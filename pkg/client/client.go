package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/muaviaUsmani/bananas/internal/queue"
	"github.com/muaviaUsmani/bananas/internal/result"
	"github.com/redis/go-redis/v9"
)

// Client provides a simple API for submitting and managing jobs
type Client struct {
	queue         *queue.RedisQueue
	resultBackend result.Backend
	ctx           context.Context
}

// NewClient creates a new job client connected to Redis
// The result backend is enabled by default with standard TTLs (1h success, 24h failure)
func NewClient(redisURL string) (*Client, error) {
	// Connect to Redis queue
	q, err := queue.NewRedisQueue(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Create Redis client for result backend
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}
	redisClient := redis.NewClient(opts)

	// Create result backend with default TTLs
	resultBackend := result.NewRedisBackend(redisClient, 1*time.Hour, 24*time.Hour)

	return &Client{
		queue:         q,
		resultBackend: resultBackend,
		ctx:           context.Background(),
	}, nil
}

// NewClientWithConfig creates a new job client with custom result backend TTLs
func NewClientWithConfig(redisURL string, successTTL, failureTTL time.Duration) (*Client, error) {
	// Connect to Redis queue
	q, err := queue.NewRedisQueue(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Create Redis client for result backend
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}
	redisClient := redis.NewClient(opts)

	// Create result backend with custom TTLs
	resultBackend := result.NewRedisBackend(redisClient, successTTL, failureTTL)

	return &Client{
		queue:         q,
		resultBackend: resultBackend,
		ctx:           context.Background(),
	}, nil
}

// SubmitJob creates and submits a new job with the given parameters.
// The payload will be marshaled to JSON automatically.
// Description is optional - if provided, the first value will be used.
// Returns the job ID on success.
func (c *Client) SubmitJob(name string, payload interface{}, priority job.JobPriority, description ...string) (string, error) {
	// Marshal payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create new job
	j := job.NewJob(name, payloadBytes, priority, description...)

	// Enqueue to Redis
	if err := c.queue.Enqueue(c.ctx, j); err != nil {
		return "", fmt.Errorf("failed to enqueue job: %w", err)
	}

	return j.ID, nil
}

// SubmitJobWithRoute creates and submits a new job with a specific routing key.
// The payload will be marshaled to JSON automatically.
// The routing key determines which workers will process this job.
// Description is optional - if provided, the first value will be used.
// Returns the job ID on success.
func (c *Client) SubmitJobWithRoute(name string, payload interface{}, priority job.JobPriority, routingKey string, description ...string) (string, error) {
	// Marshal payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create new job
	j := job.NewJob(name, payloadBytes, priority, description...)

	// Set routing key
	if err := j.SetRoutingKey(routingKey); err != nil {
		return "", fmt.Errorf("failed to set routing key: %w", err)
	}

	// Enqueue to Redis
	if err := c.queue.Enqueue(c.ctx, j); err != nil {
		return "", fmt.Errorf("failed to enqueue job: %w", err)
	}

	return j.ID, nil
}

// SubmitJobScheduled creates and submits a new job scheduled for future execution.
// The payload will be marshaled to JSON automatically.
// Description is optional - if provided, the first value will be used.
// Returns the job ID on success.
func (c *Client) SubmitJobScheduled(name string, payload interface{}, priority job.JobPriority, scheduledFor time.Time, description ...string) (string, error) {
	// Marshal payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create new job
	j := job.NewJob(name, payloadBytes, priority, description...)
	j.ScheduledFor = &scheduledFor
	j.Status = job.StatusScheduled

	// For scheduled jobs, we need to fail it once to trigger the scheduling mechanism
	// Or we can directly add to the scheduled set via Redis
	// For now, we'll enqueue it and immediately "fail" it to schedule it
	// A better approach would be to add a ScheduleJob method to the queue

	// For simplicity in this implementation, we'll add it to scheduled set directly
	// by using the internal queue method after enqueueing
	if err := c.queue.Enqueue(c.ctx, j); err != nil {
		return "", fmt.Errorf("failed to enqueue job: %w", err)
	}

	// Dequeue it immediately
	priorities := []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow}
	dequeuedJob, err := c.queue.Dequeue(c.ctx, priorities)
	if err != nil || dequeuedJob == nil || dequeuedJob.ID != j.ID {
		return "", fmt.Errorf("failed to schedule job: %w", err)
	}

	// Fail it with zero retries to put it in scheduled set
	if err := c.queue.Fail(c.ctx, dequeuedJob, "scheduled for later execution"); err != nil {
		return "", fmt.Errorf("failed to schedule job: %w", err)
	}

	return j.ID, nil
}

// GetJob retrieves a job by its ID from Redis
func (c *Client) GetJob(jobID string) (*job.Job, error) {
	j, err := c.queue.GetJob(c.ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	return j, nil
}

// GetResult retrieves the result of a completed job by its ID
// Returns nil if the job hasn't completed yet or if the result has expired
// Returns an error if retrieval fails
func (c *Client) GetResult(ctx context.Context, jobID string) (*job.JobResult, error) {
	result, err := c.resultBackend.GetResult(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get result: %w", err)
	}

	return result, nil
}

// SubmitAndWait submits a job and waits for its result
// This is a convenience method for RPC-style task execution
// Blocks until the job completes or the timeout is reached
// Returns the result if the job completes within the timeout
// Returns an error if the job fails, times out, or if submission fails
func (c *Client) SubmitAndWait(ctx context.Context, name string, payload interface{}, priority job.JobPriority, timeout time.Duration) (*job.JobResult, error) {
	// Submit the job
	jobID, err := c.SubmitJob(name, payload, priority)
	if err != nil {
		return nil, fmt.Errorf("failed to submit job: %w", err)
	}

	// Wait for the result
	result, err := c.resultBackend.WaitForResult(ctx, jobID, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for result: %w", err)
	}

	if result == nil {
		return nil, fmt.Errorf("job did not complete within timeout of %v", timeout)
	}

	return result, nil
}

// Close closes the Redis connections
func (c *Client) Close() error {
	var queueErr, resultErr error

	if c.queue != nil {
		queueErr = c.queue.Close()
	}

	if c.resultBackend != nil {
		resultErr = c.resultBackend.Close()
	}

	// Return first error encountered
	if queueErr != nil {
		return queueErr
	}
	return resultErr
}


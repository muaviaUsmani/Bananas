package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/muaviaUsmani/bananas/internal/queue"
)

// Client provides a simple API for submitting and managing jobs
type Client struct {
	queue *queue.RedisQueue
	ctx   context.Context
}

// NewClient creates a new job client connected to Redis
func NewClient(redisURL string) (*Client, error) {
	// Connect to Redis queue
	q, err := queue.NewRedisQueue(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Client{
		queue: q,
		ctx:   context.Background(),
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

// Close closes the Redis connection
func (c *Client) Close() error {
	if c.queue != nil {
		return c.queue.Close()
	}
	return nil
}


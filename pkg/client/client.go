package client

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/muaviaUsmani/bananas/internal/job"
)

// Client provides a simple API for submitting and managing jobs
type Client struct {
	jobs map[string]*job.Job
	mu   sync.RWMutex
}

// NewClient creates a new job client
func NewClient() *Client {
	return &Client{
		jobs: make(map[string]*job.Job),
	}
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

	// Store job
	c.mu.Lock()
	c.jobs[j.ID] = j
	c.mu.Unlock()

	return j.ID, nil
}

// GetJob retrieves a job by its ID
func (c *Client) GetJob(jobID string) (*job.Job, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	j, exists := c.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	return j, nil
}

// ListJobs returns all jobs
func (c *Client) ListJobs() []*job.Job {
	c.mu.RLock()
	defer c.mu.RUnlock()

	jobs := make([]*job.Job, 0, len(c.jobs))
	for _, j := range c.jobs {
		jobs = append(jobs, j)
	}

	return jobs
}


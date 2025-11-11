package job

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// JobStatus represents the current status of a job
type JobStatus string

const (
	// StatusPending indicates the job is waiting to be processed
	StatusPending JobStatus = "pending"
	// StatusProcessing indicates the job is currently being processed
	StatusProcessing JobStatus = "processing"
	// StatusCompleted indicates the job was successfully completed
	StatusCompleted JobStatus = "completed"
	// StatusFailed indicates the job failed and will not be retried
	StatusFailed JobStatus = "failed"
	// StatusScheduled indicates the job is scheduled for future execution
	StatusScheduled JobStatus = "scheduled"
)

// JobPriority represents the priority level of a job
type JobPriority string

const (
	// PriorityHigh indicates high priority jobs that should be processed first
	PriorityHigh JobPriority = "high"
	// PriorityNormal indicates normal priority jobs
	PriorityNormal JobPriority = "normal"
	// PriorityLow indicates low priority jobs that can be processed later
	PriorityLow JobPriority = "low"
)

// Job represents a unit of work to be processed by the task queue
type Job struct {
	// ID is the unique identifier for the job
	ID string `json:"id"`
	// Name identifies the kind of job to be executed
	Name string `json:"name"`
	// Description is an optional human-readable description of the job
	Description string `json:"description,omitempty"`
	// Payload contains the job-specific data in JSON format
	Payload json.RawMessage `json:"payload"`
	// Status is the current status of the job
	Status JobStatus `json:"status"`
	// Priority determines the processing order
	Priority JobPriority `json:"priority"`
	// CreatedAt is when the job was created
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when the job was last updated
	UpdatedAt time.Time `json:"updated_at"`
	// ScheduledFor is the time when a scheduled job should be executed (nullable)
	ScheduledFor *time.Time `json:"scheduled_for,omitempty"`
	// Attempts is the number of times the job has been attempted
	Attempts int `json:"attempts"`
	// MaxRetries is the maximum number of retry attempts allowed
	MaxRetries int `json:"max_retries"`
	// Error contains the error message if the job failed
	Error string `json:"error,omitempty"`
}

// NewJob creates a new job with the specified name, payload, priority, and optional description.
// The description parameter is optional - if provided, the first value will be used.
//
// Example usage:
//
//	job := NewJob("send_email", payload, PriorityNormal, "Send welcome email to new user")
//	job := NewJob("resize_image", payload, PriorityHigh)
func NewJob(name string, payload []byte, priority JobPriority, description ...string) *Job {
	now := time.Now()

	// Extract optional description (take first if provided)
	var desc string
	if len(description) > 0 {
		desc = description[0]
	}

	return &Job{
		ID:          uuid.New().String(),
		Name:        name,
		Description: desc,
		Payload:     payload,
		Status:      StatusPending,
		Priority:    priority,
		CreatedAt:   now,
		UpdatedAt:   now,
		Attempts:    0,
		MaxRetries:  3, // Default, can be overridden
		Error:       "",
	}
}

// UpdateStatus updates the job's status and UpdatedAt timestamp
func (j *Job) UpdateStatus(status JobStatus) {
	j.Status = status
	j.UpdatedAt = time.Now()
}

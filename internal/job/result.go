package job

import (
	"encoding/json"
	"time"
)

// JobResult represents the result of a completed job
type JobResult struct {
	// JobID is the unique identifier of the job
	JobID string `json:"job_id"`

	// Status is the final status of the job (completed or failed)
	Status JobStatus `json:"status"`

	// Result contains the job's return value (only for successful jobs)
	// This is the data returned by the job handler
	Result json.RawMessage `json:"result,omitempty"`

	// Error contains the error message if the job failed
	Error string `json:"error,omitempty"`

	// CompletedAt is when the job finished executing
	CompletedAt time.Time `json:"completed_at"`

	// Duration is how long the job took to execute
	Duration time.Duration `json:"duration"`
}

// IsSuccess returns true if the job completed successfully
func (r *JobResult) IsSuccess() bool {
	return r.Status == StatusCompleted
}

// IsFailed returns true if the job failed
func (r *JobResult) IsFailed() bool {
	return r.Status == StatusFailed
}

// UnmarshalResult unmarshals the result data into the provided destination
// Returns an error if the job failed or if unmarshaling fails
func (r *JobResult) UnmarshalResult(dest interface{}) error {
	if r.IsFailed() {
		return &ResultError{Message: r.Error}
	}

	if len(r.Result) == 0 {
		return nil // No result data
	}

	return json.Unmarshal(r.Result, dest)
}

// ResultError represents an error when retrieving or processing a result
type ResultError struct {
	Message string
}

func (e *ResultError) Error() string {
	return e.Message
}

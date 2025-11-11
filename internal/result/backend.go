// Package result provides backend interfaces and implementations for storing and retrieving job results.
package result

import (
	"context"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
)

// Backend defines the interface for storing and retrieving job results
type Backend interface {
	// StoreResult stores a job result in the backend
	// Returns an error if storage fails
	StoreResult(ctx context.Context, result *job.JobResult) error

	// GetResult retrieves a job result by job ID
	// Returns nil if the result doesn't exist (job not yet complete or result expired)
	// Returns an error if retrieval fails
	GetResult(ctx context.Context, jobID string) (*job.JobResult, error)

	// WaitForResult blocks until a result is available or the timeout is reached
	// Returns the result if available within the timeout
	// Returns nil and no error if the timeout is reached
	// Returns an error if waiting fails
	WaitForResult(ctx context.Context, jobID string, timeout time.Duration) (*job.JobResult, error)

	// DeleteResult removes a result from the backend
	// Returns an error if deletion fails
	// Does not error if the result doesn't exist
	DeleteResult(ctx context.Context, jobID string) error

	// Close closes any connections used by the backend
	Close() error
}

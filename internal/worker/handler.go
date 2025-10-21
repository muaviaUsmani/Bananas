package worker

import (
	"context"
	"fmt"

	"github.com/muaviaUsmani/bananas/internal/job"
)

// HandlerFunc is a function that processes a job
type HandlerFunc func(context.Context, *job.Job) error

// Registry manages job handlers by name
type Registry struct {
	handlers map[string]HandlerFunc
}

// NewRegistry creates a new handler registry
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]HandlerFunc),
	}
}

// Register adds a handler for a specific job name
func (r *Registry) Register(name string, handler HandlerFunc) {
	r.handlers[name] = handler
}

// Get retrieves a handler by job name. Returns the handler and a boolean indicating if it exists.
func (r *Registry) Get(name string) (HandlerFunc, bool) {
	handler, exists := r.handlers[name]
	return handler, exists
}

// Count returns the number of registered handlers
func (r *Registry) Count() int {
	return len(r.handlers)
}

// Execute runs the appropriate handler for a job
func (r *Registry) Execute(ctx context.Context, j *job.Job) error {
	handler, exists := r.Get(j.Name)
	if !exists {
		return fmt.Errorf("no handler registered for job: %s", j.Name)
	}
	return handler(ctx, j)
}


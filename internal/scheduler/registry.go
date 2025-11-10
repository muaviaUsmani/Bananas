package scheduler

import (
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

var (
	// scheduleIDPattern validates schedule IDs (alphanumeric, underscores, hyphens)
	scheduleIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// Registry stores and manages periodic schedules
type Registry struct {
	mu        sync.RWMutex
	schedules map[string]*Schedule
	parser    cron.Parser
}

// NewRegistry creates a new schedule registry
func NewRegistry() *Registry {
	return &Registry{
		schedules: make(map[string]*Schedule),
		parser:    cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
	}
}

// Register adds a schedule to the registry
func (r *Registry) Register(schedule *Schedule) error {
	// Validate schedule
	if err := r.validate(schedule); err != nil {
		return fmt.Errorf("invalid schedule: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicate ID
	if _, exists := r.schedules[schedule.ID]; exists {
		return fmt.Errorf("schedule with ID %s already exists", schedule.ID)
	}

	// Set defaults
	if schedule.Timezone == "" {
		schedule.Timezone = "UTC"
	}

	r.schedules[schedule.ID] = schedule
	return nil
}

// MustRegister registers a schedule, panicking on error
// Useful for initialization-time schedule registration
func (r *Registry) MustRegister(schedule *Schedule) {
	if err := r.Register(schedule); err != nil {
		panic(fmt.Sprintf("failed to register schedule: %v", err))
	}
}

// Get retrieves a schedule by ID
func (r *Registry) Get(id string) (*Schedule, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, exists := r.schedules[id]
	return s, exists
}

// List returns all registered schedules
func (r *Registry) List() []*Schedule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	schedules := make([]*Schedule, 0, len(r.schedules))
	for _, s := range r.schedules {
		schedules = append(schedules, s)
	}
	return schedules
}

// Count returns the number of registered schedules
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.schedules)
}

// NextRun calculates the next run time for a schedule
func (r *Registry) NextRun(schedule *Schedule, after time.Time) (time.Time, error) {
	cronSchedule, err := r.parser.Parse(schedule.Cron)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse cron expression: %w", err)
	}

	// Load timezone
	loc := time.UTC
	if schedule.Timezone != "" && schedule.Timezone != "UTC" {
		loc, err = time.LoadLocation(schedule.Timezone)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid timezone %s: %w", schedule.Timezone, err)
		}
	}

	// Calculate next run in the schedule's timezone
	afterInTz := after.In(loc)
	next := cronSchedule.Next(afterInTz)
	return next, nil
}

// validate validates a schedule
func (r *Registry) validate(schedule *Schedule) error {
	// Validate ID
	if schedule.ID == "" {
		return fmt.Errorf("schedule ID cannot be empty")
	}
	if !scheduleIDPattern.MatchString(schedule.ID) {
		return fmt.Errorf("schedule ID must contain only alphanumeric characters, underscores, and hyphens")
	}

	// Validate cron expression
	if schedule.Cron == "" {
		return fmt.Errorf("cron expression cannot be empty")
	}
	if _, err := r.parser.Parse(schedule.Cron); err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", schedule.Cron, err)
	}

	// Validate job name
	if schedule.Job == "" {
		return fmt.Errorf("job name cannot be empty")
	}

	// Validate timezone (if specified)
	if schedule.Timezone != "" && schedule.Timezone != "UTC" {
		if _, err := time.LoadLocation(schedule.Timezone); err != nil {
			return fmt.Errorf("invalid timezone %q: %w", schedule.Timezone, err)
		}
	}

	// Validate priority (if specified, must be valid)
	if schedule.Priority != "" {
		validPriorities := map[string]bool{
			string(job.PriorityHigh):   true,
			string(job.PriorityNormal): true,
			string(job.PriorityLow):    true,
		}
		if !validPriorities[string(schedule.Priority)] {
			return fmt.Errorf("invalid priority %q", schedule.Priority)
		}
	}

	return nil
}

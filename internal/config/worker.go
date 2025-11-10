package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
)

// WorkerMode defines the operational mode of a worker process
type WorkerMode string

const (
	// WorkerModeThin is a single-process worker handling all queues
	// Use for: development, testing, very low traffic (<100 jobs/hour)
	WorkerModeThin WorkerMode = "thin"

	// WorkerModeDefault is the standard priority-aware worker
	// Use for: standard production (1K-10K jobs/hour)
	WorkerModeDefault WorkerMode = "default"

	// WorkerModeSpecialized is a worker dedicated to specific priority queues
	// Use for: high traffic with priority isolation (10K+ jobs/hour)
	WorkerModeSpecialized WorkerMode = "specialized"

	// WorkerModeJobSpecialized is a worker handling specific job types only
	// Use for: different resource requirements per job type
	WorkerModeJobSpecialized WorkerMode = "job-specialized"

	// WorkerModeSchedulerOnly runs only the scheduler (no job execution)
	// Use for: dedicated scheduler process in distributed setup
	WorkerModeSchedulerOnly WorkerMode = "scheduler-only"
)

// WorkerConfig holds worker-specific configuration
type WorkerConfig struct {
	// Mode determines the operational mode of the worker
	Mode WorkerMode

	// Concurrency is the number of concurrent worker goroutines
	// Recommended ranges by mode:
	//   - thin: 1-10
	//   - default: 10-50
	//   - specialized: 10-100 (depends on priority)
	//   - job-specialized: depends on job type
	//   - scheduler-only: 0 (no workers)
	Concurrency int

	// Priorities specifies which priority queues this worker should process
	// Empty slice means all priorities (high, normal, low)
	// Example: []job.PriorityHigh means only process high priority jobs
	Priorities []job.JobPriority

	// RoutingKeys specifies which routing keys this worker should handle
	// Examples: ["default"], ["gpu"], ["gpu", "default"]
	// Workers will process jobs from these routing keys in order (first has priority)
	// Defaults to ["default"] if not specified
	RoutingKeys []string

	// JobTypes specifies which job types this worker should handle
	// Empty slice means all job types
	// Only applicable in job-specialized mode
	// Example: ["send_email", "generate_report"]
	JobTypes []string

	// SchedulerInterval is how often to check for scheduled jobs
	// Default: 1 second
	SchedulerInterval time.Duration

	// EnableScheduler determines whether to run the scheduler loop
	// True for all modes except when you have a dedicated scheduler-only worker
	EnableScheduler bool
}

// LoadWorkerConfig loads worker configuration from environment variables
func LoadWorkerConfig() (*WorkerConfig, error) {
	cfg := &WorkerConfig{
		Mode:              WorkerMode(getEnv("WORKER_MODE", string(WorkerModeDefault))),
		Concurrency:       getEnvAsInt("WORKER_CONCURRENCY", 10),
		Priorities:        parsePriorities(getEnv("WORKER_PRIORITIES", "")),
		RoutingKeys:       getEnvAsStringSlice("WORKER_ROUTING_KEYS", []string{"default"}),
		JobTypes:          parseJobTypes(getEnv("WORKER_JOB_TYPES", "")),
		SchedulerInterval: getEnvAsDuration("SCHEDULER_INTERVAL", 1*time.Second),
		EnableScheduler:   getEnvAsBool("ENABLE_SCHEDULER", true),
	}

	// Apply mode-specific defaults
	cfg.applyModeDefaults()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// applyModeDefaults applies sensible defaults based on the worker mode
func (c *WorkerConfig) applyModeDefaults() {
	switch c.Mode {
	case WorkerModeThin:
		// Thin mode: low concurrency, all priorities, scheduler enabled
		if c.Concurrency == 10 { // If user didn't override
			c.Concurrency = 5
		}
		if len(c.Priorities) == 0 {
			c.Priorities = allPriorities()
		}
		c.EnableScheduler = true

	case WorkerModeDefault:
		// Default mode: medium concurrency, all priorities, scheduler enabled
		if len(c.Priorities) == 0 {
			c.Priorities = allPriorities()
		}
		if !getEnvAsBool("ENABLE_SCHEDULER", false) {
			// Only enable scheduler if explicitly set or not in distributed setup
			c.EnableScheduler = true
		}

	case WorkerModeSpecialized:
		// Specialized mode: MUST specify priorities
		if len(c.Priorities) == 0 {
			// Default to high priority if not specified
			c.Priorities = []job.JobPriority{job.PriorityHigh}
		}
		// Disable scheduler by default (use dedicated scheduler-only worker)
		if getEnv("ENABLE_SCHEDULER", "") == "" {
			c.EnableScheduler = false
		}

	case WorkerModeJobSpecialized:
		// Job-specialized mode: MUST specify job types
		// Priorities default to all
		if len(c.Priorities) == 0 {
			c.Priorities = allPriorities()
		}
		// Disable scheduler by default
		if getEnv("ENABLE_SCHEDULER", "") == "" {
			c.EnableScheduler = false
		}

	case WorkerModeSchedulerOnly:
		// Scheduler-only mode: no workers, only scheduler
		c.Concurrency = 0
		c.Priorities = nil
		c.JobTypes = nil
		c.EnableScheduler = true
	}
}

// Validate checks if the worker configuration is valid
func (c *WorkerConfig) Validate() error {
	// Validate mode
	validModes := []WorkerMode{
		WorkerModeThin,
		WorkerModeDefault,
		WorkerModeSpecialized,
		WorkerModeJobSpecialized,
		WorkerModeSchedulerOnly,
	}
	validMode := false
	for _, mode := range validModes {
		if c.Mode == mode {
			validMode = true
			break
		}
	}
	if !validMode {
		return fmt.Errorf("invalid worker mode: %s (must be one of: thin, default, specialized, job-specialized, scheduler-only)", c.Mode)
	}

	// Validate concurrency
	if c.Mode != WorkerModeSchedulerOnly {
		if c.Concurrency < 1 {
			return fmt.Errorf("worker concurrency must be at least 1 (got %d)", c.Concurrency)
		}
		if c.Concurrency > 1000 {
			return fmt.Errorf("worker concurrency too high: %d (maximum 1000)", c.Concurrency)
		}
	} else {
		// Scheduler-only must have 0 concurrency
		if c.Concurrency != 0 {
			return fmt.Errorf("scheduler-only mode must have concurrency=0 (got %d)", c.Concurrency)
		}
	}

	// Validate priorities
	if c.Mode != WorkerModeSchedulerOnly {
		if len(c.Priorities) == 0 {
			return fmt.Errorf("worker must process at least one priority queue")
		}
		// Validate each priority value
		for _, p := range c.Priorities {
			if p != job.PriorityHigh && p != job.PriorityNormal && p != job.PriorityLow {
				return fmt.Errorf("invalid priority: %s", p)
			}
		}
	}

	// Validate job types for job-specialized mode
	if c.Mode == WorkerModeJobSpecialized {
		if len(c.JobTypes) == 0 {
			return fmt.Errorf("job-specialized mode requires at least one job type to be specified")
		}
		// Validate no empty job type names
		for _, jt := range c.JobTypes {
			if strings.TrimSpace(jt) == "" {
				return fmt.Errorf("job type cannot be empty")
			}
		}
	}

	// Validate scheduler interval
	if c.EnableScheduler {
		if c.SchedulerInterval < 100*time.Millisecond {
			return fmt.Errorf("scheduler interval too short: %v (minimum 100ms)", c.SchedulerInterval)
		}
		if c.SchedulerInterval > 1*time.Minute {
			return fmt.Errorf("scheduler interval too long: %v (maximum 1 minute)", c.SchedulerInterval)
		}
	}

	return nil
}

// ShouldProcessJob checks if this worker should process a given job
// based on its configuration (priorities, job types)
func (c *WorkerConfig) ShouldProcessJob(j *job.Job) bool {
	// Check priority filter
	if len(c.Priorities) > 0 {
		priorityMatch := false
		for _, p := range c.Priorities {
			if j.Priority == p {
				priorityMatch = true
				break
			}
		}
		if !priorityMatch {
			return false
		}
	}

	// Check job type filter (only for job-specialized mode)
	if c.Mode == WorkerModeJobSpecialized && len(c.JobTypes) > 0 {
		jobTypeMatch := false
		for _, jt := range c.JobTypes {
			if j.Name == jt {
				jobTypeMatch = true
				break
			}
		}
		if !jobTypeMatch {
			return false
		}
	}

	return true
}

// String returns a human-readable description of the worker config
func (c *WorkerConfig) String() string {
	priorities := "all"
	if len(c.Priorities) > 0 && len(c.Priorities) < 3 {
		parts := make([]string, len(c.Priorities))
		for i, p := range c.Priorities {
			parts[i] = string(p)
		}
		priorities = strings.Join(parts, ",")
	}

	jobTypes := "all"
	if len(c.JobTypes) > 0 {
		if len(c.JobTypes) <= 3 {
			jobTypes = strings.Join(c.JobTypes, ",")
		} else {
			jobTypes = fmt.Sprintf("%s... (%d types)", strings.Join(c.JobTypes[:3], ","), len(c.JobTypes))
		}
	}

	scheduler := "disabled"
	if c.EnableScheduler {
		scheduler = fmt.Sprintf("enabled (interval: %v)", c.SchedulerInterval)
	}

	return fmt.Sprintf(
		"WorkerConfig{mode=%s, concurrency=%d, priorities=%s, jobTypes=%s, scheduler=%s}",
		c.Mode, c.Concurrency, priorities, jobTypes, scheduler,
	)
}

// parsePriorities parses a comma-separated string of priorities
// Empty string returns nil (will use defaults)
func parsePriorities(s string) []job.JobPriority {
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	priorities := make([]job.JobPriority, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(strings.ToLower(part))
		switch trimmed {
		case "high":
			priorities = append(priorities, job.PriorityHigh)
		case "normal":
			priorities = append(priorities, job.PriorityNormal)
		case "low":
			priorities = append(priorities, job.PriorityLow)
		}
	}

	return priorities
}

// parseJobTypes parses a comma-separated string of job types
// Empty string returns nil (all job types)
func parseJobTypes(s string) []string {
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	jobTypes := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			jobTypes = append(jobTypes, trimmed)
		}
	}

	if len(jobTypes) == 0 {
		return nil
	}

	return jobTypes
}

// allPriorities returns all three priorities in order
func allPriorities() []job.JobPriority {
	return []job.JobPriority{
		job.PriorityHigh,
		job.PriorityNormal,
		job.PriorityLow,
	}
}

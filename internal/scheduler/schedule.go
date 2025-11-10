package scheduler

import (
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
)

// Schedule represents a periodic task schedule
type Schedule struct {
	// ID is a unique identifier for the schedule
	ID string

	// Cron expression (standard 5-field: minute hour day month weekday)
	// Examples:
	//   "0 * * * *"     - Every hour at minute 0
	//   "*/15 * * * *"  - Every 15 minutes
	//   "0 9 * * 1"     - Every Monday at 9:00 AM
	//   "0 0 1 * *"     - First day of every month at midnight
	Cron string

	// Job name (must be registered with worker)
	Job string

	// Payload is the job payload (JSON bytes)
	Payload []byte

	// Priority for the enqueued job
	Priority job.JobPriority

	// Timezone for cron evaluation (default: UTC)
	// Must be a valid IANA timezone (e.g., "America/New_York", "UTC")
	Timezone string

	// Enabled flag (allows disabling without removing)
	Enabled bool

	// Description for logging/monitoring
	Description string
}

// ScheduleState represents the runtime state of a schedule
type ScheduleState struct {
	ID          string
	LastRun     time.Time
	NextRun     time.Time
	RunCount    int64
	LastError   string
	LastSuccess time.Time
}

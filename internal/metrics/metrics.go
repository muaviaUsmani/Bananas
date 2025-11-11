package metrics

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
)

// Collector is the global metrics collector instance
var (
	globalCollector *Collector
	once            sync.Once
)

// Collector tracks system-wide metrics in memory
type Collector struct {
	// Counters (atomic for thread-safety)
	totalJobsProcessed atomic.Int64
	totalJobsCompleted atomic.Int64
	totalJobsFailed    atomic.Int64

	// Job tracking by status and priority (protected by mutex)
	mu             sync.RWMutex
	jobsByStatus   map[job.JobStatus]int64
	jobsByPriority map[job.JobPriority]int64
	queueDepths    map[job.JobPriority]int64
	totalDuration  time.Duration
	startTime      time.Time
	activeWorkers  int64
	totalWorkers   int64
	errorCount     int64
	operationCount int64
}

// Metrics represents a snapshot of current system metrics
type Metrics struct {
	TotalJobsProcessed int64                     `json:"total_jobs_processed"`
	TotalJobsCompleted int64                     `json:"total_jobs_completed"`
	TotalJobsFailed    int64                     `json:"total_jobs_failed"`
	JobsByStatus       map[job.JobStatus]int64   `json:"jobs_by_status"`
	JobsByPriority     map[job.JobPriority]int64 `json:"jobs_by_priority"`
	QueueDepths        map[job.JobPriority]int64 `json:"queue_depths"`
	AvgJobDuration     time.Duration             `json:"avg_job_duration"`
	WorkerUtilization  float64                   `json:"worker_utilization"`
	ErrorRate          float64                   `json:"error_rate"`
	Uptime             time.Duration             `json:"uptime"`
}

// Default returns the global metrics collector instance
func Default() *Collector {
	once.Do(func() {
		globalCollector = NewCollector()
	})
	return globalCollector
}

// NewCollector creates a new metrics collector
func NewCollector() *Collector {
	return &Collector{
		jobsByStatus:   make(map[job.JobStatus]int64),
		jobsByPriority: make(map[job.JobPriority]int64),
		queueDepths:    make(map[job.JobPriority]int64),
		startTime:      time.Now(),
	}
}

// RecordJobStarted increments the jobs processed counter
func (c *Collector) RecordJobStarted(priority job.JobPriority) {
	c.totalJobsProcessed.Add(1)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.jobsByPriority[priority]++
	c.jobsByStatus[job.StatusProcessing]++
}

// RecordJobCompleted records a successfully completed job
func (c *Collector) RecordJobCompleted(_ job.JobPriority, duration time.Duration) {
	c.totalJobsCompleted.Add(1)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.jobsByStatus[job.StatusProcessing]--
	c.jobsByStatus[job.StatusCompleted]++
	c.totalDuration += duration
	c.operationCount++
}

// RecordJobFailed records a failed job
func (c *Collector) RecordJobFailed(_ job.JobPriority, duration time.Duration) {
	c.totalJobsFailed.Add(1)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.jobsByStatus[job.StatusProcessing]--
	c.jobsByStatus[job.StatusFailed]++
	c.totalDuration += duration
	c.operationCount++
	c.errorCount++
}

// RecordQueueDepth updates the current queue depth for a priority
func (c *Collector) RecordQueueDepth(priority job.JobPriority, depth int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queueDepths[priority] = depth
}

// RecordWorkerActivity updates worker utilization metrics
func (c *Collector) RecordWorkerActivity(active, total int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activeWorkers = active
	c.totalWorkers = total
}

// GetMetrics returns a snapshot of current metrics
func (c *Collector) GetMetrics() Metrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Create copies of maps
	jobsByStatus := make(map[job.JobStatus]int64, len(c.jobsByStatus))
	for k, v := range c.jobsByStatus {
		jobsByStatus[k] = v
	}

	jobsByPriority := make(map[job.JobPriority]int64, len(c.jobsByPriority))
	for k, v := range c.jobsByPriority {
		jobsByPriority[k] = v
	}

	queueDepths := make(map[job.JobPriority]int64, len(c.queueDepths))
	for k, v := range c.queueDepths {
		queueDepths[k] = v
	}

	// Calculate average duration
	var avgDuration time.Duration
	if c.operationCount > 0 {
		avgDuration = c.totalDuration / time.Duration(c.operationCount)
	}

	// Calculate worker utilization
	var utilization float64
	if c.totalWorkers > 0 {
		utilization = float64(c.activeWorkers) / float64(c.totalWorkers) * 100
	}

	// Calculate error rate
	var errorRate float64
	totalOps := c.operationCount
	if totalOps > 0 {
		errorRate = float64(c.errorCount) / float64(totalOps) * 100
	}

	return Metrics{
		TotalJobsProcessed: c.totalJobsProcessed.Load(),
		TotalJobsCompleted: c.totalJobsCompleted.Load(),
		TotalJobsFailed:    c.totalJobsFailed.Load(),
		JobsByStatus:       jobsByStatus,
		JobsByPriority:     jobsByPriority,
		QueueDepths:        queueDepths,
		AvgJobDuration:     avgDuration,
		WorkerUtilization:  utilization,
		ErrorRate:          errorRate,
		Uptime:             time.Since(c.startTime),
	}
}

// Reset clears all metrics (useful for testing)
func (c *Collector) Reset() {
	c.totalJobsProcessed.Store(0)
	c.totalJobsCompleted.Store(0)
	c.totalJobsFailed.Store(0)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.jobsByStatus = make(map[job.JobStatus]int64)
	c.jobsByPriority = make(map[job.JobPriority]int64)
	c.queueDepths = make(map[job.JobPriority]int64)
	c.totalDuration = 0
	c.startTime = time.Now()
	c.activeWorkers = 0
	c.totalWorkers = 0
	c.errorCount = 0
	c.operationCount = 0
}

// GetMetrics returns metrics from the global collector
func GetMetrics() Metrics {
	return Default().GetMetrics()
}

// ResetMetrics resets the global collector
func ResetMetrics() {
	Default().Reset()
}

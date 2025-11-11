package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/muaviaUsmani/bananas/internal/metrics"
	"github.com/redis/go-redis/v9"
)

// RedisQueue manages job queues in Redis
type RedisQueue struct {
	client    *redis.Client
	keyPrefix string
	// Pre-computed keys for better performance (avoid string allocations)
	queueHighKey    string
	queueNormalKey  string
	queueLowKey     string
	processingKey   string
	deadLetterKey   string
	scheduledSetKey string
	// TTL configuration for job data retention
	completedJobTTL time.Duration // TTL for completed jobs (default: 24 hours)
	failedJobTTL    time.Duration // TTL for failed jobs in dead letter queue (default: 7 days)
}

// NewRedisQueue creates a new Redis queue and tests the connection
func NewRedisQueue(redisURL string) (*RedisQueue, error) {
	// Parse Redis URL
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	// Configure connection pool for job queue workload
	// These settings are optimized for:
	// - Multiple concurrent workers (default 10, configurable up to 50+)
	// - API server handling enqueue requests
	// - Scheduler moving jobs from scheduled set
	// - Long-lived connections with blocking operations (BRPOPLPUSH)
	//
	// Pool size calculation: workers + API concurrency + scheduler + buffer
	// Example: 10 workers + 10 API + 1 scheduler + 5 buffer = 26 connections
	opts.PoolSize = 50                      // Maximum connections in pool (handles up to ~40 workers)
	opts.MinIdleConns = 5                   // Keep 5 idle connections ready (reduces connection setup latency)
	opts.ConnMaxIdleTime = 10 * time.Minute // Close idle connections after 10 minutes
	opts.PoolTimeout = 5 * time.Second      // Wait up to 5 seconds for connection from pool

	// Retry configuration for transient failures
	opts.MaxRetries = 3                           // Retry failed commands up to 3 times
	opts.MinRetryBackoff = 8 * time.Millisecond   // Minimum 8ms between retries
	opts.MaxRetryBackoff = 512 * time.Millisecond // Maximum 512ms between retries
	opts.DialTimeout = 5 * time.Second            // Timeout for establishing connection
	opts.ReadTimeout = 10 * time.Second           // Longer timeout for blocking operations (BRPOPLPUSH)
	opts.WriteTimeout = 3 * time.Second           // Timeout for write operations
	opts.ContextTimeoutEnabled = true             // Respect context timeouts

	// Create client with optimized options
	client := redis.NewClient(opts)

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Printf("Connected to Redis at %s (pool: %d max, %d min idle)",
		redisURL, opts.PoolSize, opts.MinIdleConns)

	prefix := "bananas:"
	return &RedisQueue{
		client:    client,
		keyPrefix: prefix,
		// Pre-compute all static keys once to avoid repeated string allocations
		queueHighKey:    prefix + "queue:high",
		queueNormalKey:  prefix + "queue:normal",
		queueLowKey:     prefix + "queue:low",
		processingKey:   prefix + "queue:processing",
		deadLetterKey:   prefix + "queue:dead",
		scheduledSetKey: prefix + "queue:scheduled",
		// Set default TTL values for job data retention
		// These prevent Redis from growing unbounded with old job data
		completedJobTTL: 24 * time.Hour,     // Keep completed jobs for 24 hours
		failedJobTTL:    7 * 24 * time.Hour, // Keep failed jobs for 7 days
	}, nil
}

// Key generation helpers
func (q *RedisQueue) jobKey(jobID string) string {
	// Use strings.Builder for efficient string concatenation
	var b strings.Builder
	b.Grow(len(q.keyPrefix) + 4 + len(jobID)) // "job:" = 4 chars
	b.WriteString(q.keyPrefix)
	b.WriteString("job:")
	b.WriteString(jobID)
	return b.String()
}

func (q *RedisQueue) queueKey(priority job.JobPriority) string {
	// Return pre-computed queue keys based on priority
	switch priority {
	case job.PriorityHigh:
		return q.queueHighKey
	case job.PriorityNormal:
		return q.queueNormalKey
	case job.PriorityLow:
		return q.queueLowKey
	default:
		return q.queueNormalKey
	}
}

func (q *RedisQueue) processingQueueKey() string {
	return q.processingKey
}

func (q *RedisQueue) deadLetterQueueKey() string {
	return q.deadLetterKey
}

func (q *RedisQueue) getScheduledSetKey() string {
	return q.scheduledSetKey
}

// Enqueue adds a job to the appropriate priority queue
func (q *RedisQueue) Enqueue(ctx context.Context, j *job.Job) error {
	// Serialize job to JSON
	jobData, err := json.Marshal(j)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// Use pipeline for atomic operations
	pipe := q.client.Pipeline()

	// Store job data in hash
	pipe.Set(ctx, q.jobKey(j.ID), jobData, 0)

	// Push job ID to priority queue
	pipe.LPush(ctx, q.queueKey(j.Priority), j.ID)

	// Execute pipeline
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	log.Printf("Enqueued job %s to %s queue", j.ID, j.Priority)

	// Update queue depth metrics (best-effort, don't fail enqueue on error)
	q.updateQueueMetrics(ctx)

	return nil
}

// Dequeue retrieves a job from the highest priority non-empty queue using blocking operations
//
// Implementation Strategy:
// Uses BRPOPLPUSH (blocking right-pop + left-push) to eliminate busy-waiting while maintaining
// strict priority ordering. Each priority queue is checked in order with a timeout:
// - High priority: 1 second timeout
// - Normal priority: 1 second timeout
// - Low priority: 3 seconds timeout
//
// This approach:
// - Eliminates the 100ms polling sleep in worker loops
// - Maintains strict priority ordering (high jobs always processed first)
// - Reduces Redis CPU usage by using native blocking operations
// - Allows graceful shutdown via context cancellation
//
// Trade-off: A high-priority job arriving while blocking on low-priority queue may wait
// up to 3 seconds. This is acceptable for most workloads and far better than continuous polling.
func (q *RedisQueue) Dequeue(ctx context.Context, priorities []job.JobPriority) (*job.Job, error) {
	processingKey := q.processingQueueKey()

	// Try each priority queue in order with blocking operations
	for i, priority := range priorities {
		queueKey := q.queueKey(priority)

		// Calculate timeout based on priority and position
		// High and normal get 1s, low gets longer timeout since it's checked last
		var timeout time.Duration
		if i == len(priorities)-1 {
			// Last priority queue gets longer timeout (3 seconds)
			timeout = 3 * time.Second
		} else {
			// Higher priority queues get shorter timeout (1 second)
			timeout = 1 * time.Second
		}

		// Use BRPOPLPUSH for blocking dequeue with timeout
		// This atomically moves job from priority queue to processing queue
		result, err := q.client.BRPopLPush(ctx, queueKey, processingKey, timeout).Result()
		if err == redis.Nil {
			// Queue is empty after timeout, try next priority
			continue
		}
		if err != nil {
			// Check if context was cancelled
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("failed to dequeue job: %w", err)
		}

		jobID := result

		// Retrieve job data
		jobData, err := q.client.Get(ctx, q.jobKey(jobID)).Result()
		if err != nil {
			// Job data not found (corrupted reference) - move to dead letter queue
			log.Printf("ERROR: Job data not found for ID %s (corrupted reference) - moving to dead letter queue", jobID)

			pipe := q.client.Pipeline()
			pipe.LPush(ctx, q.deadLetterQueueKey(), jobID)
			pipe.LRem(ctx, processingKey, 1, jobID)
			// Store error info as job data with TTL
			errorJob := map[string]interface{}{
				"id":    jobID,
				"error": "Job data not found (corrupted reference)",
			}
			errorData, err := json.Marshal(errorJob)
			if err != nil {
				log.Printf("ERROR: Failed to marshal error job data: %v", err)
			} else {
				pipe.Set(ctx, q.jobKey(jobID), errorData, q.failedJobTTL)
			}
			if _, err := pipe.Exec(ctx); err != nil {
				log.Printf("ERROR: Failed to execute pipeline for corrupted job: %v", err)
			}

			// Skip this corrupted job and continue processing other jobs
			continue
		}

		// Deserialize job
		var j job.Job
		if err := json.Unmarshal([]byte(jobData), &j); err != nil {
			// Invalid/corrupted job data - move to dead letter queue WITHOUT RETRY
			log.Printf("ERROR: Failed to unmarshal job %s (corrupted data) - moving to dead letter queue", jobID)
			log.Printf("Corrupted job data (first 200 chars): %s", truncate(jobData, 200))

			pipe := q.client.Pipeline()
			pipe.LPush(ctx, q.deadLetterQueueKey(), jobID)
			pipe.LRem(ctx, processingKey, 1, jobID)
			// Update job data to mark as corrupted with TTL
			errorJob := map[string]interface{}{
				"id":             jobID,
				"error":          fmt.Sprintf("Failed to unmarshal job: %v", err),
				"corrupted_data": truncate(jobData, 500), // Store truncated data for debugging
			}
			errorData, err := json.Marshal(errorJob)
			if err != nil {
				log.Printf("ERROR: Failed to marshal error job data: %v", err)
			} else {
				pipe.Set(ctx, q.jobKey(jobID), errorData, q.failedJobTTL)
			}
			if _, err := pipe.Exec(ctx); err != nil {
				log.Printf("ERROR: Failed to execute pipeline for unmarshal error: %v", err)
			}

			// Skip this corrupted job and continue processing other jobs
			continue
		}

		log.Printf("Dequeued job %s from %s queue", j.ID, priority)
		return &j, nil
	}

	// All queues are empty after checking with timeouts
	return nil, nil
}

// Complete marks a job as completed and removes it from the processing queue
func (q *RedisQueue) Complete(ctx context.Context, jobID string) error {
	// Retrieve job data first (must be done before updates)
	jobData, err := q.client.Get(ctx, q.jobKey(jobID)).Result()
	if err != nil {
		return fmt.Errorf("failed to get job data: %w", err)
	}

	var j job.Job
	if err := json.Unmarshal([]byte(jobData), &j); err != nil {
		return fmt.Errorf("failed to unmarshal job: %w", err)
	}

	j.UpdateStatus(job.StatusCompleted)

	updatedData, err := json.Marshal(j)
	if err != nil {
		return fmt.Errorf("failed to marshal updated job: %w", err)
	}

	// Use pipeline to batch removal from processing queue and status update
	// This reduces 2 round trips to 1
	// Set TTL on completed job data to prevent unbounded Redis growth
	pipe := q.client.Pipeline()
	pipe.LRem(ctx, q.processingQueueKey(), 1, jobID)
	pipe.Set(ctx, q.jobKey(jobID), updatedData, q.completedJobTTL)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	log.Printf("Completed job %s (TTL: %v)", jobID, q.completedJobTTL)
	return nil
}

// Fail handles a failed job with exponential backoff retry or moves to dead letter queue
//
// Retry Strategy:
// Instead of immediately re-enqueuing failed jobs to priority queues, we use a scheduled set
// with exponential backoff. This prevents:
// - Thundering herd problems when external services fail
// - Overwhelming failing dependencies with retry storms
// - Priority queue pollution with repeatedly failing jobs
//
// Exponential Backoff Calculation:
// - 1st retry: 2^1 = 2 seconds
// - 2nd retry: 2^2 = 4 seconds
// - 3rd retry: 2^3 = 8 seconds
// - Nth retry: 2^N seconds
//
// Jobs are stored in a Redis sorted set (ZSET) with the retry timestamp as the score.
// A background process (scheduler) periodically calls MoveScheduledToReady() to move
// jobs from the scheduled set back to their priority queues when ready.
func (q *RedisQueue) Fail(ctx context.Context, j *job.Job, errMsg string) error {
	// Update job state
	j.Attempts++
	j.Error = errMsg

	pipe := q.client.Pipeline()

	// Check if we should retry
	if j.Attempts < j.MaxRetries {
		// Calculate exponential backoff delay: 2^attempts seconds
		delaySecs := 1 << j.Attempts // Bit shift: 2^attempts
		retryDelay := time.Duration(delaySecs) * time.Second
		nextRetryTime := time.Now().Add(retryDelay)

		// Update job for retry
		j.UpdateStatus(job.StatusPending)
		j.ScheduledFor = &nextRetryTime

		// Serialize updated job
		jobData, err := json.Marshal(j)
		if err != nil {
			return fmt.Errorf("failed to marshal job: %w", err)
		}

		// Update job data in Redis
		pipe.Set(ctx, q.jobKey(j.ID), jobData, 0)

		// Add to scheduled set with retry time as score
		pipe.ZAdd(ctx, q.getScheduledSetKey(), redis.Z{
			Score:  float64(nextRetryTime.Unix()),
			Member: j.ID,
		})

		// Remove from processing queue
		pipe.LRem(ctx, q.processingQueueKey(), 1, j.ID)

		if _, err := pipe.Exec(ctx); err != nil {
			return fmt.Errorf("failed to schedule job for retry: %w", err)
		}

		log.Printf("Job %s failed (attempt %d/%d), scheduled for retry in %v at %s",
			j.ID, j.Attempts, j.MaxRetries, retryDelay, nextRetryTime.Format(time.RFC3339))
		return nil
	}

	// Max retries exceeded, move to dead letter queue
	j.UpdateStatus(job.StatusFailed)
	j.ScheduledFor = nil // Clear scheduled time

	jobData, err := json.Marshal(j)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// Update job data with TTL to prevent unbounded growth of failed jobs
	pipe.Set(ctx, q.jobKey(j.ID), jobData, q.failedJobTTL)

	// Move to dead letter queue
	pipe.LPush(ctx, q.deadLetterQueueKey(), j.ID)

	// Remove from processing queue
	pipe.LRem(ctx, q.processingQueueKey(), 1, j.ID)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to move job to dead letter queue: %w", err)
	}

	log.Printf("Job %s moved to dead letter queue after %d attempts (TTL: %v)", j.ID, j.Attempts, q.failedJobTTL)
	return nil
}

// MoveScheduledToReady moves jobs from the scheduled set to their priority queues when ready
//
// This method should be called periodically by a background scheduler process (e.g., every second).
// It checks the scheduled set for jobs whose retry time has arrived and moves them back to
// their appropriate priority queues for processing.
//
// Returns the count of jobs moved to ready queues.
func (q *RedisQueue) MoveScheduledToReady(ctx context.Context) (int, error) {
	now := time.Now().Unix()

	// Find all jobs ready to run (score <= current timestamp)
	jobIDs, err := q.client.ZRangeByScore(ctx, q.getScheduledSetKey(), &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%d", now),
	}).Result()

	if err != nil {
		return 0, fmt.Errorf("failed to get scheduled jobs: %w", err)
	}

	if len(jobIDs) == 0 {
		return 0, nil
	}

	// Batch fetch all job data using MGET for efficiency
	jobKeys := make([]string, len(jobIDs))
	for i, jobID := range jobIDs {
		jobKeys[i] = q.jobKey(jobID)
	}

	jobDataList, err := q.client.MGet(ctx, jobKeys...).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get job data: %w", err)
	}

	// Process all jobs and prepare updates in memory
	type jobUpdate struct {
		job           *job.Job
		updatedData   []byte
		originalJobID string
	}
	updates := make([]jobUpdate, 0, len(jobIDs))

	for i, jobID := range jobIDs {
		jobData := jobDataList[i]
		if jobData == nil {
			// Job data not found, will remove from scheduled set
			log.Printf("Warning: Job %s in scheduled set but data not found, will be removed", jobID)
			// Add cleanup to pipeline later
			continue
		}

		// Deserialize job
		var j job.Job
		if err := json.Unmarshal([]byte(jobData.(string)), &j); err != nil {
			log.Printf("Error unmarshaling job %s: %v", jobID, err)
			continue
		}

		// Clear scheduled time
		j.ScheduledFor = nil

		// Update job data
		updatedJobData, err := json.Marshal(j)
		if err != nil {
			log.Printf("Error marshaling job %s: %v", jobID, err)
			continue
		}

		updates = append(updates, jobUpdate{
			job:           &j,
			updatedData:   updatedJobData,
			originalJobID: jobID,
		})
	}

	if len(updates) == 0 {
		return 0, nil
	}

	// Execute all updates in a single pipeline for maximum efficiency
	// This reduces N round trips to 1 when moving N jobs
	pipe := q.client.Pipeline()

	for _, update := range updates {
		// Update job data (clear ScheduledFor)
		pipe.Set(ctx, q.jobKey(update.job.ID), update.updatedData, 0)

		// Enqueue to appropriate priority queue
		pipe.LPush(ctx, q.queueKey(update.job.Priority), update.job.ID)

		// Remove from scheduled set
		pipe.ZRem(ctx, q.getScheduledSetKey(), update.job.ID)
	}

	// Execute the entire batch
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("failed to execute batch job updates: %w", err)
	}

	// Log each moved job
	for _, update := range updates {
		log.Printf("Moved scheduled job %s to priority %s (attempt %d/%d)",
			update.job.ID, update.job.Priority, update.job.Attempts, update.job.MaxRetries)
	}

	movedCount := len(updates)

	if movedCount > 0 {
		log.Printf("Moved %d scheduled jobs to ready queues", movedCount)
	}

	return movedCount, nil
}

// GetJob retrieves a job by ID from Redis
func (q *RedisQueue) GetJob(ctx context.Context, jobID string) (*job.Job, error) {
	jobData, err := q.client.Get(ctx, q.jobKey(jobID)).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	var j job.Job
	if err := json.Unmarshal([]byte(jobData), &j); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &j, nil
}

// updateQueueMetrics updates metrics with current queue depths
// This is called periodically and best-effort (errors are logged but not returned)
func (q *RedisQueue) updateQueueMetrics(ctx context.Context) {
	// Get queue depths for all priorities
	priorities := []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow}

	for _, priority := range priorities {
		depth, err := q.client.LLen(ctx, q.queueKey(priority)).Result()
		if err != nil {
			// Log error but don't fail - metrics are best-effort
			log.Printf("Failed to get queue depth for %s: %v", priority, err)
			continue
		}
		metrics.Default().RecordQueueDepth(priority, depth)
	}
}

// Close closes the Redis connection
func (q *RedisQueue) Close() error {
	if err := q.client.Close(); err != nil {
		return fmt.Errorf("failed to close Redis connection: %w", err)
	}
	log.Println("Closed Redis connection")
	return nil
}

// truncate truncates a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// DeadLetterQueueLength returns the number of jobs in the dead letter queue
// This is primarily useful for testing and monitoring
func (q *RedisQueue) DeadLetterQueueLength(ctx context.Context) (int64, error) {
	return q.client.LLen(ctx, q.deadLetterQueueKey()).Result()
}

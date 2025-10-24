package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
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
}

// NewRedisQueue creates a new Redis queue and tests the connection
func NewRedisQueue(redisURL string) (*RedisQueue, error) {
	// Parse Redis URL
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	// Create client
	client := redis.NewClient(opts)

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Printf("Connected to Redis at %s", redisURL)

	prefix := "bananas:"
	return &RedisQueue{
		client:          client,
		keyPrefix:       prefix,
		// Pre-compute all static keys once to avoid repeated string allocations
		queueHighKey:    prefix + "queue:high",
		queueNormalKey:  prefix + "queue:normal",
		queueLowKey:     prefix + "queue:low",
		processingKey:   prefix + "queue:processing",
		deadLetterKey:   prefix + "queue:dead",
		scheduledSetKey: prefix + "queue:scheduled",
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
	return nil
}

// Dequeue retrieves a job from the highest priority non-empty queue
func (q *RedisQueue) Dequeue(ctx context.Context, priorities []job.JobPriority) (*job.Job, error) {
	// Try each priority queue in order
	for _, priority := range priorities {
		queueKey := q.queueKey(priority)
		processingKey := q.processingQueueKey()

		// Atomically move job from priority queue to processing queue
		result, err := q.client.RPopLPush(ctx, queueKey, processingKey).Result()
		if err == redis.Nil {
			// Queue is empty, try next priority
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("failed to dequeue job: %w", err)
		}

		jobID := result

		// Retrieve job data
		jobData, err := q.client.Get(ctx, q.jobKey(jobID)).Result()
		if err != nil {
			// Job data not found, remove from processing queue
			q.client.LRem(ctx, processingKey, 1, jobID)
			return nil, fmt.Errorf("job data not found for ID %s: %w", jobID, err)
		}

		// Deserialize job
		var j job.Job
		if err := json.Unmarshal([]byte(jobData), &j); err != nil {
			q.client.LRem(ctx, processingKey, 1, jobID)
			return nil, fmt.Errorf("failed to unmarshal job: %w", err)
		}

		log.Printf("Dequeued job %s from %s queue", j.ID, priority)
		return &j, nil
	}

	// All queues are empty
	return nil, nil
}

// Complete marks a job as completed and removes it from the processing queue
func (q *RedisQueue) Complete(ctx context.Context, jobID string) error {
	// Remove from processing queue
	if err := q.client.LRem(ctx, q.processingQueueKey(), 1, jobID).Err(); err != nil {
		return fmt.Errorf("failed to remove job from processing queue: %w", err)
	}

	// Update job status in Redis
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

	if err := q.client.Set(ctx, q.jobKey(jobID), updatedData, 0).Err(); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	log.Printf("Completed job %s", jobID)
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

	// Update job data
	pipe.Set(ctx, q.jobKey(j.ID), jobData, 0)

	// Move to dead letter queue
	pipe.LPush(ctx, q.deadLetterQueueKey(), j.ID)

	// Remove from processing queue
	pipe.LRem(ctx, q.processingQueueKey(), 1, j.ID)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to move job to dead letter queue: %w", err)
	}

	log.Printf("Job %s moved to dead letter queue after %d attempts", j.ID, j.Attempts)
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

	movedCount := 0

	// Process each ready job
	for _, jobID := range jobIDs {
		// Retrieve job data
		jobData, err := q.client.Get(ctx, q.jobKey(jobID)).Result()
		if err == redis.Nil {
			// Job data not found, remove from scheduled set
			q.client.ZRem(ctx, q.getScheduledSetKey(), jobID)
			log.Printf("Warning: Job %s in scheduled set but data not found, removed", jobID)
			continue
		}
		if err != nil {
			log.Printf("Error retrieving job %s: %v", jobID, err)
			continue
		}

		// Deserialize job
		var j job.Job
		if err := json.Unmarshal([]byte(jobData), &j); err != nil {
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

		// Use pipeline for atomic operations
		pipe := q.client.Pipeline()

		// Update job data (clear ScheduledFor)
		pipe.Set(ctx, q.jobKey(j.ID), updatedJobData, 0)

		// Enqueue to appropriate priority queue
		pipe.LPush(ctx, q.queueKey(j.Priority), j.ID)

		// Remove from scheduled set
		pipe.ZRem(ctx, q.getScheduledSetKey(), j.ID)

		if _, err := pipe.Exec(ctx); err != nil {
			log.Printf("Error moving job %s to ready queue: %v", jobID, err)
			continue
		}

		movedCount++
		log.Printf("Moved scheduled job %s to %s queue (attempt %d/%d)",
			j.ID, j.Priority, j.Attempts, j.MaxRetries)
	}

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

// Close closes the Redis connection
func (q *RedisQueue) Close() error {
	if err := q.client.Close(); err != nil {
		return fmt.Errorf("failed to close Redis connection: %w", err)
	}
	log.Println("Closed Redis connection")
	return nil
}


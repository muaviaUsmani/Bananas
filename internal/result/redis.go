package result

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/redis/go-redis/v9"
)

// RedisBackend implements the Backend interface using Redis
type RedisBackend struct {
	client     *redis.Client
	successTTL time.Duration
	failureTTL time.Duration
}

// NewRedisBackend creates a new Redis-backed result backend
func NewRedisBackend(client *redis.Client, successTTL, failureTTL time.Duration) *RedisBackend {
	return &RedisBackend{
		client:     client,
		successTTL: successTTL,
		failureTTL: failureTTL,
	}
}

// StoreResult stores a job result in Redis
func (r *RedisBackend) StoreResult(ctx context.Context, result *job.JobResult) error {
	key := fmt.Sprintf("bananas:result:%s", result.JobID)
	notifyChannel := fmt.Sprintf("bananas:result:notify:%s", result.JobID)

	// Prepare data to store
	data := map[string]interface{}{
		"status":       string(result.Status),
		"completed_at": result.CompletedAt.Format(time.RFC3339),
		"duration_ms":  result.Duration.Milliseconds(),
	}

	if result.IsSuccess() && len(result.Result) > 0 {
		data["result"] = string(result.Result)
	}

	if result.IsFailed() && result.Error != "" {
		data["error"] = result.Error
	}

	// Determine TTL based on status
	ttl := r.successTTL
	if result.IsFailed() {
		ttl = r.failureTTL
	}

	// Use pipeline for atomicity: HSET + EXPIRE + PUBLISH
	pipe := r.client.Pipeline()
	pipe.HSet(ctx, key, data)
	pipe.Expire(ctx, key, ttl)
	pipe.Publish(ctx, notifyChannel, "ready")

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to store result: %w", err)
	}

	return nil
}

// GetResult retrieves a job result from Redis
func (r *RedisBackend) GetResult(ctx context.Context, jobID string) (*job.JobResult, error) {
	key := fmt.Sprintf("bananas:result:%s", jobID)

	// Get all fields
	data, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get result: %w", err)
	}

	// If no data, result doesn't exist
	if len(data) == 0 {
		return nil, nil
	}

	// Parse the result
	result := &job.JobResult{
		JobID: jobID,
	}

	// Parse status
	if status, exists := data["status"]; exists {
		result.Status = job.JobStatus(status)
	}

	// Parse completed_at
	if completedAt, exists := data["completed_at"]; exists {
		t, err := time.Parse(time.RFC3339, completedAt)
		if err == nil {
			result.CompletedAt = t
		}
	}

	// Parse duration
	if durationMs, exists := data["duration_ms"]; exists {
		ms, err := strconv.ParseInt(durationMs, 10, 64)
		if err == nil {
			result.Duration = time.Duration(ms) * time.Millisecond
		}
	}

	// Parse result data
	if resultData, exists := data["result"]; exists {
		result.Result = json.RawMessage(resultData)
	}

	// Parse error
	if errorMsg, exists := data["error"]; exists {
		result.Error = errorMsg
	}

	return result, nil
}

// WaitForResult blocks until a result is available or timeout is reached
// Uses Redis pub/sub for efficient waiting
func (r *RedisBackend) WaitForResult(ctx context.Context, jobID string, timeout time.Duration) (*job.JobResult, error) {
	notifyChannel := fmt.Sprintf("bananas:result:notify:%s", jobID)

	// First check if result already exists
	result, err := r.GetResult(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if result != nil {
		return result, nil
	}

	// Create a context with timeout
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Subscribe to notification channel
	pubsub := r.client.Subscribe(waitCtx, notifyChannel)
	defer func() {
		if err := pubsub.Close(); err != nil {
			// Log error but don't fail - we're already in defer
		}
	}()

	// Wait for notification or timeout
	select {
	case <-waitCtx.Done():
		// Timeout or context cancelled
		// Do one final check in case notification was missed
		result, err := r.GetResult(ctx, jobID)
		if err != nil {
			return nil, err
		}
		return result, nil // May be nil if still not ready

	case msg := <-pubsub.Channel():
		// Notification received
		if msg != nil && msg.Payload == "ready" {
			// Retrieve the result
			result, err := r.GetResult(ctx, jobID)
			if err != nil {
				return nil, err
			}
			return result, nil
		}
	}

	// Should not reach here, but return nil if we do
	return nil, nil
}

// DeleteResult removes a result from Redis
func (r *RedisBackend) DeleteResult(ctx context.Context, jobID string) error {
	key := fmt.Sprintf("bananas:result:%s", jobID)

	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete result: %w", err)
	}

	return nil
}

// Close closes the Redis client connection
func (r *RedisBackend) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

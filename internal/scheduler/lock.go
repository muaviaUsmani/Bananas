package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// DistributedLock provides Redis-based distributed locking
// Ensures only one scheduler instance can execute a given schedule at a time
type DistributedLock struct {
	client *redis.Client
	key    string
	token  string
	ttl    time.Duration
}

// AcquireLock attempts to acquire a distributed lock
// Returns lock if successful, nil if already locked by another instance
func AcquireLock(ctx context.Context, client *redis.Client, key string, ttl time.Duration) (*DistributedLock, error) {
	token := uuid.New().String()

	// SETNX: Set if not exists
	acquired, err := client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	if !acquired {
		return nil, nil // Lock already held by another instance
	}

	return &DistributedLock{
		client: client,
		key:    key,
		token:  token,
		ttl:    ttl,
	}, nil
}

// Release releases the lock (only if we still own it)
// Uses Lua script to ensure atomic check-and-delete
func (l *DistributedLock) Release(ctx context.Context) error {
	// Lua script ensures we only delete our own lock
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	_, err := l.client.Eval(ctx, script, []string{l.key}, l.token).Result()
	return err
}

// Extend extends the lock TTL (for long-running operations)
// Returns error if we no longer own the lock
func (l *DistributedLock) Extend(ctx context.Context, ttl time.Duration) error {
	// Lua script to extend only if we still own the lock
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("pexpire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`

	result, err := l.client.Eval(ctx, script, []string{l.key}, l.token, ttl.Milliseconds()).Result()
	if err != nil {
		return err
	}

	// Check if extension succeeded
	if result == int64(0) {
		return fmt.Errorf("lock no longer owned by this instance")
	}

	l.ttl = ttl
	return nil
}

// Key returns the Redis key for this lock
func (l *DistributedLock) Key() string {
	return l.key
}

// Token returns the lock token
func (l *DistributedLock) Token() string {
	return l.token
}

// TTL returns the lock time-to-live
func (l *DistributedLock) TTL() time.Duration {
	return l.ttl
}

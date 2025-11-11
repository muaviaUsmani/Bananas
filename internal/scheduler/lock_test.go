package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return client, mr
}

func TestAcquireLock_Success(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	key := "test:lock"
	ttl := 10 * time.Second

	lock, err := AcquireLock(ctx, client, key, ttl)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	if lock == nil {
		t.Fatal("Expected non-nil lock, got nil")
	}

	if lock.Key() != key {
		t.Errorf("Lock key mismatch: got %s, want %s", lock.Key(), key)
	}

	if lock.Token() == "" {
		t.Error("Expected non-empty lock token")
	}

	if lock.TTL() != ttl {
		t.Errorf("Lock TTL mismatch: got %v, want %v", lock.TTL(), ttl)
	}
}

func TestAcquireLock_AlreadyLocked(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	key := "test:lock"
	ttl := 10 * time.Second

	// Acquire first lock
	lock1, err := AcquireLock(ctx, client, key, ttl)
	if err != nil {
		t.Fatalf("Failed to acquire first lock: %v", err)
	}
	if lock1 == nil {
		t.Fatal("Expected non-nil first lock")
	}

	// Try to acquire second lock (should fail)
	lock2, err := AcquireLock(ctx, client, key, ttl)
	if err != nil {
		t.Fatalf("Unexpected error on second acquire: %v", err)
	}

	if lock2 != nil {
		t.Error("Expected nil for already-locked key, got lock")
	}
}

func TestReleaseLock_Success(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	key := "test:lock"
	ttl := 10 * time.Second

	lock, err := AcquireLock(ctx, client, key, ttl)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Release lock
	err = lock.Release(ctx)
	if err != nil {
		t.Fatalf("Failed to release lock: %v", err)
	}

	// Should be able to acquire again
	lock2, err := AcquireLock(ctx, client, key, ttl)
	if err != nil {
		t.Fatalf("Failed to re-acquire lock: %v", err)
	}

	if lock2 == nil {
		t.Error("Expected to acquire lock after release, got nil")
	}
}

func TestReleaseLock_NotOwned(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	key := "test:lock"
	ttl := 10 * time.Second

	// Manually set a lock with a different token
	client.Set(ctx, key, "different-token", ttl)

	// Try to release with a different token
	lock := &DistributedLock{
		client: client,
		key:    key,
		token:  "my-token",
		ttl:    ttl,
	}

	// Should not error, just not delete the key
	err := lock.Release(ctx)
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	// Key should still exist
	exists, err := client.Exists(ctx, key).Result()
	if err != nil {
		t.Fatalf("Failed to check key existence: %v", err)
	}

	if exists != 1 {
		t.Error("Expected key to still exist after failed release")
	}
}

func TestExtendLock_Success(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	key := "test:lock"
	initialTTL := 5 * time.Second
	extendedTTL := 10 * time.Second

	lock, err := AcquireLock(ctx, client, key, initialTTL)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Extend lock
	err = lock.Extend(ctx, extendedTTL)
	if err != nil {
		t.Fatalf("Failed to extend lock: %v", err)
	}

	if lock.TTL() != extendedTTL {
		t.Errorf("Lock TTL not updated: got %v, want %v", lock.TTL(), extendedTTL)
	}

	// Check Redis TTL is actually extended
	ttl, err := client.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("Failed to get TTL: %v", err)
	}

	// TTL should be close to extendedTTL (within 1 second)
	if ttl < 9*time.Second || ttl > 10*time.Second {
		t.Errorf("Redis TTL not extended correctly: got %v, want ~%v", ttl, extendedTTL)
	}
}

func TestExtendLock_NotOwned(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	key := "test:lock"
	ttl := 10 * time.Second

	// Manually set a lock with a different token
	client.Set(ctx, key, "different-token", ttl)

	// Try to extend with a different token
	lock := &DistributedLock{
		client: client,
		key:    key,
		token:  "my-token",
		ttl:    ttl,
	}

	err := lock.Extend(ctx, 20*time.Second)
	if err == nil {
		t.Error("Expected error when extending lock not owned, got nil")
	}
}

func TestAcquireLock_TTLExpiration(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	key := "test:lock"
	ttl := 1 * time.Second

	lock, err := AcquireLock(ctx, client, key, ttl)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}
	if lock == nil {
		t.Fatal("Expected non-nil lock")
	}

	// Wait for TTL to expire
	// Use miniredis FastForward to simulate TTL expiration
	mr.FastForward(2 * time.Second)

	// Should be able to acquire again
	lock2, err := AcquireLock(ctx, client, key, ttl)
	if err != nil {
		t.Fatalf("Failed to re-acquire lock after expiry: %v", err)
	}

	if lock2 == nil {
		t.Error("Expected to acquire lock after TTL expiry, got nil")
	}
}

func TestAcquireLock_ConcurrentAttempts(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	key := "test:lock"
	ttl := 10 * time.Second

	// Simulate concurrent lock acquisition attempts
	results := make(chan *DistributedLock, 10)
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func() {
			lock, err := AcquireLock(ctx, client, key, ttl)
			if err != nil {
				errors <- err
				return
			}
			results <- lock
		}()
	}

	// Collect results
	var locks []*DistributedLock
	var errs []error

	timeout := time.After(2 * time.Second)
	for i := 0; i < 10; i++ {
		select {
		case lock := <-results:
			locks = append(locks, lock)
		case err := <-errors:
			errs = append(errs, err)
		case <-timeout:
			t.Fatal("Timeout waiting for lock attempts")
		}
	}

	// Should have no errors
	if len(errs) > 0 {
		t.Errorf("Unexpected errors: %v", errs)
	}

	// Should have exactly 1 non-nil lock
	nonNilCount := 0
	for _, lock := range locks {
		if lock != nil {
			nonNilCount++
		}
	}

	if nonNilCount != 1 {
		t.Errorf("Expected exactly 1 successful lock, got %d", nonNilCount)
	}

	// Should have 9 nil locks
	nilCount := len(locks) - nonNilCount
	if nilCount != 9 {
		t.Errorf("Expected 9 failed lock attempts, got %d", nilCount)
	}
}

func TestLock_MultipleRelease(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	key := "test:lock"
	ttl := 10 * time.Second

	lock, err := AcquireLock(ctx, client, key, ttl)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Release once
	err = lock.Release(ctx)
	if err != nil {
		t.Fatalf("First release failed: %v", err)
	}

	// Release again (should be safe)
	err = lock.Release(ctx)
	if err != nil {
		t.Error("Second release should not error")
	}
}

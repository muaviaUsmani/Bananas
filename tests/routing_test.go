package tests

import (
	"context"
	"testing"
	"time"

	"github.com/muaviaUsmani/bananas/internal/config"
	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/muaviaUsmani/bananas/internal/queue"
	"github.com/muaviaUsmani/bananas/internal/worker"
	"github.com/redis/go-redis/v9"
)

// TestTaskRouting_BasicRouting tests that jobs are routed to the correct worker pools
func TestTaskRouting_BasicRouting(t *testing.T) {
	// Setup Redis queue
	q, client := setupTestQueue(t)
	defer q.Close()
	defer client.Close()

	ctx := context.Background()

	// Clear all queues before test
	clearAllQueues(t, client)

	// Create two jobs with different routing keys
	gpuJob := job.NewJob("process_image", []byte(`{"image":"test.jpg"}`), job.PriorityNormal)
	if err := gpuJob.SetRoutingKey("gpu"); err != nil {
		t.Fatalf("failed to set routing key: %v", err)
	}

	emailJob := job.NewJob("send_email", []byte(`{"to":"test@example.com"}`), job.PriorityNormal)
	if err := emailJob.SetRoutingKey("email"); err != nil {
		t.Fatalf("failed to set routing key: %v", err)
	}

	// Enqueue both jobs
	if err := q.Enqueue(ctx, gpuJob); err != nil {
		t.Fatalf("failed to enqueue GPU job: %v", err)
	}
	if err := q.Enqueue(ctx, emailJob); err != nil {
		t.Fatalf("failed to enqueue email job: %v", err)
	}

	// GPU worker should only get GPU jobs
	gpuWorkerJob, err := q.DequeueWithRouting(ctx, []string{"gpu"})
	if err != nil {
		t.Fatalf("failed to dequeue from GPU worker: %v", err)
	}
	if gpuWorkerJob == nil {
		t.Fatal("GPU worker got no job")
	}
	if gpuWorkerJob.RoutingKey != "gpu" {
		t.Errorf("expected routing key 'gpu', got '%s'", gpuWorkerJob.RoutingKey)
	}
	if gpuWorkerJob.Name != "process_image" {
		t.Errorf("expected job name 'process_image', got '%s'", gpuWorkerJob.Name)
	}

	// Email worker should only get email jobs
	emailWorkerJob, err := q.DequeueWithRouting(ctx, []string{"email"})
	if err != nil {
		t.Fatalf("failed to dequeue from email worker: %v", err)
	}
	if emailWorkerJob == nil {
		t.Fatal("Email worker got no job")
	}
	if emailWorkerJob.RoutingKey != "email" {
		t.Errorf("expected routing key 'email', got '%s'", emailWorkerJob.RoutingKey)
	}
	if emailWorkerJob.Name != "send_email" {
		t.Errorf("expected job name 'send_email', got '%s'", emailWorkerJob.Name)
	}

	// Cleanup
	if err := q.Complete(ctx, gpuWorkerJob.ID); err != nil {
		t.Fatalf("failed to complete GPU job: %v", err)
	}
	if err := q.Complete(ctx, emailWorkerJob.ID); err != nil {
		t.Fatalf("failed to complete email job: %v", err)
	}
}

// TestTaskRouting_MultipleRoutingKeys tests that workers can handle multiple routing keys
func TestTaskRouting_MultipleRoutingKeys(t *testing.T) {
	// Setup Redis queue
	q, client := setupTestQueue(t)
	defer q.Close()
	defer client.Close()

	ctx := context.Background()

	// Clear all queues before test
	clearAllQueues(t, client)

	// Create jobs with different routing keys
	gpuJob := job.NewJob("process_image", []byte(`{"image":"test.jpg"}`), job.PriorityHigh)
	if err := gpuJob.SetRoutingKey("gpu"); err != nil {
		t.Fatalf("failed to set routing key: %v", err)
	}

	defaultJob := job.NewJob("send_email", []byte(`{"to":"test@example.com"}`), job.PriorityNormal)
	// defaultJob already has "default" routing key

	// Enqueue both jobs
	if err := q.Enqueue(ctx, gpuJob); err != nil {
		t.Fatalf("failed to enqueue GPU job: %v", err)
	}
	if err := q.Enqueue(ctx, defaultJob); err != nil {
		t.Fatalf("failed to enqueue default job: %v", err)
	}

	// Worker handling both "gpu" and "default" routing keys should get GPU job first (higher priority)
	multiWorkerJob1, err := q.DequeueWithRouting(ctx, []string{"gpu", "default"})
	if err != nil {
		t.Fatalf("failed to dequeue from multi worker: %v", err)
	}
	if multiWorkerJob1 == nil {
		t.Fatal("Multi worker got no job")
	}
	if multiWorkerJob1.RoutingKey != "gpu" {
		t.Errorf("expected first job to be GPU job, got routing key '%s'", multiWorkerJob1.RoutingKey)
	}

	// Second dequeue should get the default job
	multiWorkerJob2, err := q.DequeueWithRouting(ctx, []string{"gpu", "default"})
	if err != nil {
		t.Fatalf("failed to dequeue from multi worker: %v", err)
	}
	if multiWorkerJob2 == nil {
		t.Fatal("Multi worker got no second job")
	}
	if multiWorkerJob2.RoutingKey != "default" {
		t.Errorf("expected second job to be default job, got routing key '%s'", multiWorkerJob2.RoutingKey)
	}

	// Cleanup
	if err := q.Complete(ctx, multiWorkerJob1.ID); err != nil {
		t.Fatalf("failed to complete job 1: %v", err)
	}
	if err := q.Complete(ctx, multiWorkerJob2.ID); err != nil {
		t.Fatalf("failed to complete job 2: %v", err)
	}
}

// TestTaskRouting_PriorityWithinRouting tests that priority is respected within each routing key
func TestTaskRouting_PriorityWithinRouting(t *testing.T) {
	// Setup Redis queue
	q, client := setupTestQueue(t)
	defer q.Close()
	defer client.Close()

	ctx := context.Background()

	// Clear all queues before test
	clearAllQueues(t, client)

	// Create GPU jobs with different priorities
	lowPriorityGPU := job.NewJob("job1", []byte(`{}`), job.PriorityLow)
	if err := lowPriorityGPU.SetRoutingKey("gpu"); err != nil {
		t.Fatalf("failed to set routing key: %v", err)
	}

	highPriorityGPU := job.NewJob("job2", []byte(`{}`), job.PriorityHigh)
	if err := highPriorityGPU.SetRoutingKey("gpu"); err != nil {
		t.Fatalf("failed to set routing key: %v", err)
	}

	// Enqueue low priority first, then high priority
	if err := q.Enqueue(ctx, lowPriorityGPU); err != nil {
		t.Fatalf("failed to enqueue low priority job: %v", err)
	}
	if err := q.Enqueue(ctx, highPriorityGPU); err != nil {
		t.Fatalf("failed to enqueue high priority job: %v", err)
	}

	// Dequeue should get high priority job first (even though low was enqueued first)
	dequeuedJob, err := q.DequeueWithRouting(ctx, []string{"gpu"})
	if err != nil {
		t.Fatalf("failed to dequeue: %v", err)
	}
	if dequeuedJob == nil {
		t.Fatal("got no job")
	}
	if dequeuedJob.Priority != job.PriorityHigh {
		t.Errorf("expected high priority job first, got priority '%s'", dequeuedJob.Priority)
	}

	// Cleanup
	if err := q.Complete(ctx, dequeuedJob.ID); err != nil {
		t.Fatalf("failed to complete job: %v", err)
	}
}

// TestTaskRouting_ScheduledJobsRespectRouting tests that scheduled jobs are routed correctly
func TestTaskRouting_ScheduledJobsRespectRouting(t *testing.T) {
	// Setup Redis queue
	q, client := setupTestQueue(t)
	defer q.Close()
	defer client.Close()

	ctx := context.Background()

	// Clear all queues before test
	clearAllQueues(t, client)

	// Create a GPU job scheduled for immediate retry (simulate a failed job with retry)
	gpuJob := job.NewJob("process_image", []byte(`{"image":"test.jpg"}`), job.PriorityNormal)
	if err := gpuJob.SetRoutingKey("gpu"); err != nil {
		t.Fatalf("failed to set routing key: %v", err)
	}

	// Enqueue and immediately dequeue to get it into processing state
	if err := q.Enqueue(ctx, gpuJob); err != nil {
		t.Fatalf("failed to enqueue job: %v", err)
	}

	dequeuedJob, err := q.DequeueWithRouting(ctx, []string{"gpu"})
	if err != nil {
		t.Fatalf("failed to dequeue job: %v", err)
	}

	// Fail the job (will schedule it for retry with backoff)
	if err := q.Fail(ctx, dequeuedJob, "test error"); err != nil {
		t.Fatalf("failed to fail job: %v", err)
	}

	// Wait for scheduled job to be ready (exponential backoff: 2 seconds for first retry)
	time.Sleep(3 * time.Second)

	// Move scheduled jobs to ready
	movedCount, err := q.MoveScheduledToReady(ctx)
	if err != nil {
		t.Fatalf("failed to move scheduled jobs: %v", err)
	}
	if movedCount != 1 {
		t.Errorf("expected 1 job moved, got %d", movedCount)
	}

	// GPU worker should get the retry job (with correct routing key)
	retryJob, err := q.DequeueWithRouting(ctx, []string{"gpu"})
	if err != nil {
		t.Fatalf("failed to dequeue retry job: %v", err)
	}
	if retryJob == nil {
		t.Fatal("got no retry job")
	}
	if retryJob.RoutingKey != "gpu" {
		t.Errorf("expected routing key 'gpu' for retry job, got '%s'", retryJob.RoutingKey)
	}
	if retryJob.ID != gpuJob.ID {
		t.Errorf("expected same job ID, got different job")
	}
	if retryJob.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", retryJob.Attempts)
	}

	// Cleanup
	if err := q.Complete(ctx, retryJob.ID); err != nil {
		t.Fatalf("failed to complete retry job: %v", err)
	}
}

// TestTaskRouting_BackwardCompatibility tests that jobs without routing keys work with default routing
func TestTaskRouting_BackwardCompatibility(t *testing.T) {
	// Setup Redis queue
	q, client := setupTestQueue(t)
	defer q.Close()
	defer client.Close()

	ctx := context.Background()

	// Clear all queues before test
	clearAllQueues(t, client)

	// Create a job without explicitly setting routing key (should default to "default")
	defaultJob := job.NewJob("send_email", []byte(`{"to":"test@example.com"}`), job.PriorityNormal)

	// Enqueue job
	if err := q.Enqueue(ctx, defaultJob); err != nil {
		t.Fatalf("failed to enqueue job: %v", err)
	}

	// Worker with default routing key should get the job
	dequeuedJob, err := q.DequeueWithRouting(ctx, []string{"default"})
	if err != nil {
		t.Fatalf("failed to dequeue job: %v", err)
	}
	if dequeuedJob == nil {
		t.Fatal("got no job")
	}
	if dequeuedJob.RoutingKey != "default" {
		t.Errorf("expected routing key 'default', got '%s'", dequeuedJob.RoutingKey)
	}

	// Cleanup
	if err := q.Complete(ctx, dequeuedJob.ID); err != nil {
		t.Fatalf("failed to complete job: %v", err)
	}
}

// TestTaskRouting_WorkerPoolIntegration tests worker pool with routing keys
func TestTaskRouting_WorkerPoolIntegration(t *testing.T) {
	// This test verifies that the worker pool correctly uses routing keys
	// Setup Redis queue
	q, client := setupTestQueue(t)
	defer q.Close()
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Clear all queues before test
	clearAllQueues(t, client)

	// Create handler registry
	registry := worker.NewRegistry()
	jobProcessed := make(chan string, 10)

	// Register a GPU job handler
	registry.Register("process_image", func(ctx context.Context, j *job.Job) error {
		jobProcessed <- "gpu"
		return nil
	})

	// Create executor
	executor := worker.NewExecutor(registry, q, 2)

	// Create worker config with GPU routing key
	workerConfig := &config.WorkerConfig{
		Mode:        config.WorkerModeDefault,
		Concurrency: 2,
		Priorities:  []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow},
		RoutingKeys: []string{"gpu"},
	}

	// Create worker pool
	pool := worker.NewPoolWithConfig(executor, q, workerConfig, 30*time.Second)

	// Start worker pool
	pool.Start(ctx)
	defer pool.Stop()

	// Give workers a moment to start
	time.Sleep(100 * time.Millisecond)

	// Submit a GPU job
	gpuJob := job.NewJob("process_image", []byte(`{"image":"test.jpg"}`), job.PriorityNormal)
	if err := gpuJob.SetRoutingKey("gpu"); err != nil {
		t.Fatalf("failed to set routing key: %v", err)
	}

	if err := q.Enqueue(ctx, gpuJob); err != nil {
		t.Fatalf("failed to enqueue GPU job: %v", err)
	}

	// Wait for job to be processed
	select {
	case processedType := <-jobProcessed:
		if processedType != "gpu" {
			t.Errorf("expected GPU job to be processed, got '%s'", processedType)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for job to be processed")
	}
}

// Helper functions

func setupTestQueue(t *testing.T) (*queue.RedisQueue, *redis.Client) {
	redisURL := "redis://localhost:6379/1" // Use database 1 for tests

	q, err := queue.NewRedisQueue(redisURL)
	if err != nil {
		t.Fatalf("failed to connect to Redis: %v", err)
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("failed to parse Redis URL: %v", err)
	}
	client := redis.NewClient(opts)

	return q, client
}

func clearAllQueues(t *testing.T, client *redis.Client) {
	ctx := context.Background()

	// Clear all routing key queues we might use in tests
	routingKeys := []string{"default", "gpu", "email"}
	priorities := []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow}

	for _, routingKey := range routingKeys {
		for _, priority := range priorities {
			queueKey := "bananas:route:" + routingKey + ":queue:" + string(priority)
			client.Del(ctx, queueKey)
		}
	}

	// Clear shared queues
	client.Del(ctx, "bananas:queue:processing")
	client.Del(ctx, "bananas:queue:dead")
	client.Del(ctx, "bananas:queue:scheduled")

	// Clear any job keys from previous tests
	keys, err := client.Keys(ctx, "bananas:job:*").Result()
	if err == nil && len(keys) > 0 {
		client.Del(ctx, keys...)
	}
}

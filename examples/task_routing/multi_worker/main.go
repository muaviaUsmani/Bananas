package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/muaviaUsmani/bananas/internal/config"
	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/muaviaUsmani/bananas/internal/queue"
	"github.com/muaviaUsmani/bananas/internal/worker"
)

func main() {
	log.Println("Starting Multi-Routing Worker Example")

	// Connect to Redis
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379/0")
	q, err := queue.NewRedisQueue(redisURL)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer q.Close()

	// Create handler registry
	registry := worker.NewRegistry()

	// Register handlers for different job types
	registry.Register("process_image", processImage)
	registry.Register("send_email", sendEmail)
	registry.Register("generate_report", generateReport)
	registry.Register("process_data", processData)

	log.Printf("Registered %d job handlers", registry.Count())

	// Create executor
	executor := worker.NewExecutor(registry, q, 10)

	// Configure worker to handle multiple routing keys
	// This worker will prioritize jobs in the order: gpu > email > default
	workerConfig := &config.WorkerConfig{
		Mode:        config.WorkerModeDefault,
		Concurrency: 10, // 10 concurrent jobs
		Priorities:  []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow},
		RoutingKeys: []string{"gpu", "email", "default"}, // Handle multiple routing keys
	}

	log.Printf("Worker configuration: %s", workerConfig.String())
	log.Println("This worker will process jobs in order: gpu:high, gpu:normal, gpu:low, email:high, email:normal, email:low, default:high, default:normal, default:low")

	// Create worker pool
	pool := worker.NewPoolWithConfig(executor, q, workerConfig, 5*time.Minute)

	// Start processing
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)
	log.Println("Multi-routing worker pool started")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutdown signal received, stopping worker...")

	pool.Stop()
	log.Println("Worker stopped")
}

// processImage handles image processing (can run on GPU or CPU)
func processImage(ctx context.Context, j *job.Job) error {
	log.Printf("Processing image (routing: %s)", j.RoutingKey)

	select {
	case <-time.After(2 * time.Second):
		log.Println("Image processed")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// sendEmail handles email sending
func sendEmail(ctx context.Context, j *job.Job) error {
	type EmailPayload struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}

	var payload EmailPayload
	if err := j.UnmarshalPayload(&payload); err != nil {
		return err
	}

	log.Printf("Sending email to %s: %s (routing: %s)", payload.To, payload.Subject, j.RoutingKey)

	select {
	case <-time.After(1 * time.Second):
		log.Printf("Email sent to %s", payload.To)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// generateReport handles report generation
func generateReport(ctx context.Context, j *job.Job) error {
	log.Printf("Generating report (routing: %s)", j.RoutingKey)

	select {
	case <-time.After(3 * time.Second):
		log.Println("Report generated")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// processData handles general data processing
func processData(ctx context.Context, j *job.Job) error {
	log.Printf("Processing data (routing: %s)", j.RoutingKey)

	select {
	case <-time.After(2 * time.Second):
		log.Println("Data processed")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

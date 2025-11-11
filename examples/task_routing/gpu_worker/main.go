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
	log.Println("Starting GPU Worker Example")

	// Connect to Redis
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379/0")
	q, err := queue.NewRedisQueue(redisURL)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer q.Close()

	// Create handler registry
	registry := worker.NewRegistry()

	// Register GPU-intensive job handlers
	registry.Register("process_image", processImage)
	registry.Register("train_model", trainModel)
	registry.Register("video_transcode", videoTranscode)

	log.Printf("Registered %d GPU job handlers", registry.Count())

	// Create executor
	executor := worker.NewExecutor(registry, q, 4)

	// Configure worker to only handle GPU routing key
	workerConfig := &config.WorkerConfig{
		Mode:        config.WorkerModeDefault,
		Concurrency: 4, // 4 concurrent GPU jobs
		Priorities:  []job.JobPriority{job.PriorityHigh, job.PriorityNormal, job.PriorityLow},
		RoutingKeys: []string{"gpu"}, // Only process GPU jobs
	}

	log.Printf("Worker configuration: %s", workerConfig.String())

	// Create worker pool
	pool := worker.NewPoolWithConfig(executor, q, workerConfig, 5*time.Minute)

	// Start processing
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)
	log.Println("GPU worker pool started - processing jobs from 'gpu' routing key")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutdown signal received, stopping GPU worker...")

	pool.Stop()
	log.Println("GPU worker stopped")
}

// processImage simulates GPU-based image processing
func processImage(ctx context.Context, j *job.Job) error {
	type ImagePayload struct {
		URL    string `json:"url"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}

	var payload ImagePayload
	if err := j.UnmarshalPayload(&payload); err != nil {
		return err
	}

	log.Printf("Processing image on GPU: %s (resize to %dx%d)", payload.URL, payload.Width, payload.Height)

	// Simulate GPU processing (1-3 seconds)
	select {
	case <-time.After(2 * time.Second):
		log.Printf("Image processed successfully: %s", payload.URL)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// trainModel simulates GPU-based ML model training
func trainModel(ctx context.Context, j *job.Job) error {
	type TrainingPayload struct {
		Model     string `json:"model"`
		Dataset   string `json:"dataset"`
		Epochs    int    `json:"epochs"`
		BatchSize int    `json:"batch_size"`
	}

	var payload TrainingPayload
	if err := j.UnmarshalPayload(&payload); err != nil {
		return err
	}

	log.Printf("Training model on GPU: %s (dataset: %s, epochs: %d)",
		payload.Model, payload.Dataset, payload.Epochs)

	// Simulate GPU training (5-10 seconds)
	select {
	case <-time.After(7 * time.Second):
		log.Printf("Model trained successfully: %s", payload.Model)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// videoTranscode simulates GPU-based video transcoding
func videoTranscode(ctx context.Context, j *job.Job) error {
	type VideoPayload struct {
		InputURL  string `json:"input_url"`
		OutputURL string `json:"output_url"`
		Format    string `json:"format"`
		Quality   string `json:"quality"`
	}

	var payload VideoPayload
	if err := j.UnmarshalPayload(&payload); err != nil {
		return err
	}

	log.Printf("Transcoding video on GPU: %s -> %s (format: %s, quality: %s)",
		payload.InputURL, payload.OutputURL, payload.Format, payload.Quality)

	// Simulate GPU transcoding (3-5 seconds)
	select {
	case <-time.After(4 * time.Second):
		log.Printf("Video transcoded successfully: %s", payload.OutputURL)
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

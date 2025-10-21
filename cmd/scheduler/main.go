package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/muaviaUsmani/bananas/internal/config"
	"github.com/muaviaUsmani/bananas/internal/queue"
)

// connectWithRetry attempts to connect to Redis with exponential backoff
func connectWithRetry(redisURL string, maxRetries int) (*queue.RedisQueue, error) {
	var redisQueue *queue.RedisQueue
	var err error

	for attempt := 0; attempt < maxRetries; attempt++ {
		redisQueue, err = queue.NewRedisQueue(redisURL)
		if err == nil {
			return redisQueue, nil
		}

		// Calculate exponential backoff delay: 2^attempt seconds (max 30 seconds)
		delay := time.Duration(1<<uint(attempt)) * time.Second
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}

		log.Printf("Failed to connect to Redis (attempt %d/%d): %v. Retrying in %v...",
			attempt+1, maxRetries, err, delay)

		time.Sleep(delay)
	}

	return nil, fmt.Errorf("failed to connect to Redis after %d attempts: %w", maxRetries, err)
}

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Println("Scheduler starting...")
	fmt.Printf("Connecting to Redis: %s\n", cfg.RedisURL)
	fmt.Printf("Max retries for failed jobs: %d\n", cfg.MaxRetries)

	// Connect to Redis queue with retry logic
	redisQueue, err := connectWithRetry(cfg.RedisURL, 5)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisQueue.Close()

	log.Println("Successfully connected to Redis")

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start background goroutine to move scheduled jobs
	go func() {
		// Ticker for periodic execution (every second)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		log.Println("Scheduler ready - monitoring scheduled jobs...")

		for {
			select {
			case <-ticker.C:
				// Move scheduled jobs that are ready to their priority queues
				count, err := redisQueue.MoveScheduledToReady(ctx)
				if err != nil {
					log.Printf("Error moving scheduled jobs: %v", err)
				}
				// Only log if jobs were actually moved (reduces log noise)
				if count > 0 {
					log.Printf("Moved %d scheduled jobs to ready queues", count)
				}

			case <-ctx.Done():
				log.Println("Scheduler stopping...")
				return
			}
		}
	}()

	// Wait for shutdown signal
	sig := <-sigChan
	log.Printf("Received signal %v, initiating graceful shutdown...", sig)

	// Cancel context to stop background goroutine
	cancel()

	// Give background tasks time to finish
	time.Sleep(2 * time.Second)

	log.Println("Scheduler shut down successfully")
}


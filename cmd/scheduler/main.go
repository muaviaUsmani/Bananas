package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/muaviaUsmani/bananas/internal/config"
	"github.com/muaviaUsmani/bananas/internal/logger"
	"github.com/muaviaUsmani/bananas/internal/queue"
)

// connectWithRetry attempts to connect to Redis with exponential backoff
func connectWithRetry(redisURL string, maxRetries int, log logger.Logger) (*queue.RedisQueue, error) {
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

		log.Warn("Failed to connect to Redis, retrying",
			"attempt", attempt+1,
			"max_attempts", maxRetries,
			"error", err,
			"retry_in", delay)

		time.Sleep(delay)
	}

	return nil, fmt.Errorf("failed to connect to Redis after %d attempts: %w", maxRetries, err)
}

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.NewLogger(cfg.Logging)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Close()

	// Set as default logger
	logger.SetDefault(log)

	// Create component-specific logger
	schedulerLog := log.WithComponent(logger.ComponentScheduler).WithSource(logger.LogSourceInternal)

	schedulerLog.Info("Scheduler starting",
		"redis_url", cfg.RedisURL,
		"max_retries", cfg.MaxRetries)

	// Start pprof server on separate port for profiling
	pprofPort := os.Getenv("PPROF_PORT")
	if pprofPort == "" {
		pprofPort = "6062"
	}
	go func() {
		schedulerLog.Info("Starting pprof server", "port", pprofPort, "url", fmt.Sprintf("http://localhost:%s/debug/pprof/", pprofPort))
		if err := http.ListenAndServe(":"+pprofPort, nil); err != nil {
			schedulerLog.Error("pprof server failed", "error", err)
		}
	}()

	// Connect to Redis queue with retry logic
	redisQueue, err := connectWithRetry(cfg.RedisURL, 5, schedulerLog)
	if err != nil {
		schedulerLog.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer redisQueue.Close()

	schedulerLog.Info("Successfully connected to Redis")

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

		schedulerLog.Info("Scheduler ready - monitoring scheduled jobs")

		for {
			select {
			case <-ticker.C:
				// Move scheduled jobs that are ready to their priority queues
				count, err := redisQueue.MoveScheduledToReady(ctx)
				if err != nil {
					schedulerLog.Error("Error moving scheduled jobs", "error", err)
				}
				// Only log if jobs were actually moved (reduces log noise)
				if count > 0 {
					schedulerLog.Info("Moved scheduled jobs to ready queues", "count", count)
				}

			case <-ctx.Done():
				schedulerLog.Info("Scheduler stopping")
				return
			}
		}
	}()

	// Wait for shutdown signal
	sig := <-sigChan
	schedulerLog.Info("Received shutdown signal, initiating graceful shutdown", "signal", sig)

	// Cancel context to stop background goroutine
	cancel()

	// Give background tasks time to finish
	time.Sleep(2 * time.Second)

	schedulerLog.Info("Scheduler shut down successfully")
}

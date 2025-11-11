// Package main provides the Bananas scheduler service for managing cron-based job scheduling.
package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof" // #nosec G108 - pprof is intentionally exposed for debugging, isolated to separate port
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/muaviaUsmani/bananas/internal/config"
	"github.com/muaviaUsmani/bananas/internal/logger"
	"github.com/muaviaUsmani/bananas/internal/queue"
	"github.com/muaviaUsmani/bananas/internal/scheduler"
	"github.com/redis/go-redis/v9"
)

// createRedisClient creates a Redis client from the Redis URL
func createRedisClient(redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}
	return redis.NewClient(opts), nil
}

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
		// #nosec G115 - attempt is bounded by maxRetries parameter, overflow not possible
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
	defer func() {
		if err := log.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close logger: %v\n", err)
		}
	}()

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
		// Create server with timeouts for security
		server := &http.Server{
			Addr:              ":" + pprofPort,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       60 * time.Second,
		}
		if err := server.ListenAndServe(); err != nil {
			schedulerLog.Error("pprof server failed", "error", err)
		}
	}()

	// Connect to Redis queue with retry logic
	redisQueue, err := connectWithRetry(cfg.RedisURL, 5, schedulerLog)
	if err != nil {
		schedulerLog.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := redisQueue.Close(); err != nil {
			schedulerLog.Error("Failed to close Redis queue", "error", err)
		}
	}()

	schedulerLog.Info("Successfully connected to Redis")

	// Create Redis client for cron scheduler
	redisClient, err := createRedisClient(cfg.RedisURL)
	if err != nil {
		schedulerLog.Error("Failed to create Redis client", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			schedulerLog.Error("Failed to close Redis client", "error", err)
		}
	}()

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize cron scheduler if enabled
	var cronScheduler *scheduler.CronScheduler
	if cfg.CronSchedulerEnabled {
		registry := scheduler.NewRegistry()

		// Register example schedules (users should replace this with their own schedules)
		// Example: Daily report at midnight UTC
		// registry.MustRegister(&scheduler.Schedule{
		// 	ID:          "daily-report",
		// 	Cron:        "0 0 * * *",
		// 	Job:         "generate_report",
		// 	Priority:    job.PriorityNormal,
		// 	Timezone:    "UTC",
		// 	Enabled:     true,
		// 	Description: "Generate daily report",
		// })

		cronScheduler = scheduler.NewCronScheduler(registry, redisQueue, redisClient, cfg.CronSchedulerInterval)
		schedulerLog.Info("Cron scheduler initialized",
			"interval", cfg.CronSchedulerInterval,
			"schedules", registry.Count())

		// Start cron scheduler in background
		go cronScheduler.Start(ctx)
	}

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

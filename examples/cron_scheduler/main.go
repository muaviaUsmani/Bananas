package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/muaviaUsmani/bananas/internal/queue"
	"github.com/muaviaUsmani/bananas/internal/scheduler"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Configuration
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379")

	// Create Redis client
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL: %v", err)
	}
	redisClient := redis.NewClient(opts)
	defer redisClient.Close()

	// Test Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Connected to Redis")

	// Create Redis queue for job enqueueing
	redisQueue, err := queue.NewRedisQueue(redisURL)
	if err != nil {
		log.Fatalf("Failed to create Redis queue: %v", err)
	}
	defer redisQueue.Close()

	// Create schedule registry
	registry := scheduler.NewRegistry()

	// Example 1: Every minute
	registry.MustRegister(&scheduler.Schedule{
		ID:          "every-minute",
		Cron:        "* * * * *",
		Job:         "ping",
		Payload:     []byte(`{"message": "Hello from cron!"}`),
		Priority:    job.PriorityNormal,
		Timezone:    "UTC",
		Enabled:     true,
		Description: "Ping job that runs every minute",
	})

	// Example 2: Every 5 minutes
	registry.MustRegister(&scheduler.Schedule{
		ID:          "every-5-minutes",
		Cron:        "*/5 * * * *",
		Job:         "cleanup",
		Payload:     []byte(`{"type": "temp_files"}`),
		Priority:    job.PriorityLow,
		Timezone:    "UTC",
		Enabled:     true,
		Description: "Cleanup temporary files every 5 minutes",
	})

	// Example 3: Every hour at minute 0
	registry.MustRegister(&scheduler.Schedule{
		ID:          "hourly-report",
		Cron:        "0 * * * *",
		Job:         "generate_report",
		Payload:     []byte(`{"report_type": "hourly"}`),
		Priority:    job.PriorityHigh,
		Timezone:    "UTC",
		Enabled:     true,
		Description: "Generate hourly metrics report",
	})

	// Example 4: Daily at 9 AM EST
	registry.MustRegister(&scheduler.Schedule{
		ID:          "daily-summary",
		Cron:        "0 9 * * *",
		Job:         "send_summary_email",
		Payload:     []byte(`{"recipients": ["admin@example.com"]}`),
		Priority:    job.PriorityNormal,
		Timezone:    "America/New_York",
		Enabled:     true,
		Description: "Send daily summary email at 9 AM EST",
	})

	// Example 5: Weekly on Monday at midnight
	registry.MustRegister(&scheduler.Schedule{
		ID:          "weekly-backup",
		Cron:        "0 0 * * 1",
		Job:         "database_backup",
		Payload:     []byte(`{"backup_type": "full", "retention_days": 30}`),
		Priority:    job.PriorityHigh,
		Timezone:    "UTC",
		Enabled:     true,
		Description: "Full database backup every Monday at midnight",
	})

	// Example 6: Monthly on 1st at midnight
	registry.MustRegister(&scheduler.Schedule{
		ID:          "monthly-invoice",
		Cron:        "0 0 1 * *",
		Job:         "generate_invoices",
		Payload:     []byte(`{"billing_cycle": "monthly"}`),
		Priority:    job.PriorityHigh,
		Timezone:    "UTC",
		Enabled:     true,
		Description: "Generate monthly invoices on the 1st",
	})

	// Example 7: Disabled schedule (won't run)
	registry.MustRegister(&scheduler.Schedule{
		ID:          "disabled-job",
		Cron:        "* * * * *",
		Job:         "test_job",
		Priority:    job.PriorityNormal,
		Timezone:    "UTC",
		Enabled:     false, // This schedule is disabled
		Description: "Example of a disabled schedule",
	})

	log.Printf("Registered %d schedules", registry.Count())

	// Print schedule details
	for _, sched := range registry.List() {
		nextRun, _ := registry.NextRun(sched, time.Now())
		log.Printf("Schedule: %s | Cron: %s | Job: %s | Next Run: %s | Enabled: %v",
			sched.ID, sched.Cron, sched.Job, nextRun.Format(time.RFC3339), sched.Enabled)
	}

	// Create cron scheduler (checks every second)
	cronScheduler := scheduler.NewCronScheduler(registry, redisQueue, redisClient, 1*time.Second)

	// Create context for graceful shutdown
	shutdownCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start cron scheduler
	go cronScheduler.Start(shutdownCtx)

	log.Println("Cron scheduler started. Press Ctrl+C to stop.")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	cancel()

	// Give scheduler time to finish current operations
	time.Sleep(2 * time.Second)

	log.Println("Scheduler stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

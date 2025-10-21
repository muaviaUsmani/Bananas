package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/muaviaUsmani/bananas/internal/config"
	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/muaviaUsmani/bananas/internal/queue"
	"github.com/muaviaUsmani/bananas/internal/worker"
	"github.com/muaviaUsmani/bananas/pkg/client"
)

// UserSignupPayload represents the data for user signup jobs
type UserSignupPayload struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Plan  string `json:"plan"`
}

// EmailPayload represents the data for email sending jobs
type EmailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// DataProcessingPayload represents the data for data processing jobs
type DataProcessingPayload struct {
	DataSet string `json:"dataset"`
	Options map[string]interface{} `json:"options"`
}

// HandleUserSignup processes user signup jobs
func HandleUserSignup(ctx context.Context, j *job.Job) error {
	var data UserSignupPayload
	if err := json.Unmarshal(j.Payload, &data); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	log.Printf("[UserSignup] Processing signup for %s (%s) - Plan: %s", data.Name, data.Email, data.Plan)

	// Simulate account creation
	time.Sleep(1 * time.Second)

	log.Printf("[UserSignup] Account created for %s", data.Email)
	return nil
}

// HandleSendWelcomeEmail sends welcome emails to new users
func HandleSendWelcomeEmail(ctx context.Context, j *job.Job) error {
	var data EmailPayload
	if err := json.Unmarshal(j.Payload, &data); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	log.Printf("[WelcomeEmail] Sending email to %s: %s", data.To, data.Subject)

	// Simulate email sending
	time.Sleep(500 * time.Millisecond)

	log.Printf("[WelcomeEmail] Email sent successfully to %s", data.To)
	return nil
}

// HandleDataProcessing processes large datasets
func HandleDataProcessing(ctx context.Context, j *job.Job) error {
	var data DataProcessingPayload
	if err := json.Unmarshal(j.Payload, &data); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	log.Printf("[DataProcessing] Processing dataset: %s", data.DataSet)

	// Simulate data processing
	for i := 1; i <= 5; i++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			log.Printf("[DataProcessing] Progress: %d/5 - %s", i, data.DataSet)
			time.Sleep(500 * time.Millisecond)
		}
	}

	log.Printf("[DataProcessing] Completed processing: %s", data.DataSet)
	return nil
}

// startWorkers initializes and starts the worker pool
func startWorkers(registry *worker.Registry, redisURL string) {
	log.Println("=== Starting Workers ===")

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to Redis queue
	redisQueue, err := queue.NewRedisQueue(redisURL)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisQueue.Close()

	// Create executor
	executor := worker.NewExecutor(registry, redisQueue, cfg.WorkerConcurrency)

	// Create worker pool
	pool := worker.NewPool(executor, redisQueue, cfg.WorkerConcurrency, cfg.JobTimeout)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start worker pool
	pool.Start(ctx)

	log.Printf("Workers started with %d concurrent workers", cfg.WorkerConcurrency)

	// Wait for shutdown signal
	sig := <-sigChan
	log.Printf("Received signal %v, shutting down workers...", sig)

	// Cancel context and stop pool
	cancel()
	pool.Stop()

	log.Println("Workers shut down successfully")
}

// startScheduler initializes and starts the scheduler service
func startScheduler(redisURL string) {
	log.Println("=== Starting Scheduler ===")

	// Connect to Redis queue
	redisQueue, err := queue.NewRedisQueue(redisURL)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisQueue.Close()

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start scheduler loop in goroutine
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		log.Println("Scheduler ready - monitoring scheduled jobs...")

		for {
			select {
			case <-ticker.C:
				count, err := redisQueue.MoveScheduledToReady(ctx)
				if err != nil {
					log.Printf("Error moving scheduled jobs: %v", err)
				}
				if count > 0 {
					log.Printf("Scheduler: Moved %d jobs to ready queues", count)
				}
			case <-ctx.Done():
				log.Println("Scheduler stopping...")
				return
			}
		}
	}()

	// Wait for shutdown signal
	sig := <-sigChan
	log.Printf("Received signal %v, shutting down scheduler...", sig)

	cancel()
	time.Sleep(1 * time.Second)

	log.Println("Scheduler shut down successfully")
}

// monitorJob polls for job status updates and prints them
func monitorJob(c *client.Client, jobID string, timeout time.Duration) (*job.Job, error) {
	deadline := time.Now().Add(timeout)

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for job completion")
		}

		j, err := c.GetJob(jobID)
		if err != nil {
			return nil, fmt.Errorf("failed to get job: %w", err)
		}

		switch j.Status {
		case job.StatusCompleted:
			return j, nil
		case job.StatusFailed:
			return j, fmt.Errorf("job failed: %s", j.Error)
		default:
			// Job still processing, wait a bit
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func main() {
	fmt.Println("========================================")
	fmt.Println("   Bananas - Complete Workflow Example")
	fmt.Println("========================================")
	fmt.Println()

	// Use default Redis URL or from environment
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	fmt.Printf("Redis URL: %s\n", redisURL)
	fmt.Println()

	// Step 1: Register job handlers
	fmt.Println("Step 1: Registering job handlers...")
	registry := worker.NewRegistry()
	registry.Register("user_signup", HandleUserSignup)
	registry.Register("send_welcome_email", HandleSendWelcomeEmail)
	registry.Register("data_processing", HandleDataProcessing)
	fmt.Printf("Registered %d handlers\n\n", registry.Count())

	// Step 2: Start workers in background
	fmt.Println("Step 2: Starting workers...")
	go startWorkers(registry, redisURL)
	time.Sleep(1 * time.Second) // Give workers time to start
	fmt.Println()

	// Step 3: Start scheduler in background
	fmt.Println("Step 3: Starting scheduler...")
	go startScheduler(redisURL)
	time.Sleep(1 * time.Second) // Give scheduler time to start
	fmt.Println()

	// Step 4: Create client and submit jobs
	fmt.Println("Step 4: Creating client and submitting jobs...")
	c, err := client.NewClient(redisURL)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	// Submit high-priority user signup
	signupPayload := UserSignupPayload{
		Email: "john.doe@example.com",
		Name:  "John Doe",
		Plan:  "premium",
	}
	jobID1, err := c.SubmitJob("user_signup", signupPayload, job.PriorityHigh, "Process new premium user signup")
	if err != nil {
		log.Fatalf("Failed to submit signup job: %v", err)
	}
	fmt.Printf("✓ Submitted user signup job: %s\n", jobID1)

	// Submit normal-priority welcome email
	emailPayload := EmailPayload{
		To:      "john.doe@example.com",
		Subject: "Welcome to Bananas!",
		Body:    "Thank you for signing up...",
	}
	jobID2, err := c.SubmitJob("send_welcome_email", emailPayload, job.PriorityNormal, "Send welcome email to new user")
	if err != nil {
		log.Fatalf("Failed to submit email job: %v", err)
	}
	fmt.Printf("✓ Submitted welcome email job: %s\n", jobID2)

	// Submit low-priority data processing
	dataPayload := DataProcessingPayload{
		DataSet: "user_analytics_2025",
		Options: map[string]interface{}{
			"format": "json",
			"compress": true,
		},
	}
	jobID3, err := c.SubmitJob("data_processing", dataPayload, job.PriorityLow, "Process analytics dataset")
	if err != nil {
		log.Fatalf("Failed to submit data processing job: %v", err)
	}
	fmt.Printf("✓ Submitted data processing job: %s\n", jobID3)

	// Submit a scheduled job (5 seconds from now)
	scheduledTime := time.Now().Add(5 * time.Second)
	jobID4, err := c.SubmitJobScheduled("send_welcome_email", EmailPayload{
		To:      "jane.smith@example.com",
		Subject: "Reminder: Complete your profile",
		Body:    "We noticed you haven't completed your profile...",
	}, job.PriorityNormal, scheduledTime, "Scheduled reminder email")
	if err != nil {
		log.Fatalf("Failed to submit scheduled job: %v", err)
	}
	fmt.Printf("✓ Submitted scheduled email job: %s (scheduled for %s)\n", jobID4, scheduledTime.Format(time.RFC3339))
	fmt.Println()

	// Step 5: Monitor job status
	fmt.Println("Step 5: Monitoring job execution...")
	fmt.Println("(Watch the worker logs above for processing details)")
	fmt.Println()

	// Monitor first 3 jobs
	jobs := []string{jobID1, jobID2, jobID3}
	for i, id := range jobs {
		fmt.Printf("Waiting for job %d/%d (%s)...\n", i+1, len(jobs), id)
		completedJob, err := monitorJob(c, id, 30*time.Second)
		if err != nil {
			log.Printf("Error monitoring job %s: %v", id, err)
			continue
		}
		duration := completedJob.UpdatedAt.Sub(completedJob.CreatedAt)
		fmt.Printf("✓ Job %d completed in %v\n", i+1, duration.Round(time.Millisecond))
	}
	fmt.Println()

	// Wait a bit for the scheduled job
	fmt.Println("Waiting for scheduled job to execute...")
	time.Sleep(6 * time.Second)

	scheduledJob, err := c.GetJob(jobID4)
	if err != nil {
		log.Printf("Error getting scheduled job: %v", err)
	} else {
		fmt.Printf("✓ Scheduled job status: %s\n", scheduledJob.Status)
	}
	fmt.Println()

	// Final summary
	fmt.Println("========================================")
	fmt.Println("   Workflow Completed Successfully!")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("What happened:")
	fmt.Println("1. Workers started and began polling Redis queues")
	fmt.Println("2. Scheduler started monitoring for scheduled jobs")
	fmt.Println("3. Four jobs were submitted (3 immediate, 1 scheduled)")
	fmt.Println("4. Jobs were processed in priority order (High > Normal > Low)")
	fmt.Println("5. Scheduled job executed at the specified time")
	fmt.Println()
	fmt.Println("Key Features Demonstrated:")
	fmt.Println("✓ Priority-based job processing")
	fmt.Println("✓ Scheduled job execution")
	fmt.Println("✓ Graceful worker shutdown")
	fmt.Println("✓ Job status monitoring")
	fmt.Println("✓ Custom job handlers")
	fmt.Println()
	fmt.Println("Press Ctrl+C to shutdown workers and scheduler...")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")
}

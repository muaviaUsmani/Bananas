package main

import (
	"log"
	"os"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/muaviaUsmani/bananas/pkg/client"
)

func main() {
	log.Println("Task Routing Client Example")

	// Connect to Redis
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379/0")
	c, err := client.NewClient(redisURL)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	log.Println("Connected to Bananas job queue")
	log.Println("Submitting jobs with different routing keys...")
	log.Println()

	// Submit GPU jobs
	submitGPUJobs(c)
	time.Sleep(500 * time.Millisecond)

	// Submit email jobs
	submitEmailJobs(c)
	time.Sleep(500 * time.Millisecond)

	// Submit default jobs
	submitDefaultJobs(c)

	log.Println("\nAll jobs submitted successfully!")
	log.Println("Check worker logs to see which workers process which jobs")
}

func submitGPUJobs(c *client.Client) {
	log.Println("=== Submitting GPU Jobs ===")

	// Image processing job
	imagePayload := map[string]interface{}{
		"url":    "https://example.com/image.jpg",
		"width":  1920,
		"height": 1080,
	}
	jobID, err := c.SubmitJobWithRoute(
		"process_image",
		imagePayload,
		job.PriorityHigh,
		"gpu",
		"Resize image to 1920x1080",
	)
	if err != nil {
		log.Fatalf("Failed to submit image job: %v", err)
	}
	log.Printf("✓ Submitted GPU image processing job: %s", jobID)

	// Model training job
	trainingPayload := map[string]interface{}{
		"model":      "resnet50",
		"dataset":    "imagenet",
		"epochs":     10,
		"batch_size": 32,
	}
	jobID, err = c.SubmitJobWithRoute(
		"train_model",
		trainingPayload,
		job.PriorityNormal,
		"gpu",
		"Train ResNet-50 on ImageNet",
	)
	if err != nil {
		log.Fatalf("Failed to submit training job: %v", err)
	}
	log.Printf("✓ Submitted GPU model training job: %s", jobID)

	// Video transcoding job
	videoPayload := map[string]interface{}{
		"input_url":  "https://example.com/input.mp4",
		"output_url": "https://example.com/output.mp4",
		"format":     "h264",
		"quality":    "high",
	}
	jobID, err = c.SubmitJobWithRoute(
		"video_transcode",
		videoPayload,
		job.PriorityLow,
		"gpu",
		"Transcode video to H.264",
	)
	if err != nil {
		log.Fatalf("Failed to submit video job: %v", err)
	}
	log.Printf("✓ Submitted GPU video transcoding job: %s\n", jobID)
}

func submitEmailJobs(c *client.Client) {
	log.Println("=== Submitting Email Jobs ===")

	emails := []struct {
		to      string
		subject string
		body    string
	}{
		{"user1@example.com", "Welcome!", "Welcome to our service"},
		{"user2@example.com", "Password Reset", "Click here to reset your password"},
		{"user3@example.com", "Monthly Report", "Here is your monthly usage report"},
	}

	for _, email := range emails {
		payload := map[string]interface{}{
			"to":      email.to,
			"subject": email.subject,
			"body":    email.body,
		}
		jobID, err := c.SubmitJobWithRoute(
			"send_email",
			payload,
			job.PriorityNormal,
			"email",
			"Send email to "+email.to,
		)
		if err != nil {
			log.Fatalf("Failed to submit email job: %v", err)
		}
		log.Printf("✓ Submitted email job to %s: %s", email.to, jobID)
	}
	log.Println()
}

func submitDefaultJobs(c *client.Client) {
	log.Println("=== Submitting Default Jobs ===")

	// Report generation job (uses default routing)
	reportPayload := map[string]interface{}{
		"report_type": "sales",
		"period":      "monthly",
		"year":        2024,
		"month":       11,
	}
	jobID, err := c.SubmitJob(
		"generate_report",
		reportPayload,
		job.PriorityHigh,
		"Generate monthly sales report",
	)
	if err != nil {
		log.Fatalf("Failed to submit report job: %v", err)
	}
	log.Printf("✓ Submitted report generation job: %s", jobID)

	// Data processing job (uses default routing)
	dataPayload := map[string]interface{}{
		"dataset":   "user_events",
		"operation": "aggregate",
		"filters":   []string{"active_users", "premium_only"},
	}
	jobID, err = c.SubmitJob(
		"process_data",
		dataPayload,
		job.PriorityNormal,
		"Process user event data",
	)
	if err != nil {
		log.Fatalf("Failed to submit data processing job: %v", err)
	}
	log.Printf("✓ Submitted data processing job: %s", jobID)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

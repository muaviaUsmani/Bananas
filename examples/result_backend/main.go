package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/muaviaUsmani/bananas/pkg/client"
)

func main() {
	// Create client (result backend enabled by default)
	c, err := client.NewClient("redis://localhost:6379")
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	fmt.Println("=== Bananas Result Backend Example ===\n")

	// Example 1: Submit and wait (RPC-style)
	fmt.Println("Example 1: Submit and Wait (RPC-style)")
	fmt.Println("---------------------------------------")

	payload := map[string]interface{}{
		"task":  "process_data",
		"count": 100,
	}

	ctx := context.Background()
	result, err := c.SubmitAndWait(ctx, "process_data", payload, job.PriorityNormal, 30*time.Second)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else if result.IsSuccess() {
		fmt.Printf("✓ Job completed successfully!\n")
		fmt.Printf("  Duration: %v\n", result.Duration)
		fmt.Printf("  Completed at: %s\n\n", result.CompletedAt.Format(time.RFC3339))
	} else {
		fmt.Printf("✗ Job failed: %s\n\n", result.Error)
	}

	// Example 2: Submit and check later
	fmt.Println("Example 2: Submit and Check Later")
	fmt.Println("----------------------------------")

	jobID, err := c.SubmitJob("send_email", map[string]string{
		"to":      "user@example.com",
		"subject": "Welcome!",
	}, job.PriorityHigh)

	if err != nil {
		log.Fatalf("Failed to submit job: %v", err)
	}

	fmt.Printf("Job submitted: %s\n", jobID)
	fmt.Println("Waiting for completion...")

	// Poll for result
	for i := 0; i < 10; i++ {
		result, err := c.GetResult(ctx, jobID)
		if err != nil {
			fmt.Printf("Error getting result: %v\n", err)
			break
		}

		if result != nil {
			if result.IsSuccess() {
				fmt.Printf("✓ Job completed after %v\n\n", result.Duration)
			} else {
				fmt.Printf("✗ Job failed: %s\n\n", result.Error)
			}
			break
		}

		fmt.Printf("  Still running... (%d/%d)\n", i+1, 10)
		time.Sleep(time.Second)
	}

	// Example 3: Multiple jobs
	fmt.Println("Example 3: Batch Job Submission")
	fmt.Println("--------------------------------")

	jobIDs := []string{}
	for i := 0; i < 5; i++ {
		jobID, err := c.SubmitJob("count_items", map[string]int{"count": i * 10}, job.PriorityNormal)
		if err != nil {
			fmt.Printf("Failed to submit job %d: %v\n", i, err)
			continue
		}
		jobIDs = append(jobIDs, jobID)
	}

	fmt.Printf("Submitted %d jobs\n", len(jobIDs))
	fmt.Println("Waiting for all to complete...")

	// Wait for all results
	completed := 0
	for _, jobID := range jobIDs {
		for {
			result, err := c.GetResult(ctx, jobID)
			if err != nil || result != nil {
				if result != nil && result.IsSuccess() {
					completed++
				}
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	fmt.Printf("✓ Completed: %d/%d jobs\n\n", completed, len(jobIDs))

	fmt.Println("=== Example Complete ===")
}

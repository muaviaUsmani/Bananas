// Package worker contains example job handlers for demonstration.
// Users should create their own handlers based on their needs.
package worker

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
)

// HandleCountItems counts items in a JSON array payload
func HandleCountItems(ctx context.Context, j *job.Job) error {
	var items []string
	if err := json.Unmarshal(j.Payload, &items); err != nil {
		return err
	}
	log.Printf("Job %s: counted %d items", j.ID, len(items))
	return nil
}

// HandleSendEmail simulates sending an email
func HandleSendEmail(ctx context.Context, j *job.Job) error {
	var email struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := json.Unmarshal(j.Payload, &email); err != nil {
		return err
	}
	log.Printf("Job %s: sending email to %s", j.ID, email.To)
	time.Sleep(2 * time.Second) // Simulate work
	return nil
}

// HandleProcessData simulates data processing
func HandleProcessData(ctx context.Context, j *job.Job) error {
	log.Printf("Job %s: processing data", j.ID)
	time.Sleep(3 * time.Second) // Simulate work
	return nil
}


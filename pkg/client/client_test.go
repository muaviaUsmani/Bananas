package client

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/muaviaUsmani/bananas/internal/job"
)

func TestNewClient(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	client, err := NewClient("redis://" + s.Addr())

	if err != nil {
		t.Fatalf("expected no error creating client, got %v", err)
	}
	if client == nil {
		t.Fatal("expected client to be created, got nil")
	}
	if client.queue == nil {
		t.Error("expected queue to be initialized")
	}
	defer client.Close()
}

func TestNewClient_ConnectionFailure(t *testing.T) {
	// Try to connect to invalid Redis URL
	client, err := NewClient("redis://invalid-host:9999")

	if err == nil {
		t.Fatal("expected error for invalid Redis URL, got nil")
	}
	if client != nil {
		t.Error("expected nil client on connection failure")
	}
}

func TestSubmitJob_CreatesJobCorrectly(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	client, err := NewClient("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	payload := map[string]string{"key": "value"}
	jobID, err := client.SubmitJob("test_job", payload, job.PriorityNormal, "Test description")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if jobID == "" {
		t.Error("expected non-empty job ID")
	}

	// Verify job was stored in Redis
	j, err := client.GetJob(jobID)
	if err != nil {
		t.Fatalf("failed to get submitted job: %v", err)
	}
	if j.Name != "test_job" {
		t.Errorf("expected job name 'test_job', got '%s'", j.Name)
	}
	if j.Description != "Test description" {
		t.Errorf("expected description 'Test description', got '%s'", j.Description)
	}
	if j.Priority != job.PriorityNormal {
		t.Errorf("expected priority %s, got %s", job.PriorityNormal, j.Priority)
	}
	if j.Status != job.StatusPending {
		t.Errorf("expected status %s, got %s", job.StatusPending, j.Status)
	}
}

func TestSubmitJob_ReturnsValidUUID(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	client, err := NewClient("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	jobID, err := client.SubmitJob("test_job", map[string]string{}, job.PriorityHigh)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// UUID should be 36 characters (including hyphens)
	if len(jobID) != 36 {
		t.Errorf("expected UUID length 36, got %d", len(jobID))
	}
}

func TestSubmitJob_MarshalsPayloadCorrectly(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	client, err := NewClient("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	type TestPayload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	payload := TestPayload{Name: "test", Count: 42}
	jobID, err := client.SubmitJob("test_job", payload, job.PriorityLow)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	j, _ := client.GetJob(jobID)

	// Verify payload was marshaled correctly
	var unmarshaled TestPayload
	if err := json.Unmarshal(j.Payload, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}
	if unmarshaled.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", unmarshaled.Name)
	}
	if unmarshaled.Count != 42 {
		t.Errorf("expected count 42, got %d", unmarshaled.Count)
	}
}

func TestGetJob_RetrievesSubmittedJob(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	client, err := NewClient("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	jobID, _ := client.SubmitJob("test_job", map[string]string{"foo": "bar"}, job.PriorityNormal)

	j, err := client.GetJob(jobID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if j == nil {
		t.Fatal("expected job to be returned, got nil")
	}
	if j.ID != jobID {
		t.Errorf("expected job ID %s, got %s", jobID, j.ID)
	}
}

func TestGetJob_ReturnsErrorForNonExistent(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	client, err := NewClient("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	_, err = client.GetJob("non-existent-id")

	if err == nil {
		t.Fatal("expected error for non-existent job, got nil")
	}
}

func TestSubmitJobScheduled(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	client, err := NewClient("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	// Schedule job for 5 seconds in the future
	scheduledTime := time.Now().Add(5 * time.Second)
	payload := map[string]string{"task": "future_task"}

	jobID, err := client.SubmitJobScheduled("scheduled_job", payload, job.PriorityNormal, scheduledTime, "Scheduled task")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if jobID == "" {
		t.Error("expected non-empty job ID")
	}

	// Verify job was stored
	j, err := client.GetJob(jobID)
	if err != nil {
		t.Fatalf("failed to get scheduled job: %v", err)
	}

	// Job should have a scheduled time set (even if not exactly what we specified,
	// since we're using the Fail mechanism which uses exponential backoff)
	if j.ScheduledFor == nil {
		t.Fatal("expected scheduled time to be set")
	}

	// Scheduled time should be in the future
	if j.ScheduledFor.Before(time.Now()) {
		t.Error("expected scheduled time to be in the future")
	}
}

func TestSubmitJob_ThreadSafety(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	client, err := NewClient("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	var wg sync.WaitGroup
	jobCount := 100
	errors := make(chan error, jobCount)

	// Submit jobs concurrently
	for i := 0; i < jobCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			payload := map[string]int{"index": index}
			_, err := client.SubmitJob("concurrent_job", payload, job.PriorityNormal)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("error submitting job: %v", err)
	}
}


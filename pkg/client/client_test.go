package client

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/muaviaUsmani/bananas/internal/job"
)

func TestNewClient(t *testing.T) {
	client := NewClient()

	if client == nil {
		t.Fatal("expected client to be created, got nil")
	}
	if client.jobs == nil {
		t.Error("expected jobs map to be initialized")
	}
	if len(client.jobs) != 0 {
		t.Errorf("expected empty jobs map, got %d jobs", len(client.jobs))
	}
}

func TestSubmitJob_CreatesJobCorrectly(t *testing.T) {
	client := NewClient()

	payload := map[string]string{"key": "value"}
	jobID, err := client.SubmitJob("test_job", payload, job.PriorityNormal, "Test description")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if jobID == "" {
		t.Error("expected non-empty job ID")
	}

	// Verify job was stored
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
	client := NewClient()

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
	client := NewClient()

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
	client := NewClient()

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
	client := NewClient()

	_, err := client.GetJob("non-existent-id")

	if err == nil {
		t.Fatal("expected error for non-existent job, got nil")
	}
}

func TestListJobs_ReturnsAllSubmittedJobs(t *testing.T) {
	client := NewClient()

	// Submit multiple jobs
	id1, _ := client.SubmitJob("job1", map[string]string{}, job.PriorityHigh)
	id2, _ := client.SubmitJob("job2", map[string]string{}, job.PriorityNormal)
	id3, _ := client.SubmitJob("job3", map[string]string{}, job.PriorityLow)

	jobs := client.ListJobs()

	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	// Verify all job IDs are present
	ids := make(map[string]bool)
	for _, j := range jobs {
		ids[j.ID] = true
	}

	if !ids[id1] || !ids[id2] || !ids[id3] {
		t.Error("not all submitted jobs were returned by ListJobs")
	}
}

func TestSubmitJob_ThreadSafety(t *testing.T) {
	client := NewClient()

	var wg sync.WaitGroup
	jobCount := 100

	// Submit jobs concurrently
	for i := 0; i < jobCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			payload := map[string]int{"index": index}
			_, err := client.SubmitJob("concurrent_job", payload, job.PriorityNormal)
			if err != nil {
				t.Errorf("error submitting job: %v", err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all jobs were stored
	jobs := client.ListJobs()
	if len(jobs) != jobCount {
		t.Errorf("expected %d jobs, got %d", jobCount, len(jobs))
	}
}


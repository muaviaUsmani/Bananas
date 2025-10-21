package job

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewJob_CreatesWithCorrectDefaults(t *testing.T) {
	payload := []byte(`{"key":"value"}`)
	j := NewJob("test_job", payload, PriorityNormal)

	if j == nil {
		t.Fatal("expected job to be created, got nil")
	}
	if j.Name != "test_job" {
		t.Errorf("expected name 'test_job', got '%s'", j.Name)
	}
	if j.Priority != PriorityNormal {
		t.Errorf("expected priority %s, got %s", PriorityNormal, j.Priority)
	}
	if j.Status != StatusPending {
		t.Errorf("expected status %s, got %s", StatusPending, j.Status)
	}
	if j.Attempts != 0 {
		t.Errorf("expected 0 attempts, got %d", j.Attempts)
	}
	if j.MaxRetries != 3 {
		t.Errorf("expected max retries 3, got %d", j.MaxRetries)
	}
	if string(j.Payload) != `{"key":"value"}` {
		t.Errorf("expected payload to match, got %s", string(j.Payload))
	}
}

func TestNewJob_GeneratesUniqueIDs(t *testing.T) {
	payload := []byte("{}")

	j1 := NewJob("test1", payload, PriorityNormal)
	j2 := NewJob("test2", payload, PriorityNormal)
	j3 := NewJob("test3", payload, PriorityNormal)

	if j1.ID == j2.ID || j2.ID == j3.ID || j1.ID == j3.ID {
		t.Error("expected unique IDs, got duplicates")
	}

	// Verify UUIDs are proper length (36 characters)
	if len(j1.ID) != 36 || len(j2.ID) != 36 || len(j3.ID) != 36 {
		t.Error("expected UUID format with length 36")
	}
}

func TestNewJob_WithDescription(t *testing.T) {
	payload := []byte("{}")
	description := "Test job description"

	j := NewJob("test_job", payload, PriorityHigh, description)

	if j.Description != description {
		t.Errorf("expected description '%s', got '%s'", description, j.Description)
	}
}

func TestNewJob_WithoutDescription(t *testing.T) {
	payload := []byte("{}")

	j := NewJob("test_job", payload, PriorityLow)

	if j.Description != "" {
		t.Errorf("expected empty description, got '%s'", j.Description)
	}
}

func TestUpdateStatus_ChangesStatusAndTimestamp(t *testing.T) {
	j := NewJob("test_job", []byte("{}"), PriorityNormal)

	initialStatus := j.Status
	initialTime := j.UpdatedAt

	// Wait a bit to ensure timestamp changes
	time.Sleep(10 * time.Millisecond)

	j.UpdateStatus(StatusProcessing)

	if j.Status == initialStatus {
		t.Error("expected status to change")
	}
	if j.Status != StatusProcessing {
		t.Errorf("expected status %s, got %s", StatusProcessing, j.Status)
	}
	if !j.UpdatedAt.After(initialTime) {
		t.Error("expected UpdatedAt timestamp to be updated")
	}
}

func TestJobPriority_Values(t *testing.T) {
	tests := []struct {
		priority JobPriority
		expected string
	}{
		{PriorityHigh, "high"},
		{PriorityNormal, "normal"},
		{PriorityLow, "low"},
	}

	for _, tt := range tests {
		if string(tt.priority) != tt.expected {
			t.Errorf("expected priority value '%s', got '%s'", tt.expected, string(tt.priority))
		}
	}
}

func TestJobStatus_Values(t *testing.T) {
	tests := []struct {
		status   JobStatus
		expected string
	}{
		{StatusPending, "pending"},
		{StatusProcessing, "processing"},
		{StatusCompleted, "completed"},
		{StatusFailed, "failed"},
		{StatusScheduled, "scheduled"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("expected status value '%s', got '%s'", tt.expected, string(tt.status))
		}
	}
}

func TestJob_JSONMarshaling(t *testing.T) {
	payload := []byte(`{"test":"data"}`)
	j := NewJob("test_job", payload, PriorityHigh, "Test description")

	// Marshal to JSON
	data, err := json.Marshal(j)
	if err != nil {
		t.Fatalf("failed to marshal job: %v", err)
	}

	// Unmarshal back
	var unmarshaled Job
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal job: %v", err)
	}

	// Verify fields
	if unmarshaled.ID != j.ID {
		t.Errorf("expected ID %s, got %s", j.ID, unmarshaled.ID)
	}
	if unmarshaled.Name != j.Name {
		t.Errorf("expected name %s, got %s", j.Name, unmarshaled.Name)
	}
	if unmarshaled.Description != j.Description {
		t.Errorf("expected description %s, got %s", j.Description, unmarshaled.Description)
	}
	if unmarshaled.Priority != j.Priority {
		t.Errorf("expected priority %s, got %s", j.Priority, unmarshaled.Priority)
	}
}

func TestJob_TimestampsSet(t *testing.T) {
	before := time.Now()
	j := NewJob("test_job", []byte("{}"), PriorityNormal)
	after := time.Now()

	if j.CreatedAt.Before(before) || j.CreatedAt.After(after) {
		t.Error("CreatedAt timestamp not set correctly")
	}
	if j.UpdatedAt.Before(before) || j.UpdatedAt.After(after) {
		t.Error("UpdatedAt timestamp not set correctly")
	}
}


package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/muaviaUsmani/bananas/internal/job"
)

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()

	handler := func(ctx context.Context, j *job.Job) error {
		return nil
	}

	registry.Register("test_handler", handler)

	if registry.Count() != 1 {
		t.Errorf("expected 1 handler, got %d", registry.Count())
	}
}

func TestRegistry_Get_RegisteredHandler(t *testing.T) {
	registry := NewRegistry()

	expectedHandler := func(ctx context.Context, j *job.Job) error {
		return nil
	}

	registry.Register("test_handler", expectedHandler)

	handler, exists := registry.Get("test_handler")

	if !exists {
		t.Fatal("expected handler to exist")
	}
	if handler == nil {
		t.Error("expected handler to be non-nil")
	}
}

func TestRegistry_Get_UnregisteredHandler(t *testing.T) {
	registry := NewRegistry()

	_, exists := registry.Get("non_existent")

	if exists {
		t.Error("expected handler not to exist")
	}
}

func TestHandleCountItems_ExecutesWithoutError(t *testing.T) {
	ctx := context.Background()

	items := []string{"item1", "item2", "item3", "item4"}
	payload, _ := json.Marshal(items)
	j := job.NewJob("count_items", payload, job.PriorityNormal)

	err := HandleCountItems(ctx, j)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestHandleCountItems_InvalidPayload(t *testing.T) {
	ctx := context.Background()

	// Invalid JSON payload
	j := job.NewJob("count_items", []byte("invalid json"), job.PriorityNormal)

	err := HandleCountItems(ctx, j)

	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestHandleSendEmail_ExecutesWithoutError(t *testing.T) {
	ctx := context.Background()

	email := struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}{
		To:      "test@example.com",
		Subject: "Test Email",
		Body:    "This is a test",
	}

	payload, _ := json.Marshal(email)
	j := job.NewJob("send_email", payload, job.PriorityHigh)

	err := HandleSendEmail(ctx, j)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestHandleSendEmail_InvalidPayload(t *testing.T) {
	ctx := context.Background()

	j := job.NewJob("send_email", []byte("not valid json"), job.PriorityNormal)

	err := HandleSendEmail(ctx, j)

	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestHandleProcessData_ExecutesWithoutError(t *testing.T) {
	ctx := context.Background()

	j := job.NewJob("process_data", []byte("{}"), job.PriorityNormal)

	err := HandleProcessData(ctx, j)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestRegistry_MultipleHandlers(t *testing.T) {
	registry := NewRegistry()

	registry.Register("handler1", HandleCountItems)
	registry.Register("handler2", HandleSendEmail)
	registry.Register("handler3", HandleProcessData)

	if registry.Count() != 3 {
		t.Errorf("expected 3 handlers, got %d", registry.Count())
	}

	// Verify all handlers can be retrieved
	tests := []string{"handler1", "handler2", "handler3"}
	for _, name := range tests {
		_, exists := registry.Get(name)
		if !exists {
			t.Errorf("expected handler %s to exist", name)
		}
	}
}

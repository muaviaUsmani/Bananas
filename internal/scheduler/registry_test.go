package scheduler

import (
	"testing"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	if registry == nil {
		t.Fatal("Expected non-nil registry")
	}
	if registry.Count() != 0 {
		t.Errorf("Expected empty registry, got %d schedules", registry.Count())
	}
}

func TestRegister_Valid(t *testing.T) {
	registry := NewRegistry()

	schedule := &Schedule{
		ID:          "test_schedule",
		Cron:        "0 * * * *",
		Job:         "test_job",
		Priority:    job.PriorityNormal,
		Timezone:    "UTC",
		Enabled:     true,
		Description: "Test schedule",
	}

	err := registry.Register(schedule)
	if err != nil {
		t.Fatalf("Failed to register valid schedule: %v", err)
	}

	if registry.Count() != 1 {
		t.Errorf("Expected 1 schedule, got %d", registry.Count())
	}

	retrieved, exists := registry.Get("test_schedule")
	if !exists {
		t.Fatal("Schedule not found after registration")
	}
	if retrieved.ID != schedule.ID {
		t.Errorf("Retrieved schedule ID mismatch: got %s, want %s", retrieved.ID, schedule.ID)
	}
}

func TestRegister_DuplicateID(t *testing.T) {
	registry := NewRegistry()

	schedule1 := &Schedule{
		ID:   "duplicate",
		Cron: "0 * * * *",
		Job:  "job1",
	}

	schedule2 := &Schedule{
		ID:   "duplicate",
		Cron: "0 0 * * *",
		Job:  "job2",
	}

	err := registry.Register(schedule1)
	if err != nil {
		t.Fatalf("Failed to register first schedule: %v", err)
	}

	err = registry.Register(schedule2)
	if err == nil {
		t.Error("Expected error for duplicate schedule ID, got nil")
	}

	if registry.Count() != 1 {
		t.Errorf("Expected 1 schedule after duplicate, got %d", registry.Count())
	}
}

func TestRegister_InvalidID(t *testing.T) {
	registry := NewRegistry()

	tests := []struct {
		name string
		id   string
	}{
		{"empty", ""},
		{"spaces", "test schedule"},
		{"special chars", "test@schedule"},
		{"dots", "test.schedule"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule := &Schedule{
				ID:   tt.id,
				Cron: "0 * * * *",
				Job:  "test_job",
			}

			err := registry.Register(schedule)
			if err == nil {
				t.Errorf("Expected error for invalid ID %q, got nil", tt.id)
			}
		})
	}
}

func TestRegister_InvalidCron(t *testing.T) {
	registry := NewRegistry()

	tests := []struct {
		name string
		cron string
	}{
		{"empty", ""},
		{"invalid format", "0 * * *"}, // Only 4 fields
		{"invalid field", "60 * * * *"}, // Minute 60 doesn't exist
		{"garbage", "not a cron expression"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule := &Schedule{
				ID:   "test_schedule",
				Cron: tt.cron,
				Job:  "test_job",
			}

			err := registry.Register(schedule)
			if err == nil {
				t.Errorf("Expected error for invalid cron %q, got nil", tt.cron)
			}
		})
	}
}

func TestRegister_EmptyJob(t *testing.T) {
	registry := NewRegistry()

	schedule := &Schedule{
		ID:   "test_schedule",
		Cron: "0 * * * *",
		Job:  "",
	}

	err := registry.Register(schedule)
	if err == nil {
		t.Error("Expected error for empty job name, got nil")
	}
}

func TestRegister_InvalidTimezone(t *testing.T) {
	registry := NewRegistry()

	schedule := &Schedule{
		ID:       "test_schedule",
		Cron:     "0 * * * *",
		Job:      "test_job",
		Timezone: "Invalid/Timezone",
	}

	err := registry.Register(schedule)
	if err == nil {
		t.Error("Expected error for invalid timezone, got nil")
	}
}

func TestRegister_InvalidPriority(t *testing.T) {
	registry := NewRegistry()

	schedule := &Schedule{
		ID:       "test_schedule",
		Cron:     "0 * * * *",
		Job:      "test_job",
		Priority: job.JobPriority("invalid"),
	}

	err := registry.Register(schedule)
	if err == nil {
		t.Error("Expected error for invalid priority, got nil")
	}
}

func TestMustRegister_Valid(t *testing.T) {
	registry := NewRegistry()

	schedule := &Schedule{
		ID:   "test_schedule",
		Cron: "0 * * * *",
		Job:  "test_job",
	}

	// Should not panic
	registry.MustRegister(schedule)

	if registry.Count() != 1 {
		t.Errorf("Expected 1 schedule, got %d", registry.Count())
	}
}

func TestMustRegister_Invalid(t *testing.T) {
	registry := NewRegistry()

	schedule := &Schedule{
		ID:   "", // Invalid
		Cron: "0 * * * *",
		Job:  "test_job",
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for invalid schedule, got none")
		}
	}()

	registry.MustRegister(schedule)
}

func TestGet_NotFound(t *testing.T) {
	registry := NewRegistry()

	_, exists := registry.Get("nonexistent")
	if exists {
		t.Error("Expected false for nonexistent schedule, got true")
	}
}

func TestList(t *testing.T) {
	registry := NewRegistry()

	schedule1 := &Schedule{
		ID:   "schedule1",
		Cron: "0 * * * *",
		Job:  "job1",
	}

	schedule2 := &Schedule{
		ID:   "schedule2",
		Cron: "0 0 * * *",
		Job:  "job2",
	}

	registry.Register(schedule1)
	registry.Register(schedule2)

	schedules := registry.List()
	if len(schedules) != 2 {
		t.Errorf("Expected 2 schedules, got %d", len(schedules))
	}
}

func TestNextRun_Simple(t *testing.T) {
	registry := NewRegistry()

	schedule := &Schedule{
		ID:       "test",
		Cron:     "0 * * * *", // Every hour
		Job:      "test_job",
		Timezone: "UTC",
	}

	registry.Register(schedule)

	// Test from a known time
	now := time.Date(2025, 11, 10, 14, 30, 0, 0, time.UTC)
	next, err := registry.NextRun(schedule, now)
	if err != nil {
		t.Fatalf("NextRun failed: %v", err)
	}

	// Next run should be 15:00:00
	expected := time.Date(2025, 11, 10, 15, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("NextRun returned %v, expected %v", next, expected)
	}
}

func TestNextRun_Every15Minutes(t *testing.T) {
	registry := NewRegistry()

	schedule := &Schedule{
		ID:       "test",
		Cron:     "*/15 * * * *", // Every 15 minutes
		Job:      "test_job",
		Timezone: "UTC",
	}

	registry.Register(schedule)

	// Test from 14:07
	now := time.Date(2025, 11, 10, 14, 7, 0, 0, time.UTC)
	next, err := registry.NextRun(schedule, now)
	if err != nil {
		t.Fatalf("NextRun failed: %v", err)
	}

	// Next run should be 14:15:00
	expected := time.Date(2025, 11, 10, 14, 15, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("NextRun returned %v, expected %v", next, expected)
	}
}

func TestNextRun_DailyAt9AM(t *testing.T) {
	registry := NewRegistry()

	schedule := &Schedule{
		ID:       "test",
		Cron:     "0 9 * * *", // Daily at 9 AM
		Job:      "test_job",
		Timezone: "UTC",
	}

	registry.Register(schedule)

	// Test from November 10 at 8 AM
	now := time.Date(2025, 11, 10, 8, 0, 0, 0, time.UTC)
	next, err := registry.NextRun(schedule, now)
	if err != nil {
		t.Fatalf("NextRun failed: %v", err)
	}

	// Next run should be same day at 9 AM
	expected := time.Date(2025, 11, 10, 9, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("NextRun returned %v, expected %v", next, expected)
	}

	// Test from November 10 at 10 AM (after 9 AM)
	now = time.Date(2025, 11, 10, 10, 0, 0, 0, time.UTC)
	next, err = registry.NextRun(schedule, now)
	if err != nil {
		t.Fatalf("NextRun failed: %v", err)
	}

	// Next run should be next day at 9 AM
	expected = time.Date(2025, 11, 11, 9, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("NextRun returned %v, expected %v", next, expected)
	}
}

func TestNextRun_Timezone(t *testing.T) {
	registry := NewRegistry()

	schedule := &Schedule{
		ID:       "test",
		Cron:     "0 9 * * *", // Daily at 9 AM
		Job:      "test_job",
		Timezone: "America/New_York",
	}

	registry.Register(schedule)

	// Test from November 10 at 8 AM EST (13:00 UTC)
	loc, _ := time.LoadLocation("America/New_York")
	now := time.Date(2025, 11, 10, 8, 0, 0, 0, loc)

	next, err := registry.NextRun(schedule, now)
	if err != nil {
		t.Fatalf("NextRun failed: %v", err)
	}

	// Next run should be 9 AM EST same day
	expected := time.Date(2025, 11, 10, 9, 0, 0, 0, loc)
	if !next.Equal(expected) {
		t.Errorf("NextRun returned %v, expected %v", next, expected)
	}
}

func TestNextRun_InvalidCron(t *testing.T) {
	registry := NewRegistry()

	schedule := &Schedule{
		ID:       "test",
		Cron:     "invalid",
		Job:      "test_job",
		Timezone: "UTC",
	}

	// Don't register (would fail validation), test directly
	_, err := registry.NextRun(schedule, time.Now())
	if err == nil {
		t.Error("Expected error for invalid cron, got nil")
	}
}

func TestNextRun_InvalidTimezone(t *testing.T) {
	registry := NewRegistry()

	schedule := &Schedule{
		ID:       "test",
		Cron:     "0 * * * *",
		Job:      "test_job",
		Timezone: "Invalid/Timezone",
	}

	// Don't register (would fail validation), test directly
	_, err := registry.NextRun(schedule, time.Now())
	if err == nil {
		t.Error("Expected error for invalid timezone, got nil")
	}
}

func TestRegister_DefaultTimezone(t *testing.T) {
	registry := NewRegistry()

	schedule := &Schedule{
		ID:   "test",
		Cron: "0 * * * *",
		Job:  "test_job",
		// Timezone not specified
	}

	err := registry.Register(schedule)
	if err != nil {
		t.Fatalf("Failed to register schedule: %v", err)
	}

	retrieved, _ := registry.Get("test")
	if retrieved.Timezone != "UTC" {
		t.Errorf("Expected default timezone UTC, got %s", retrieved.Timezone)
	}
}

func TestRegister_ValidPriorities(t *testing.T) {
	registry := NewRegistry()

	priorities := []job.JobPriority{
		job.PriorityHigh,
		job.PriorityNormal,
		job.PriorityLow,
	}

	for _, priority := range priorities {
		schedule := &Schedule{
			ID:       string(priority) + "_schedule",
			Cron:     "0 * * * *",
			Job:      "test_job",
			Priority: priority,
		}

		err := registry.Register(schedule)
		if err != nil {
			t.Errorf("Failed to register schedule with priority %s: %v", priority, err)
		}
	}
}

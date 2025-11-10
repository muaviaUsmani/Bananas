package config

import (
	"os"
	"testing"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
)

func TestLoadWorkerConfig_DefaultMode(t *testing.T) {
	// Clear environment
	os.Clearenv()

	cfg, err := LoadWorkerConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Mode != WorkerModeDefault {
		t.Errorf("Expected mode=default, got %s", cfg.Mode)
	}
	if cfg.Concurrency != 10 {
		t.Errorf("Expected concurrency=10, got %d", cfg.Concurrency)
	}
	if len(cfg.Priorities) != 3 {
		t.Errorf("Expected 3 priorities, got %d", len(cfg.Priorities))
	}
	if !cfg.EnableScheduler {
		t.Error("Expected scheduler to be enabled")
	}
}

func TestLoadWorkerConfig_ThinMode(t *testing.T) {
	os.Clearenv()
	os.Setenv("WORKER_MODE", "thin")

	cfg, err := LoadWorkerConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Mode != WorkerModeThin {
		t.Errorf("Expected mode=thin, got %s", cfg.Mode)
	}
	if cfg.Concurrency != 5 { // Default for thin mode
		t.Errorf("Expected concurrency=5, got %d", cfg.Concurrency)
	}
	if len(cfg.Priorities) != 3 {
		t.Errorf("Expected all 3 priorities, got %d", len(cfg.Priorities))
	}
	if !cfg.EnableScheduler {
		t.Error("Expected scheduler to be enabled")
	}
}

func TestLoadWorkerConfig_SpecializedMode(t *testing.T) {
	os.Clearenv()
	os.Setenv("WORKER_MODE", "specialized")
	os.Setenv("WORKER_PRIORITIES", "high")
	os.Setenv("WORKER_CONCURRENCY", "50")

	cfg, err := LoadWorkerConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Mode != WorkerModeSpecialized {
		t.Errorf("Expected mode=specialized, got %s", cfg.Mode)
	}
	if cfg.Concurrency != 50 {
		t.Errorf("Expected concurrency=50, got %d", cfg.Concurrency)
	}
	if len(cfg.Priorities) != 1 || cfg.Priorities[0] != job.PriorityHigh {
		t.Errorf("Expected only high priority, got %v", cfg.Priorities)
	}
	if cfg.EnableScheduler {
		t.Error("Expected scheduler to be disabled by default in specialized mode")
	}
}

func TestLoadWorkerConfig_JobSpecializedMode(t *testing.T) {
	os.Clearenv()
	os.Setenv("WORKER_MODE", "job-specialized")
	os.Setenv("WORKER_JOB_TYPES", "send_email,generate_report")
	os.Setenv("WORKER_CONCURRENCY", "20")

	cfg, err := LoadWorkerConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Mode != WorkerModeJobSpecialized {
		t.Errorf("Expected mode=job-specialized, got %s", cfg.Mode)
	}
	if cfg.Concurrency != 20 {
		t.Errorf("Expected concurrency=20, got %d", cfg.Concurrency)
	}
	if len(cfg.JobTypes) != 2 {
		t.Errorf("Expected 2 job types, got %d", len(cfg.JobTypes))
	}
	if cfg.JobTypes[0] != "send_email" || cfg.JobTypes[1] != "generate_report" {
		t.Errorf("Unexpected job types: %v", cfg.JobTypes)
	}
}

func TestLoadWorkerConfig_SchedulerOnlyMode(t *testing.T) {
	os.Clearenv()
	os.Setenv("WORKER_MODE", "scheduler-only")
	os.Setenv("SCHEDULER_INTERVAL", "2s")

	cfg, err := LoadWorkerConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Mode != WorkerModeSchedulerOnly {
		t.Errorf("Expected mode=scheduler-only, got %s", cfg.Mode)
	}
	if cfg.Concurrency != 0 {
		t.Errorf("Expected concurrency=0, got %d", cfg.Concurrency)
	}
	if len(cfg.Priorities) != 0 {
		t.Errorf("Expected no priorities, got %d", len(cfg.Priorities))
	}
	if !cfg.EnableScheduler {
		t.Error("Expected scheduler to be enabled")
	}
	if cfg.SchedulerInterval != 2*time.Second {
		t.Errorf("Expected scheduler interval=2s, got %v", cfg.SchedulerInterval)
	}
}

func TestValidate_InvalidMode(t *testing.T) {
	cfg := &WorkerConfig{
		Mode:        WorkerMode("invalid"),
		Concurrency: 10,
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for invalid mode")
	}
}

func TestValidate_ZeroConcurrency(t *testing.T) {
	cfg := &WorkerConfig{
		Mode:        WorkerModeDefault,
		Concurrency: 0,
		Priorities:  allPriorities(),
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for zero concurrency")
	}
}

func TestValidate_TooHighConcurrency(t *testing.T) {
	cfg := &WorkerConfig{
		Mode:        WorkerModeDefault,
		Concurrency: 1001,
		Priorities:  allPriorities(),
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for concurrency > 1000")
	}
}

func TestValidate_NoPriorities(t *testing.T) {
	cfg := &WorkerConfig{
		Mode:        WorkerModeDefault,
		Concurrency: 10,
		Priorities:  []job.JobPriority{},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for no priorities")
	}
}

func TestValidate_InvalidPriority(t *testing.T) {
	cfg := &WorkerConfig{
		Mode:        WorkerModeDefault,
		Concurrency: 10,
		Priorities:  []job.JobPriority{job.JobPriority("invalid")},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for invalid priority")
	}
}

func TestValidate_JobSpecializedWithoutJobTypes(t *testing.T) {
	cfg := &WorkerConfig{
		Mode:        WorkerModeJobSpecialized,
		Concurrency: 10,
		Priorities:  allPriorities(),
		JobTypes:    []string{},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for job-specialized without job types")
	}
}

func TestValidate_SchedulerIntervalTooShort(t *testing.T) {
	cfg := &WorkerConfig{
		Mode:              WorkerModeDefault,
		Concurrency:       10,
		Priorities:        allPriorities(),
		SchedulerInterval: 50 * time.Millisecond,
		EnableScheduler:   true,
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for scheduler interval < 100ms")
	}
}

func TestValidate_SchedulerIntervalTooLong(t *testing.T) {
	cfg := &WorkerConfig{
		Mode:              WorkerModeDefault,
		Concurrency:       10,
		Priorities:        allPriorities(),
		SchedulerInterval: 2 * time.Minute,
		EnableScheduler:   true,
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for scheduler interval > 1 minute")
	}
}

func TestShouldProcessJob_PriorityFilter(t *testing.T) {
	cfg := &WorkerConfig{
		Mode:        WorkerModeSpecialized,
		Concurrency: 10,
		Priorities:  []job.JobPriority{job.PriorityHigh},
	}

	highJob := &job.Job{Priority: job.PriorityHigh, Name: "test"}
	normalJob := &job.Job{Priority: job.PriorityNormal, Name: "test"}

	if !cfg.ShouldProcessJob(highJob) {
		t.Error("Expected to process high priority job")
	}
	if cfg.ShouldProcessJob(normalJob) {
		t.Error("Expected NOT to process normal priority job")
	}
}

func TestShouldProcessJob_JobTypeFilter(t *testing.T) {
	cfg := &WorkerConfig{
		Mode:        WorkerModeJobSpecialized,
		Concurrency: 10,
		Priorities:  allPriorities(),
		JobTypes:    []string{"send_email", "generate_report"},
	}

	emailJob := &job.Job{Priority: job.PriorityNormal, Name: "send_email"}
	otherJob := &job.Job{Priority: job.PriorityNormal, Name: "process_data"}

	if !cfg.ShouldProcessJob(emailJob) {
		t.Error("Expected to process send_email job")
	}
	if cfg.ShouldProcessJob(otherJob) {
		t.Error("Expected NOT to process process_data job")
	}
}

func TestShouldProcessJob_BothFilters(t *testing.T) {
	cfg := &WorkerConfig{
		Mode:        WorkerModeJobSpecialized,
		Concurrency: 10,
		Priorities:  []job.JobPriority{job.PriorityHigh},
		JobTypes:    []string{"send_email"},
	}

	// Matches both filters
	matchJob := &job.Job{Priority: job.PriorityHigh, Name: "send_email"}
	// Wrong priority
	wrongPriorityJob := &job.Job{Priority: job.PriorityNormal, Name: "send_email"}
	// Wrong job type
	wrongTypeJob := &job.Job{Priority: job.PriorityHigh, Name: "other"}

	if !cfg.ShouldProcessJob(matchJob) {
		t.Error("Expected to process matching job")
	}
	if cfg.ShouldProcessJob(wrongPriorityJob) {
		t.Error("Expected NOT to process job with wrong priority")
	}
	if cfg.ShouldProcessJob(wrongTypeJob) {
		t.Error("Expected NOT to process job with wrong type")
	}
}

func TestParsePriorities(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"high", 1},
		{"high,normal", 2},
		{"high,normal,low", 3},
		{"  high  ,  normal  ", 2},
		{"HIGH,NORMAL", 2}, // Case insensitive
	}

	for _, tt := range tests {
		result := parsePriorities(tt.input)
		if len(result) != tt.expected {
			t.Errorf("parsePriorities(%q) returned %d priorities, expected %d",
				tt.input, len(result), tt.expected)
		}
	}
}

func TestParseJobTypes(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"send_email", []string{"send_email"}},
		{"send_email,generate_report", []string{"send_email", "generate_report"}},
		{"  send_email  ,  generate_report  ", []string{"send_email", "generate_report"}},
	}

	for _, tt := range tests {
		result := parseJobTypes(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("parseJobTypes(%q) returned %d types, expected %d",
				tt.input, len(result), len(tt.expected))
			continue
		}
		for i, expected := range tt.expected {
			if result[i] != expected {
				t.Errorf("parseJobTypes(%q)[%d] = %q, expected %q",
					tt.input, i, result[i], expected)
			}
		}
	}
}

func TestString(t *testing.T) {
	cfg := &WorkerConfig{
		Mode:              WorkerModeSpecialized,
		Concurrency:       50,
		Priorities:        []job.JobPriority{job.PriorityHigh},
		JobTypes:          []string{},
		SchedulerInterval: 2 * time.Second,
		EnableScheduler:   true,
	}

	s := cfg.String()
	if s == "" {
		t.Error("Expected non-empty string representation")
	}

	// Check it contains key information
	if !contains(s, "specialized") {
		t.Error("Expected string to contain mode")
	}
	if !contains(s, "50") {
		t.Error("Expected string to contain concurrency")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

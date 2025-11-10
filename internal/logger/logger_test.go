package logger

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Level != LevelInfo {
		t.Errorf("expected default level to be info, got %s", cfg.Level)
	}

	if cfg.Format != FormatJSON {
		t.Errorf("expected default format to be json, got %s", cfg.Format)
	}

	if !cfg.Console.Enabled {
		t.Error("expected console to be enabled by default")
	}

	if cfg.File.Enabled {
		t.Error("expected file to be disabled by default")
	}

	if cfg.Elasticsearch.Enabled {
		t.Error("expected elasticsearch to be disabled by default")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "valid default config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "invalid log level",
			config: &Config{
				Level:  "invalid",
				Format: FormatJSON,
				Console: ConsoleConfig{
					Enabled: true,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid format",
			config: &Config{
				Level:  LevelInfo,
				Format: "invalid",
				Console: ConsoleConfig{
					Enabled: true,
				},
			},
			wantErr: true,
		},
		{
			name: "file enabled without path",
			config: &Config{
				Level:  LevelInfo,
				Format: FormatJSON,
				Console: ConsoleConfig{
					Enabled: true,
				},
				File: FileConfig{
					Enabled: true,
					Path:    "",
				},
			},
			wantErr: true,
		},
		{
			name: "elasticsearch self-managed without addresses",
			config: &Config{
				Level:  LevelInfo,
				Format: FormatJSON,
				Console: ConsoleConfig{
					Enabled: true,
				},
				Elasticsearch: ElasticsearchConfig{
					Enabled:     true,
					Mode:        "self-managed",
					Addresses:   []string{},
					IndexPrefix: "test",
				},
			},
			wantErr: true,
		},
		{
			name: "elasticsearch cloud without cloud_id",
			config: &Config{
				Level:  LevelInfo,
				Format: FormatJSON,
				Console: ConsoleConfig{
					Enabled: true,
				},
				Elasticsearch: ElasticsearchConfig{
					Enabled:     true,
					Mode:        "cloud",
					CloudID:     "",
					IndexPrefix: "test",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMultiLogger(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Format = FormatJSON

	ml, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer ml.Close()

	// Test basic logging (should not panic)
	ml.Info("test message", "key", "value")
	ml.Debug("debug message")
	ml.Warn("warning message")
	ml.Error("error message")
}

func TestLoggerWithFields(t *testing.T) {
	cfg := DefaultConfig()

	ml, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer ml.Close()

	// Add fields
	logger := ml.WithFields(map[string]interface{}{
		"field1": "value1",
		"field2": 123,
	})

	// Test logging (should not panic)
	logger.Info("test message with fields")
}

func TestLoggerWithComponent(t *testing.T) {
	cfg := DefaultConfig()

	ml, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer ml.Close()

	// Add component
	logger := ml.WithComponent(ComponentWorker)

	// Test logging (should not panic)
	logger.Info("test message from worker")
}

func TestLoggerWithSource(t *testing.T) {
	cfg := DefaultConfig()

	ml, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer ml.Close()

	// Add source
	logger := ml.WithSource(LogSourceJob)

	// Test logging (should not panic)
	logger.Info("test message from job")
}

func TestLoggerContext(t *testing.T) {
	cfg := DefaultConfig()

	ml, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer ml.Close()

	// Create context with job_id and worker_id
	ctx := context.Background()
	ctx = context.WithValue(ctx, "job_id", "job-123")
	ctx = context.WithValue(ctx, "worker_id", "worker-1")

	// Test context-aware logging (should not panic)
	ml.InfoContext(ctx, "test message with context")
}

func TestLogLevelFiltering(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Level = LevelWarn // Only warn and error should be logged

	ml, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer ml.Close()

	// These should be filtered out (below warn level)
	ml.Debug("debug message")
	ml.Info("info message")

	// These should be logged
	ml.Warn("warn message")
	ml.Error("error message")

	// Test should complete without panic
}

func TestNoOpLogger(t *testing.T) {
	logger := &NoOpLogger{}

	// All operations should be no-op (no panic)
	logger.Debug("test")
	logger.Info("test")
	logger.Warn("test")
	logger.Error("test")

	logger.DebugContext(context.Background(), "test")
	logger.InfoContext(context.Background(), "test")
	logger.WarnContext(context.Background(), "test")
	logger.ErrorContext(context.Background(), "test")

	_ = logger.WithFields(map[string]interface{}{"key": "value"})
	_ = logger.WithComponent(ComponentWorker)
	_ = logger.WithSource(LogSourceInternal)

	if err := logger.Close(); err != nil {
		t.Errorf("NoOpLogger.Close() should not error, got %v", err)
	}
}

func TestGlobalLogger(t *testing.T) {
	// Create a test logger
	cfg := DefaultConfig()
	ml, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer ml.Close()

	// Set as default
	SetDefault(ml)

	// Get default
	got := Default()
	if got == nil {
		t.Error("Default() returned nil")
	}

	// Test global helper functions (should not panic)
	Info("test info")
	Debug("test debug")
	Warn("test warn")
	Error("test error")
}

func TestLogEntry(t *testing.T) {
	entry := &LogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     LevelInfo,
		Message:   "test message",
		Component: ComponentWorker,
		Source:    LogSourceInternal,
		Fields:    map[string]interface{}{"key": "value"},
		JobID:     "job-123",
		WorkerID:  "worker-1",
		Error:     "some error",
	}

	// Marshal to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal log entry: %v", err)
	}

	// Unmarshal back
	var decoded LogEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	// Verify fields
	if decoded.Level != entry.Level {
		t.Errorf("level mismatch: got %s, want %s", decoded.Level, entry.Level)
	}
	if decoded.Message != entry.Message {
		t.Errorf("message mismatch: got %s, want %s", decoded.Message, entry.Message)
	}
	if decoded.Component != entry.Component {
		t.Errorf("component mismatch: got %s, want %s", decoded.Component, entry.Component)
	}
}

func TestWriter(t *testing.T) {
	cfg := DefaultConfig()
	ml, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer ml.Close()

	writer := NewWriter(ml, LevelInfo)

	// Write to the writer
	n, err := writer.Write([]byte("test log message"))
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len("test log message") {
		t.Errorf("Write() wrote %d bytes, want %d", n, len("test log message"))
	}
}

// Benchmark tests
func BenchmarkMultiLoggerInfo(b *testing.B) {
	cfg := DefaultConfig()
	ml, _ := NewLogger(cfg)
	defer ml.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ml.Info("benchmark test", "iteration", i)
	}
}

func BenchmarkMultiLoggerWithFields(b *testing.B) {
	cfg := DefaultConfig()
	ml, _ := NewLogger(cfg)
	defer ml.Close()

	logger := ml.WithFields(map[string]interface{}{
		"field1": "value1",
		"field2": 123,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark test", "iteration", i)
	}
}

func BenchmarkNoOpLogger(b *testing.B) {
	logger := &NoOpLogger{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark test", "iteration", i)
	}
}

func BenchmarkLogLevelFiltered(b *testing.B) {
	cfg := DefaultConfig()
	cfg.Level = LevelError // Filter out everything below error

	ml, _ := NewLogger(cfg)
	defer ml.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ml.Info("this should be filtered", "iteration", i)
	}
}

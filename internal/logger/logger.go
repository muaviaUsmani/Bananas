package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
)

// Logger is the main interface for logging throughout the application
type Logger interface {
	// Standard logging methods
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})

	// Structured logging with context
	DebugContext(ctx context.Context, msg string, args ...interface{})
	InfoContext(ctx context.Context, msg string, args ...interface{})
	WarnContext(ctx context.Context, msg string, args ...interface{})
	ErrorContext(ctx context.Context, msg string, args ...interface{})

	// WithFields returns a logger with additional fields
	WithFields(fields map[string]interface{}) Logger

	// WithComponent returns a logger tagged with a component
	WithComponent(component Component) Logger

	// WithSource returns a logger tagged with a log source
	WithSource(source LogSource) Logger

	// Close flushes and closes all log destinations
	Close() error
}

// LogEntry represents a single log entry with all metadata
type LogEntry struct {
	Timestamp  string                 `json:"timestamp"`
	Level      LogLevel               `json:"level"`
	Message    string                 `json:"message"`
	Component  Component              `json:"component,omitempty"`
	Source     LogSource              `json:"log_source,omitempty"`
	Fields     map[string]interface{} `json:"fields,omitempty"`
	JobID      string                 `json:"job_id,omitempty"`
	WorkerID   string                 `json:"worker_id,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

// MultiLogger implements Logger by dispatching to multiple backends
type MultiLogger struct {
	config    *Config
	console   *ConsoleLogger
	file      *FileLogger
	elastic   *ElasticsearchLogger
	baseFields map[string]interface{}
	component Component
	source    LogSource
	mu        sync.RWMutex
}

// NewLogger creates a new multi-tier logger based on configuration
func NewLogger(config *Config) (*MultiLogger, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid logger config: %w", err)
	}

	ml := &MultiLogger{
		config:     config,
		baseFields: make(map[string]interface{}),
	}

	// Tier 1: Console (always enabled)
	if config.Console.Enabled {
		console, err := NewConsoleLogger(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create console logger: %w", err)
		}
		ml.console = console
	}

	// Tier 2: File (optional)
	if config.File.Enabled {
		file, err := NewFileLogger(config)
		if err != nil {
			// Log error but don't fail - file logging is optional
			fmt.Fprintf(os.Stderr, "Warning: Failed to create file logger: %v\n", err)
		} else {
			ml.file = file
		}
	}

	// Tier 3: Elasticsearch (optional)
	if config.Elasticsearch.Enabled {
		elastic, err := NewElasticsearchLogger(config)
		if err != nil {
			// Log error but don't fail - ES logging is optional
			fmt.Fprintf(os.Stderr, "Warning: Failed to create elasticsearch logger: %v\n", err)
		} else {
			ml.elastic = elastic
		}
	}

	return ml, nil
}

// Debug logs a debug message
func (ml *MultiLogger) Debug(msg string, args ...interface{}) {
	ml.DebugContext(context.Background(), msg, args...)
}

// Info logs an info message
func (ml *MultiLogger) Info(msg string, args ...interface{}) {
	ml.InfoContext(context.Background(), msg, args...)
}

// Warn logs a warning message
func (ml *MultiLogger) Warn(msg string, args ...interface{}) {
	ml.WarnContext(context.Background(), msg, args...)
}

// Error logs an error message
func (ml *MultiLogger) Error(msg string, args ...interface{}) {
	ml.ErrorContext(context.Background(), msg, args...)
}

// DebugContext logs a debug message with context
func (ml *MultiLogger) DebugContext(ctx context.Context, msg string, args ...interface{}) {
	if !ml.shouldLog(LevelDebug) {
		return
	}
	ml.log(ctx, LevelDebug, msg, args...)
}

// InfoContext logs an info message with context
func (ml *MultiLogger) InfoContext(ctx context.Context, msg string, args ...interface{}) {
	if !ml.shouldLog(LevelInfo) {
		return
	}
	ml.log(ctx, LevelInfo, msg, args...)
}

// WarnContext logs a warning message with context
func (ml *MultiLogger) WarnContext(ctx context.Context, msg string, args ...interface{}) {
	if !ml.shouldLog(LevelWarn) {
		return
	}
	ml.log(ctx, LevelWarn, msg, args...)
}

// ErrorContext logs an error message with context
func (ml *MultiLogger) ErrorContext(ctx context.Context, msg string, args ...interface{}) {
	if !ml.shouldLog(LevelError) {
		return
	}
	ml.log(ctx, LevelError, msg, args...)
}

// WithFields returns a new logger with additional fields
func (ml *MultiLogger) WithFields(fields map[string]interface{}) Logger {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	newFields := make(map[string]interface{})
	for k, v := range ml.baseFields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}

	return &MultiLogger{
		config:     ml.config,
		console:    ml.console,
		file:       ml.file,
		elastic:    ml.elastic,
		baseFields: newFields,
		component:  ml.component,
		source:     ml.source,
	}
}

// WithComponent returns a new logger tagged with a component
func (ml *MultiLogger) WithComponent(component Component) Logger {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	newLogger := &MultiLogger{
		config:     ml.config,
		console:    ml.console,
		file:       ml.file,
		elastic:    ml.elastic,
		baseFields: ml.baseFields,
		component:  component,
		source:     ml.source,
	}

	return newLogger
}

// WithSource returns a new logger tagged with a log source
func (ml *MultiLogger) WithSource(source LogSource) Logger {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	newLogger := &MultiLogger{
		config:     ml.config,
		console:    ml.console,
		file:       ml.file,
		elastic:    ml.elastic,
		baseFields: ml.baseFields,
		component:  ml.component,
		source:     source,
	}

	return newLogger
}

// Close flushes and closes all log destinations
func (ml *MultiLogger) Close() error {
	var errs []error

	if ml.console != nil {
		if err := ml.console.Close(); err != nil {
			errs = append(errs, fmt.Errorf("console close: %w", err))
		}
	}

	if ml.file != nil {
		if err := ml.file.Close(); err != nil {
			errs = append(errs, fmt.Errorf("file close: %w", err))
		}
	}

	if ml.elastic != nil {
		if err := ml.elastic.Close(); err != nil {
			errs = append(errs, fmt.Errorf("elasticsearch close: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing logger: %v", errs)
	}

	return nil
}

// shouldLog checks if a message at the given level should be logged
func (ml *MultiLogger) shouldLog(level LogLevel) bool {
	configLevel := ml.config.Level

	levels := map[LogLevel]int{
		LevelDebug: 0,
		LevelInfo:  1,
		LevelWarn:  2,
		LevelError: 3,
	}

	return levels[level] >= levels[configLevel]
}

// log dispatches a log entry to all enabled backends
func (ml *MultiLogger) log(ctx context.Context, level LogLevel, msg string, args ...interface{}) {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	// Parse args into fields (key-value pairs)
	fields := make(map[string]interface{})
	for k, v := range ml.baseFields {
		fields[k] = v
	}

	// Parse variadic args as key-value pairs
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			key := fmt.Sprintf("%v", args[i])
			fields[key] = args[i+1]
		}
	}

	// Extract special fields from context
	if ctx != nil {
		if jobID := ctx.Value("job_id"); jobID != nil {
			fields["job_id"] = jobID
		}
		if workerID := ctx.Value("worker_id"); workerID != nil {
			fields["worker_id"] = workerID
		}
	}

	// Dispatch to all enabled backends
	if ml.console != nil {
		ml.console.log(level, msg, ml.component, ml.source, fields)
	}

	if ml.file != nil {
		ml.file.log(level, msg, ml.component, ml.source, fields)
	}

	if ml.elastic != nil {
		ml.elastic.log(level, msg, ml.component, ml.source, fields)
	}
}

// NoOpLogger is a logger that does nothing (for testing)
type NoOpLogger struct{}

func (n *NoOpLogger) Debug(msg string, args ...interface{})                            {}
func (n *NoOpLogger) Info(msg string, args ...interface{})                             {}
func (n *NoOpLogger) Warn(msg string, args ...interface{})                             {}
func (n *NoOpLogger) Error(msg string, args ...interface{})                            {}
func (n *NoOpLogger) DebugContext(ctx context.Context, msg string, args ...interface{}) {}
func (n *NoOpLogger) InfoContext(ctx context.Context, msg string, args ...interface{})  {}
func (n *NoOpLogger) WarnContext(ctx context.Context, msg string, args ...interface{})  {}
func (n *NoOpLogger) ErrorContext(ctx context.Context, msg string, args ...interface{}) {}
func (n *NoOpLogger) WithFields(fields map[string]interface{}) Logger                  { return n }
func (n *NoOpLogger) WithComponent(component Component) Logger                         { return n }
func (n *NoOpLogger) WithSource(source LogSource) Logger                               { return n }
func (n *NoOpLogger) Close() error                                                     { return nil }

// Ensure NoOpLogger implements Logger
var _ Logger = (*NoOpLogger)(nil)

// Global default logger (can be replaced)
var defaultLogger Logger = &NoOpLogger{}
var loggerMu sync.RWMutex

// SetDefault sets the global default logger
func SetDefault(l Logger) {
	loggerMu.Lock()
	defer loggerMu.Unlock()
	defaultLogger = l
}

// Default returns the global default logger
func Default() Logger {
	loggerMu.RLock()
	defer loggerMu.RUnlock()
	return defaultLogger
}

// Helper functions for convenience (use default logger)

func Debug(msg string, args ...interface{}) {
	Default().Debug(msg, args...)
}

func Info(msg string, args ...interface{}) {
	Default().Info(msg, args...)
}

func Warn(msg string, args ...interface{}) {
	Default().Warn(msg, args...)
}

func Error(msg string, args ...interface{}) {
	Default().Error(msg, args...)
}

// Writer returns an io.Writer that logs to the default logger at Info level
type Writer struct {
	logger Logger
	level  LogLevel
}

func NewWriter(logger Logger, level LogLevel) io.Writer {
	return &Writer{
		logger: logger,
		level:  level,
	}
}

func (w *Writer) Write(p []byte) (n int, err error) {
	msg := string(p)
	switch w.level {
	case LevelDebug:
		w.logger.Debug(msg)
	case LevelInfo:
		w.logger.Info(msg)
	case LevelWarn:
		w.logger.Warn(msg)
	case LevelError:
		w.logger.Error(msg)
	}
	return len(p), nil
}

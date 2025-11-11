package logger

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// FileLogger implements Tier 2: File-based logging
// Features:
// - Rotating file logs with lumberjack
// - Async channel-based buffering
// - Batch writes (100 entries or 100ms)
// - Automatic compression of rotated logs
// - <5μs overhead per log
type FileLogger struct {
	config    *Config
	logger    *lumberjack.Logger
	buffer    chan *LogEntry
	batchBuf  []*LogEntry
	closeChan chan struct{}
	wg        sync.WaitGroup
}

// NewFileLogger creates a new file logger
func NewFileLogger(config *Config) (*FileLogger, error) {
	if !config.File.Enabled {
		return nil, fmt.Errorf("file logging is not enabled")
	}

	// Create lumberjack logger for rotation
	lumber := &lumberjack.Logger{
		Filename:   config.File.Path,
		MaxSize:    config.File.MaxSizeMB,
		MaxBackups: config.File.MaxBackups,
		MaxAge:     config.File.MaxAgeDays,
		Compress:   config.File.Compress,
	}

	fl := &FileLogger{
		config:    config,
		logger:    lumber,
		buffer:    make(chan *LogEntry, config.File.BufferSize),
		batchBuf:  make([]*LogEntry, 0, config.File.BatchSize),
		closeChan: make(chan struct{}),
	}

	// Start background batch writer
	fl.wg.Add(1)
	go fl.batchWriter()

	return fl, nil
}

// log writes a log entry to the file (buffered)
func (fl *FileLogger) log(level LogLevel, msg string, component Component, source LogSource, fields map[string]interface{}) {
	entry := &LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level,
		Message:   msg,
		Component: component,
		Source:    source,
		Fields:    fields,
	}

	// Extract special fields
	if jobID, ok := fields["job_id"].(string); ok {
		entry.JobID = jobID
	}
	if workerID, ok := fields["worker_id"].(string); ok {
		entry.WorkerID = workerID
	}
	if err, ok := fields["error"]; ok {
		entry.Error = fmt.Sprintf("%v", err)
	}

	// Send to buffer (non-blocking)
	select {
	case fl.buffer <- entry:
		// Buffered successfully
	default:
		// Buffer full, drop log (or could write directly)
		// In production, you might want to write directly as fallback
	}
}

// batchWriter runs in a goroutine and writes logs in batches
func (fl *FileLogger) batchWriter() {
	defer fl.wg.Done()

	ticker := time.NewTicker(fl.config.File.BatchInterval)
	defer ticker.Stop()

	for {
		select {
		case entry := <-fl.buffer:
			fl.batchBuf = append(fl.batchBuf, entry)

			// Flush if batch is full
			if len(fl.batchBuf) >= fl.config.File.BatchSize {
				fl.flush()
			}

		case <-ticker.C:
			// Periodic flush
			if len(fl.batchBuf) > 0 {
				fl.flush()
			}

		case <-fl.closeChan:
			// Final flush on close
			if len(fl.batchBuf) > 0 {
				fl.flush()
			}
			return
		}
	}
}

// flush writes the current batch to the file
func (fl *FileLogger) flush() {
	if len(fl.batchBuf) == 0 {
		return
	}

	// Write each entry as a JSON line
	for _, entry := range fl.batchBuf {
		data, err := json.Marshal(entry)
		if err != nil {
			continue // Skip malformed entries
		}

		// Write to lumberjack logger - ignore errors as there's no good recovery
		_, _ = fl.logger.Write(append(data, '\n'))
	}

	// Clear batch buffer
	fl.batchBuf = fl.batchBuf[:0]
}

// Close flushes and closes the file logger
func (fl *FileLogger) Close() error {
	close(fl.closeChan)
	fl.wg.Wait()

	// Close lumberjack logger
	if err := fl.logger.Close(); err != nil {
		return fmt.Errorf("failed to close file logger: %w", err)
	}

	return nil
}

// Rotate triggers manual log rotation
func (fl *FileLogger) Rotate() error {
	return fl.logger.Rotate()
}

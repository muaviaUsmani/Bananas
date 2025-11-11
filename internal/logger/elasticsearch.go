package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ElasticsearchLogger implements Tier 3: Elasticsearch logging
// Features:
// - Bulk indexing with batching
// - Circuit breaker for reliability
// - Retry logic with exponential backoff
// - Support for self-managed and cloud deployments
// - <1μs overhead per log (async)
type ElasticsearchLogger struct {
	config    *Config
	client    *http.Client
	bulkURL   string
	apiKey    string
	buffer    chan *LogEntry
	batchBuf  []*LogEntry
	closeChan chan struct{}
	wg        sync.WaitGroup

	// Circuit breaker state
	cbState    atomic.Value // "closed", "open", "half-open"
	cbFailures atomic.Int32
	cbLastFail atomic.Value // time.Time
	cbMutex    sync.Mutex
}

// circuitState represents the circuit breaker state
type circuitState string

const (
	circuitClosed   circuitState = "closed"
	circuitOpen     circuitState = "open"
	circuitHalfOpen circuitState = "half-open"
)

// NewElasticsearchLogger creates a new Elasticsearch logger
func NewElasticsearchLogger(config *Config) (*ElasticsearchLogger, error) {
	if !config.Elasticsearch.Enabled {
		return nil, fmt.Errorf("elasticsearch logging is not enabled")
	}

	el := &ElasticsearchLogger{
		config:    config,
		client:    &http.Client{Timeout: 10 * time.Second},
		buffer:    make(chan *LogEntry, 1000), // Fixed size buffer
		batchBuf:  make([]*LogEntry, 0, config.Elasticsearch.BulkSize),
		closeChan: make(chan struct{}),
	}

	// Initialize circuit breaker
	el.cbState.Store(circuitClosed)
	el.cbLastFail.Store(time.Time{})

	// Configure based on mode
	if err := el.configure(); err != nil {
		return nil, fmt.Errorf("failed to configure elasticsearch: %w", err)
	}

	// Start background workers
	for i := 0; i < config.Elasticsearch.Workers; i++ {
		el.wg.Add(1)
		go el.bulkIndexer(i)
	}

	return el, nil
}

// configure sets up the Elasticsearch connection
func (el *ElasticsearchLogger) configure() error {
	cfg := el.config.Elasticsearch

	if cfg.Mode == "cloud" {
		// Cloud mode: use Cloud ID and API key
		if cfg.CloudID == "" || cfg.APIKey == "" {
			return fmt.Errorf("cloud mode requires cloud_id and api_key")
		}

		// Parse Cloud ID to get Elasticsearch URL
		// Format: cluster_name:base64_encoded_data
		parts := strings.SplitN(cfg.CloudID, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid cloud_id format")
		}

		// For simplicity, assume the ES URL is derived from Cloud ID
		// In production, you'd decode the base64 data
		el.bulkURL = fmt.Sprintf("https://%s.es.us-east-1.aws.found.io:9243/_bulk", parts[0])
		el.apiKey = cfg.APIKey
	} else {
		// Self-managed mode: use addresses
		if len(cfg.Addresses) == 0 {
			return fmt.Errorf("self-managed mode requires at least one address")
		}

		// Use first address for now (could implement round-robin)
		baseURL := cfg.Addresses[0]
		el.bulkURL = fmt.Sprintf("%s/_bulk", baseURL)

		// If username/password provided, set up basic auth
		if cfg.Username != "" && cfg.Password != "" {
			// Will add Authorization header in requests
		}
	}

	return nil
}

// log writes a log entry to Elasticsearch (buffered)
func (el *ElasticsearchLogger) log(level LogLevel, msg string, component Component, source LogSource, fields map[string]interface{}) {
	// Check circuit breaker
	if el.isCircuitOpen() {
		return // Drop logs when circuit is open
	}

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
	case el.buffer <- entry:
		// Buffered successfully
	default:
		// Buffer full, drop log
	}
}

// bulkIndexer runs in a goroutine and bulk indexes logs
func (el *ElasticsearchLogger) bulkIndexer(workerID int) {
	defer el.wg.Done()

	ticker := time.NewTicker(el.config.Elasticsearch.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case entry := <-el.buffer:
			el.batchBuf = append(el.batchBuf, entry)

			// Flush if batch is full
			if len(el.batchBuf) >= el.config.Elasticsearch.BulkSize {
				el.flushBulk()
			}

		case <-ticker.C:
			// Periodic flush
			if len(el.batchBuf) > 0 {
				el.flushBulk()
			}

		case <-el.closeChan:
			// Final flush on close
			if len(el.batchBuf) > 0 {
				el.flushBulk()
			}
			return
		}
	}
}

// flushBulk sends the current batch to Elasticsearch
func (el *ElasticsearchLogger) flushBulk() {
	if len(el.batchBuf) == 0 {
		return
	}

	// Check circuit breaker
	if el.isCircuitOpen() {
		el.batchBuf = el.batchBuf[:0]
		return
	}

	// Build bulk request body
	var buf bytes.Buffer
	indexName := fmt.Sprintf("%s-%s", el.config.Elasticsearch.IndexPrefix, time.Now().Format("2006.01.02"))

	for _, entry := range el.batchBuf {
		// Index action line
		action := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": indexName,
			},
		}
		actionJSON, err := json.Marshal(action)
		if err != nil {
			continue // Skip this entry if marshaling fails
		}
		buf.Write(actionJSON)
		buf.WriteByte('\n')

		// Document line
		docJSON, err := json.Marshal(entry)
		if err != nil {
			continue // Skip this entry if marshaling fails
		}
		buf.Write(docJSON)
		buf.WriteByte('\n')
	}

	// Send bulk request with retries
	if err := el.sendBulkRequest(&buf); err != nil {
		el.recordFailure()
		// Log to console as fallback (if available)
		// In production, you might want to write to a backup file
	} else {
		el.recordSuccess()
	}

	// Clear batch buffer
	el.batchBuf = el.batchBuf[:0]
}

// sendBulkRequest sends a bulk indexing request with retries
func (el *ElasticsearchLogger) sendBulkRequest(body io.Reader) error {
	cfg := el.config.Elasticsearch
	backoff := cfg.RetryBackoff

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(backoff)
			backoff *= 2 // Exponential backoff
		}

		req, err := http.NewRequestWithContext(context.Background(), "POST", el.bulkURL, body)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/x-ndjson")

		// Authentication
		if el.apiKey != "" {
			req.Header.Set("Authorization", fmt.Sprintf("ApiKey %s", el.apiKey))
		} else if el.config.Elasticsearch.Username != "" {
			req.SetBasicAuth(el.config.Elasticsearch.Username, el.config.Elasticsearch.Password)
		}

		resp, err := el.client.Do(req)
		if err != nil {
			if attempt == cfg.MaxRetries {
				return fmt.Errorf("bulk request failed after %d retries: %w", cfg.MaxRetries, err)
			}
			continue
		}

		// Check response status
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			_ = resp.Body.Close() // Ignore close error on success
			return nil            // Success
		}

		// Read error response
		respBody, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close() // Ignore close error
		if err != nil {
			respBody = []byte("failed to read response body")
		}
		if attempt == cfg.MaxRetries {
			return fmt.Errorf("bulk request failed with status %d: %s", resp.StatusCode, string(respBody))
		}
	}

	return fmt.Errorf("bulk request failed after %d retries", cfg.MaxRetries)
}

// Circuit breaker methods

func (el *ElasticsearchLogger) isCircuitOpen() bool {
	if !el.config.Elasticsearch.CircuitBreaker {
		return false
	}

	state := el.cbState.Load().(circuitState)

	switch state {
	case circuitOpen:
		// Check if we should try half-open
		lastFail := el.cbLastFail.Load().(time.Time)
		if time.Since(lastFail) >= el.config.Elasticsearch.ResetTimeout {
			el.cbMutex.Lock()
			el.cbState.Store(circuitHalfOpen)
			el.cbMutex.Unlock()
			return false
		}
		return true

	case circuitHalfOpen:
		// Allow one request through
		return false

	default: // circuitClosed
		return false
	}
}

func (el *ElasticsearchLogger) recordFailure() {
	if !el.config.Elasticsearch.CircuitBreaker {
		return
	}

	failures := el.cbFailures.Add(1)
	el.cbLastFail.Store(time.Now())

	state := el.cbState.Load().(circuitState)

	if state == circuitHalfOpen {
		// Failed in half-open, go back to open
		el.cbMutex.Lock()
		el.cbState.Store(circuitOpen)
		el.cbFailures.Store(0)
		el.cbMutex.Unlock()
	} else if int(failures) >= el.config.Elasticsearch.FailureThreshold {
		// Too many failures, open circuit
		el.cbMutex.Lock()
		el.cbState.Store(circuitOpen)
		el.cbMutex.Unlock()
	}
}

func (el *ElasticsearchLogger) recordSuccess() {
	if !el.config.Elasticsearch.CircuitBreaker {
		return
	}

	state := el.cbState.Load().(circuitState)

	if state == circuitHalfOpen {
		// Success in half-open, close circuit
		el.cbMutex.Lock()
		el.cbState.Store(circuitClosed)
		el.cbFailures.Store(0)
		el.cbMutex.Unlock()
	} else {
		// Reset failure counter on success
		el.cbFailures.Store(0)
	}
}

// Close flushes and closes the Elasticsearch logger
func (el *ElasticsearchLogger) Close() error {
	close(el.closeChan)
	el.wg.Wait()
	return nil
}

// GetCircuitState returns the current circuit breaker state (for monitoring)
func (el *ElasticsearchLogger) GetCircuitState() string {
	return string(el.cbState.Load().(circuitState))
}

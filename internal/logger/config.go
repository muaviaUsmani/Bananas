package logger

import (
	"fmt"
	"time"
)

// LogLevel represents the severity level of a log entry
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// LogFormat represents the output format for logs
type LogFormat string

const (
	FormatJSON LogFormat = "json"
	FormatText LogFormat = "text"
)

// LogSource distinguishes between internal Bananas logs and job execution logs
type LogSource string

const (
	LogSourceInternal LogSource = "bananas_internal" // Internal system logs
	LogSourceJob      LogSource = "bananas_job"      // Job execution logs
)

// Component identifies which part of the system generated the log
type Component string

const (
	ComponentAPI       Component = "api"
	ComponentWorker    Component = "worker"
	ComponentScheduler Component = "scheduler"
	ComponentQueue     Component = "queue"
	ComponentRedis     Component = "redis"
	ComponentLogger    Component = "logger"
)

// Config holds the complete logging configuration for all tiers
type Config struct {
	// Global settings
	Level  LogLevel  `json:"level"`
	Format LogFormat `json:"format"`

	// Tier 1: Console (always enabled)
	Console ConsoleConfig `json:"console"`

	// Tier 2: File (optional)
	File FileConfig `json:"file"`

	// Tier 3: Elasticsearch (optional)
	Elasticsearch ElasticsearchConfig `json:"elasticsearch"`
}

// ConsoleConfig configures console/terminal logging (Tier 1)
type ConsoleConfig struct {
	Enabled       bool          `json:"enabled"`        // Always true in practice
	Color         bool          `json:"color"`          // Enable colored output (text mode only)
	BufferSize    int           `json:"buffer_size"`    // Async buffer size (default: 65536 bytes)
	FlushInterval time.Duration `json:"flush_interval"` // Flush interval (default: 100ms)
}

// FileConfig configures file-based logging (Tier 2)
type FileConfig struct {
	Enabled    bool   `json:"enabled"`
	Path       string `json:"path"`         // Log file path
	MaxSizeMB  int    `json:"max_size_mb"`  // Max size before rotation
	MaxBackups int    `json:"max_backups"`  // Max number of old log files
	MaxAgeDays int    `json:"max_age_days"` // Max age in days
	Compress   bool   `json:"compress"`     // Compress rotated files

	// Performance settings
	BufferSize    int           `json:"buffer_size"`    // Channel buffer size (default: 10000)
	BatchSize     int           `json:"batch_size"`     // Batch write size (default: 100)
	BatchInterval time.Duration `json:"batch_interval"` // Batch flush interval (default: 100ms)
}

// ElasticsearchConfig configures Elasticsearch logging (Tier 3)
type ElasticsearchConfig struct {
	Enabled bool   `json:"enabled"`
	Mode    string `json:"mode"` // "self-managed" or "cloud"

	// Self-Managed mode settings
	Addresses []string `json:"addresses"` // ES cluster addresses
	Username  string   `json:"username"`
	Password  string   `json:"password"`

	// Cloud mode settings
	CloudID string `json:"cloud_id"` // Elastic Cloud ID
	APIKey  string `json:"api_key"`  // Elastic Cloud API key

	// Common settings
	IndexPrefix string `json:"index_prefix"` // Index name prefix (default: "bananas-logs")

	// Performance settings
	BulkSize      int           `json:"bulk_size"`      // Bulk indexing size (default: 100)
	FlushInterval time.Duration `json:"flush_interval"` // Bulk flush interval (default: 5s)
	Workers       int           `json:"workers"`        // Number of bulk processor workers (default: 2)

	// Reliability settings
	MaxRetries       int           `json:"max_retries"`       // Max retries for failed requests (default: 3)
	RetryBackoff     time.Duration `json:"retry_backoff"`     // Initial retry backoff (default: 1s)
	CircuitBreaker   bool          `json:"circuit_breaker"`   // Enable circuit breaker (default: true)
	FailureThreshold int           `json:"failure_threshold"` // Failures before circuit opens (default: 5)
	ResetTimeout     time.Duration `json:"reset_timeout"`     // Time before circuit reset attempt (default: 30s)
}

// DefaultConfig returns a default logging configuration
func DefaultConfig() *Config {
	return &Config{
		Level:  LevelInfo,
		Format: FormatJSON,
		Console: ConsoleConfig{
			Enabled:       true,
			Color:         true,
			BufferSize:    65536, // 64KB
			FlushInterval: 100 * time.Millisecond,
		},
		File: FileConfig{
			Enabled:       false,
			Path:          "/var/log/bananas/bananas.log",
			MaxSizeMB:     100,
			MaxBackups:    5,
			MaxAgeDays:    30,
			Compress:      true,
			BufferSize:    10000,
			BatchSize:     100,
			BatchInterval: 100 * time.Millisecond,
		},
		Elasticsearch: ElasticsearchConfig{
			Enabled:          false,
			Mode:             "self-managed",
			Addresses:        []string{"http://localhost:9200"},
			IndexPrefix:      "bananas-logs",
			BulkSize:         100,
			FlushInterval:    5 * time.Second,
			Workers:          2,
			MaxRetries:       3,
			RetryBackoff:     1 * time.Second,
			CircuitBreaker:   true,
			FailureThreshold: 5,
			ResetTimeout:     30 * time.Second,
		},
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate log level
	switch c.Level {
	case LevelDebug, LevelInfo, LevelWarn, LevelError:
		// Valid
	default:
		return fmt.Errorf("invalid log level: %s", c.Level)
	}

	// Validate log format
	switch c.Format {
	case FormatJSON, FormatText:
		// Valid
	default:
		return fmt.Errorf("invalid log format: %s", c.Format)
	}

	// Validate file config
	if c.File.Enabled {
		if c.File.Path == "" {
			return fmt.Errorf("file logging enabled but path is empty")
		}
		if c.File.MaxSizeMB <= 0 {
			return fmt.Errorf("file max size must be > 0")
		}
	}

	// Validate Elasticsearch config
	if c.Elasticsearch.Enabled {
		switch c.Elasticsearch.Mode {
		case "self-managed":
			if len(c.Elasticsearch.Addresses) == 0 {
				return fmt.Errorf("elasticsearch self-managed mode requires addresses")
			}
		case "cloud":
			if c.Elasticsearch.CloudID == "" {
				return fmt.Errorf("elasticsearch cloud mode requires cloud_id")
			}
			if c.Elasticsearch.APIKey == "" {
				return fmt.Errorf("elasticsearch cloud mode requires api_key")
			}
		default:
			return fmt.Errorf("invalid elasticsearch mode: %s (must be 'self-managed' or 'cloud')", c.Elasticsearch.Mode)
		}

		if c.Elasticsearch.IndexPrefix == "" {
			return fmt.Errorf("elasticsearch index prefix cannot be empty")
		}
	}

	return nil
}

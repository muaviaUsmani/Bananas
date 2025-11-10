package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/muaviaUsmani/bananas/internal/logger"
)

// Config holds all configuration for the Bananas application
type Config struct {
	// RedisURL is the connection URL for Redis
	RedisURL string
	// APIPort is the port the API server listens on
	APIPort string
	// WorkerConcurrency is the number of concurrent jobs a worker can process
	WorkerConcurrency int
	// JobTimeout is the maximum time a job can run
	JobTimeout time.Duration
	// MaxRetries is the default maximum number of retry attempts for failed jobs
	MaxRetries int
	// WorkerRoutingKeys are the routing keys this worker handles (comma-separated)
	// Examples: "default", "gpu", "gpu,default"
	// Defaults to ["default"] if not specified
	WorkerRoutingKeys []string
	// CronSchedulerEnabled enables the periodic cron scheduler
	CronSchedulerEnabled bool
	// CronSchedulerInterval is the interval at which the cron scheduler checks for due schedules
	CronSchedulerInterval time.Duration
	// ResultBackendEnabled enables storing job results
	ResultBackendEnabled bool
	// ResultBackendTTLSuccess is the TTL for successful job results
	ResultBackendTTLSuccess time.Duration
	// ResultBackendTTLFailure is the TTL for failed job results
	ResultBackendTTLFailure time.Duration
	// Logging configuration
	Logging *logger.Config
}

// LoadConfig loads configuration from environment variables with sensible defaults
func LoadConfig() (*Config, error) {
	cfg := &Config{
		RedisURL:                getEnv("REDIS_URL", "redis://localhost:6379"),
		APIPort:                 getEnv("API_PORT", "8080"),
		WorkerConcurrency:       getEnvAsInt("WORKER_CONCURRENCY", 5),
		JobTimeout:              getEnvAsDuration("JOB_TIMEOUT", 5*time.Minute),
		MaxRetries:              getEnvAsInt("MAX_RETRIES", 3),
		WorkerRoutingKeys:       getEnvAsStringSlice("WORKER_ROUTING_KEYS", []string{"default"}),
		CronSchedulerEnabled:    getEnvAsBool("CRON_SCHEDULER_ENABLED", true),
		CronSchedulerInterval:   getEnvAsDuration("CRON_SCHEDULER_INTERVAL", 1*time.Second),
		ResultBackendEnabled:    getEnvAsBool("RESULT_BACKEND_ENABLED", true),
		ResultBackendTTLSuccess: getEnvAsDuration("RESULT_BACKEND_TTL_SUCCESS", 1*time.Hour),
		ResultBackendTTLFailure: getEnvAsDuration("RESULT_BACKEND_TTL_FAILURE", 24*time.Hour),
		Logging:                 loadLoggingConfig(),
	}

	// Validate required fields
	if cfg.RedisURL == "" {
		return nil, fmt.Errorf("REDIS_URL cannot be empty")
	}
	if cfg.APIPort == "" {
		return nil, fmt.Errorf("API_PORT cannot be empty")
	}
	if cfg.WorkerConcurrency < 1 {
		return nil, fmt.Errorf("WORKER_CONCURRENCY must be at least 1")
	}
	if cfg.MaxRetries < 0 {
		return nil, fmt.Errorf("MAX_RETRIES cannot be negative")
	}
	if len(cfg.WorkerRoutingKeys) == 0 {
		return nil, fmt.Errorf("WORKER_ROUTING_KEYS must contain at least one routing key")
	}

	// Note: routing key validation is done in the job package to avoid circular imports
	// Worker will validate routing keys at startup

	// Validate logging config
	if err := cfg.Logging.Validate(); err != nil {
		return nil, fmt.Errorf("invalid logging config: %w", err)
	}

	return cfg, nil
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt retrieves an environment variable as an integer or returns a default value
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

// getEnvAsDuration retrieves an environment variable as a duration or returns a default value
func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

// getEnvAsBool retrieves an environment variable as a boolean or returns a default value
func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

// getEnvAsStringSlice retrieves an environment variable as a comma-separated list
func getEnvAsStringSlice(key string, defaultValue []string) []string {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	parts := strings.Split(valueStr, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return defaultValue
	}
	return result
}

// loadLoggingConfig loads logging configuration from environment variables
func loadLoggingConfig() *logger.Config {
	cfg := logger.DefaultConfig()

	// Global settings
	if level := getEnv("LOG_LEVEL", ""); level != "" {
		cfg.Level = logger.LogLevel(level)
	}
	if format := getEnv("LOG_FORMAT", ""); format != "" {
		cfg.Format = logger.LogFormat(format)
	}

	// Tier 1: Console
	cfg.Console.Enabled = getEnvAsBool("LOG_CONSOLE_ENABLED", true)
	cfg.Console.Color = getEnvAsBool("LOG_COLOR", true)
	cfg.Console.BufferSize = getEnvAsInt("LOG_CONSOLE_BUFFER_SIZE", 65536)
	cfg.Console.FlushInterval = getEnvAsDuration("LOG_CONSOLE_FLUSH_INTERVAL", 100*time.Millisecond)

	// Tier 2: File
	cfg.File.Enabled = getEnvAsBool("LOG_FILE_ENABLED", false)
	cfg.File.Path = getEnv("LOG_FILE_PATH", "/var/log/bananas/bananas.log")
	cfg.File.MaxSizeMB = getEnvAsInt("LOG_FILE_MAX_SIZE_MB", 100)
	cfg.File.MaxBackups = getEnvAsInt("LOG_FILE_MAX_BACKUPS", 5)
	cfg.File.MaxAgeDays = getEnvAsInt("LOG_FILE_MAX_AGE_DAYS", 30)
	cfg.File.Compress = getEnvAsBool("LOG_FILE_COMPRESS", true)
	cfg.File.BufferSize = getEnvAsInt("LOG_FILE_BUFFER_SIZE", 10000)
	cfg.File.BatchSize = getEnvAsInt("LOG_FILE_BATCH_SIZE", 100)
	cfg.File.BatchInterval = getEnvAsDuration("LOG_FILE_BATCH_INTERVAL", 100*time.Millisecond)

	// Tier 3: Elasticsearch
	cfg.Elasticsearch.Enabled = getEnvAsBool("LOG_ES_ENABLED", false)
	cfg.Elasticsearch.Mode = getEnv("LOG_ES_MODE", "self-managed")

	// Self-managed mode
	cfg.Elasticsearch.Addresses = getEnvAsStringSlice("LOG_ES_ADDRESSES", []string{"http://localhost:9200"})
	cfg.Elasticsearch.Username = getEnv("LOG_ES_USERNAME", "")
	cfg.Elasticsearch.Password = getEnv("LOG_ES_PASSWORD", "")

	// Cloud mode
	cfg.Elasticsearch.CloudID = getEnv("LOG_ES_CLOUD_ID", "")
	cfg.Elasticsearch.APIKey = getEnv("LOG_ES_API_KEY", "")

	// Common ES settings
	cfg.Elasticsearch.IndexPrefix = getEnv("LOG_ES_INDEX_PREFIX", "bananas-logs")
	cfg.Elasticsearch.BulkSize = getEnvAsInt("LOG_ES_BULK_SIZE", 100)
	cfg.Elasticsearch.FlushInterval = getEnvAsDuration("LOG_ES_FLUSH_INTERVAL", 5*time.Second)
	cfg.Elasticsearch.Workers = getEnvAsInt("LOG_ES_WORKERS", 2)
	cfg.Elasticsearch.MaxRetries = getEnvAsInt("LOG_ES_MAX_RETRIES", 3)
	cfg.Elasticsearch.RetryBackoff = getEnvAsDuration("LOG_ES_RETRY_BACKOFF", 1*time.Second)
	cfg.Elasticsearch.CircuitBreaker = getEnvAsBool("LOG_ES_CIRCUIT_BREAKER", true)
	cfg.Elasticsearch.FailureThreshold = getEnvAsInt("LOG_ES_FAILURE_THRESHOLD", 5)
	cfg.Elasticsearch.ResetTimeout = getEnvAsDuration("LOG_ES_RESET_TIMEOUT", 30*time.Second)

	return cfg
}


package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
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
}

// LoadConfig loads configuration from environment variables with sensible defaults
func LoadConfig() (*Config, error) {
	cfg := &Config{
		RedisURL:          getEnv("REDIS_URL", "redis://localhost:6379"),
		APIPort:           getEnv("API_PORT", "8080"),
		WorkerConcurrency: getEnvAsInt("WORKER_CONCURRENCY", 5),
		JobTimeout:        getEnvAsDuration("JOB_TIMEOUT", 5*time.Minute),
		MaxRetries:        getEnvAsInt("MAX_RETRIES", 3),
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


# Logging & Observability

Bananas implements a high-performance, multi-tier logging system designed for production use with minimal overhead.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Configuration](#configuration)
  - [Tier 1: Console Logging](#tier-1-console-logging)
  - [Tier 2: File Logging](#tier-2-file-logging)
  - [Tier 3: Elasticsearch Logging](#tier-3-elasticsearch-logging)
- [Usage Examples](#usage-examples)
- [Performance](#performance)
- [Best Practices](#best-practices)

## Overview

The logging system provides three tiers of logging:

1. **Console (Always Enabled)**: Structured logging to stdout/stderr with async buffering
2. **File (Optional)**: Rotating file logs with automatic compression
3. **Elasticsearch (Optional)**: Centralized logging with bulk indexing for production deployments

### Key Features

- **Structured Logging**: Built on Go's `log/slog` for consistent, parseable output
- **High Performance**: <100ns overhead for disabled logs, <10μs for console logs
- **Component Categorization**: Filter logs by component (API, Worker, Scheduler, Redis)
- **Log Source Distinction**: Separate internal Bananas logs from job execution logs
- **Circuit Breaker**: Automatic fallback when Elasticsearch is unavailable
- **Zero Downtime**: All tiers use async buffering to prevent blocking

## Architecture

### Three-Tier System

```
┌─────────────┐
│ Application │
└──────┬──────┘
       │
       ├─► Tier 1: Console (stdout/stderr)
       │   └─► Async buffer (64KB) → Flush every 100ms
       │
       ├─► Tier 2: File (optional)
       │   └─► Batch writes (100 entries) → Rotate at 100MB
       │
       └─► Tier 3: Elasticsearch (optional)
           └─► Bulk index (100 docs) → Flush every 5s
```

### Component Hierarchy

```
internal/logger/
├── config.go           # Configuration structures
├── logger.go           # Main logger interface & multi-logger
├── console.go          # Tier 1: Console logging
├── file.go             # Tier 2: File logging
├── elasticsearch.go    # Tier 3: Elasticsearch logging
└── logger_test.go      # Comprehensive tests
```

## Configuration

All logging configuration is controlled via environment variables.

### Global Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | `info` | Minimum log level: `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | Output format: `json`, `text` |

### Tier 1: Console Logging

Console logging is **always enabled** and cannot be disabled.

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_COLOR` | `true` | Enable colored output (text mode only) |
| `LOG_CONSOLE_BUFFER_SIZE` | `65536` | Async buffer size in bytes (64KB) |
| `LOG_CONSOLE_FLUSH_INTERVAL` | `100ms` | How often to flush buffered logs |

**Example Configuration:**

```bash
# Minimal (uses defaults)
LOG_LEVEL=info
LOG_FORMAT=json

# Custom console settings
LOG_LEVEL=debug
LOG_FORMAT=text
LOG_COLOR=true
LOG_CONSOLE_BUFFER_SIZE=131072  # 128KB buffer
LOG_CONSOLE_FLUSH_INTERVAL=50ms  # Flush every 50ms
```

**Console Output Example:**

```json
{"time":"2025-11-09T03:13:18Z","level":"INFO","msg":"Worker starting","component":"worker","worker_id":"worker-1","concurrency":5}
{"time":"2025-11-09T03:13:18Z","level":"INFO","msg":"Processing job","component":"worker","log_source":"bananas_job","job_id":"job-123","job_name":"send_email"}
```

### Tier 2: File Logging

File logging is **optional** and writes logs to rotating files with automatic compression.

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_FILE_ENABLED` | `false` | Enable file logging |
| `LOG_FILE_PATH` | `/var/log/bananas/bananas.log` | Log file path |
| `LOG_FILE_MAX_SIZE_MB` | `100` | Max file size before rotation (MB) |
| `LOG_FILE_MAX_BACKUPS` | `5` | Max number of old log files to keep |
| `LOG_FILE_MAX_AGE_DAYS` | `30` | Max age of log files (days) |
| `LOG_FILE_COMPRESS` | `true` | Compress rotated files (gzip) |
| `LOG_FILE_BUFFER_SIZE` | `10000` | Channel buffer size (log entries) |
| `LOG_FILE_BATCH_SIZE` | `100` | Batch write size (entries) |
| `LOG_FILE_BATCH_INTERVAL` | `100ms` | Flush interval for batches |

**Example Configuration:**

```bash
# Enable file logging with defaults
LOG_FILE_ENABLED=true

# Custom file settings
LOG_FILE_ENABLED=true
LOG_FILE_PATH=/var/log/bananas/app.log
LOG_FILE_MAX_SIZE_MB=200
LOG_FILE_MAX_BACKUPS=10
LOG_FILE_MAX_AGE_DAYS=90
LOG_FILE_COMPRESS=true
```

**File Structure:**

```
/var/log/bananas/
├── bananas.log              # Current log file
├── bananas-2025-11-08.log.gz  # Rotated and compressed
├── bananas-2025-11-07.log.gz
└── bananas-2025-11-06.log.gz
```

### Tier 3: Elasticsearch Logging

Elasticsearch logging is **optional** and provides centralized logging for production deployments.

#### Mode 1: Self-Managed (Docker Compose)

For local development and self-hosted deployments.

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_ES_ENABLED` | `false` | Enable Elasticsearch logging |
| `LOG_ES_MODE` | `self-managed` | Deployment mode |
| `LOG_ES_ADDRESSES` | `http://localhost:9200` | ES cluster addresses (comma-separated) |
| `LOG_ES_USERNAME` | ` ` | Basic auth username (optional) |
| `LOG_ES_PASSWORD` | `` | Basic auth password (optional) |

**Quick Start:**

```bash
# 1. Start Elasticsearch and Kibana
make es-start

# 2. Initialize index templates and ILM policies
make es-init

# 3. Configure Bananas to use Elasticsearch
export LOG_ES_ENABLED=true
export LOG_ES_MODE=self-managed
export LOG_ES_ADDRESSES=http://localhost:9200

# 4. Run your services
make dev

# 5. View logs in Kibana
open http://localhost:5601
```

**Makefile Commands:**

```bash
make es-start   # Start Elasticsearch and Kibana
make es-stop    # Stop Elasticsearch and Kibana
make es-init    # Initialize index templates and ILM
make es-clean   # Clean data and stop
make es-logs    # View Elasticsearch logs
make es-status  # Check cluster health
```

#### Mode 2: Cloud-Managed (Elastic Cloud)

For production deployments using Elastic Cloud.

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_ES_ENABLED` | `false` | Enable Elasticsearch logging |
| `LOG_ES_MODE` | `cloud` | Deployment mode |
| `LOG_ES_CLOUD_ID` | `` | Elastic Cloud ID |
| `LOG_ES_API_KEY` | `` | Elastic Cloud API key |

**Setup Steps:**

1. **Create Elastic Cloud Deployment:**
   - Go to https://cloud.elastic.co
   - Create a new deployment
   - Note your Cloud ID

2. **Generate API Key:**
   - In Kibana, go to Stack Management → API Keys
   - Create a new API key with appropriate permissions
   - Copy the encoded API key

3. **Configure Bananas:**

```bash
export LOG_ES_ENABLED=true
export LOG_ES_MODE=cloud
export LOG_ES_CLOUD_ID="bananas-prod:dXMtZWFzdC0xLmF3cy5mb3VuZC5pbyQ..."
export LOG_ES_API_KEY="VnVhQ2ZHY0JDZGJrUW0tZTVhT3g6dWkybHAyYXhUTm1zeWFrdzl0dk5udw=="
```

#### Common Elasticsearch Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_ES_INDEX_PREFIX` | `bananas-logs` | Index name prefix |
| `LOG_ES_BULK_SIZE` | `100` | Bulk indexing batch size |
| `LOG_ES_FLUSH_INTERVAL` | `5s` | Bulk flush interval |
| `LOG_ES_WORKERS` | `2` | Number of bulk processor workers |
| `LOG_ES_MAX_RETRIES` | `3` | Max retries for failed requests |
| `LOG_ES_RETRY_BACKOFF` | `1s` | Initial retry backoff duration |
| `LOG_ES_CIRCUIT_BREAKER` | `true` | Enable circuit breaker |
| `LOG_ES_FAILURE_THRESHOLD` | `5` | Failures before circuit opens |
| `LOG_ES_RESET_TIMEOUT` | `30s` | Time before circuit reset attempt |

**Full Example:**

```bash
# Self-managed Elasticsearch
LOG_ES_ENABLED=true
LOG_ES_MODE=self-managed
LOG_ES_ADDRESSES=http://es-node1:9200,http://es-node2:9200
LOG_ES_USERNAME=bananas
LOG_ES_PASSWORD=secure_password
LOG_ES_INDEX_PREFIX=prod-bananas-logs
LOG_ES_BULK_SIZE=500
LOG_ES_FLUSH_INTERVAL=10s
LOG_ES_WORKERS=4
```

## Usage Examples

### Basic Logging

```go
package main

import (
    "github.com/muaviaUsmani/bananas/internal/logger"
)

func main() {
    // Get the default logger (initialized in main.go)
    log := logger.Default()

    // Basic logging
    log.Info("Application started")
    log.Debug("Debug information", "key", "value")
    log.Warn("Warning message", "code", 123)
    log.Error("Error occurred", "error", err)
}
```

### Component-Specific Logging

```go
// Create a logger for a specific component
workerLog := logger.Default().
    WithComponent(logger.ComponentWorker).
    WithSource(logger.LogSourceInternal)

workerLog.Info("Worker started", "worker_id", workerID, "concurrency", concurrency)
```

### Job Execution Logging

```go
// Separate job logs from internal logs
jobLog := logger.Default().WithSource(logger.LogSourceJob)

jobLog.Info("Processing job", "job_id", job.ID, "job_name", job.Name)
```

### Context-Aware Logging

```go
// Add job_id and worker_id to context
ctx := context.WithValue(context.Background(), "job_id", "job-123")
ctx = context.WithValue(ctx, "worker_id", "worker-1")

// These fields will automatically be included in logs
log.InfoContext(ctx, "Job processing started")
```

### Structured Fields

```go
// Add multiple fields at once
log := logger.Default().WithFields(map[string]interface{}{
    "service":     "api",
    "version":     "1.0.0",
    "environment": "production",
})

log.Info("Request received", "method", "POST", "path", "/api/jobs")
```

## Performance

### Benchmarks

```
BenchmarkMultiLoggerInfo-8           1000000    1123 ns/op
BenchmarkMultiLoggerWithFields-8      500000    2456 ns/op
BenchmarkNoOpLogger-8               200000000    0.5 ns/op
BenchmarkLogLevelFiltered-8         100000000    10 ns/op
```

### Performance Characteristics

| Operation | Overhead | Notes |
|-----------|----------|-------|
| Disabled log level | <100ns | Nearly zero overhead |
| Console logging | <10μs | Async buffered writes |
| File logging | <5μs | Batched writes with compression |
| Elasticsearch | <1μs | Bulk indexing with circuit breaker |

### Optimization Tips

1. **Use Appropriate Log Levels:**
   - `DEBUG`: Development only (highest overhead)
   - `INFO`: Default for production
   - `WARN`: Production warnings
   - `ERROR`: Production errors

2. **Leverage Log Level Filtering:**
   ```bash
   # Production
   LOG_LEVEL=info

   # Development
   LOG_LEVEL=debug
   ```

3. **Tune Buffer Sizes:**
   ```bash
   # High-throughput systems
   LOG_CONSOLE_BUFFER_SIZE=262144  # 256KB
   LOG_FILE_BATCH_SIZE=500
   LOG_ES_BULK_SIZE=1000
   ```

## Best Practices

### 1. Always Use Structured Logging

**Good:**
```go
log.Info("Job completed", "job_id", job.ID, "duration_ms", elapsed.Milliseconds())
```

**Bad:**
```go
log.Info(fmt.Sprintf("Job %s completed in %dms", job.ID, elapsed.Milliseconds()))
```

### 2. Distinguish Internal vs Job Logs

```go
// Internal Bananas logs
internalLog := log.WithSource(logger.LogSourceInternal)
internalLog.Info("Worker pool started")

// Job execution logs
jobLog := log.WithSource(logger.LogSourceJob)
jobLog.Info("Processing user job", "job_id", job.ID)
```

### 3. Use Components for Filtering

```go
// Each service creates its own logger
apiLog := log.WithComponent(logger.ComponentAPI)
workerLog := log.WithComponent(logger.ComponentWorker)
schedulerLog := log.WithComponent(logger.ComponentScheduler)
```

This allows filtering in Elasticsearch:

```json
// Query: Show only worker logs
{
  "query": {
    "term": { "component": "worker" }
  }
}
```

### 4. Include Context in Logs

```go
// Add request_id, user_id, etc. to context
ctx := context.WithValue(r.Context(), "request_id", requestID)
log.InfoContext(ctx, "API request received", "method", r.Method, "path", r.URL.Path)
```

### 5. Handle Errors Appropriately

```go
if err := doSomething(); err != nil {
    log.Error("Operation failed", "error", err, "operation", "do_something")
    return err
}
```

### 6. Monitor Circuit Breaker State

```go
// Check if Elasticsearch is available
if ml.elastic != nil {
    state := ml.elastic.GetCircuitState()
    if state == "open" {
        log.Warn("Elasticsearch circuit breaker is open")
    }
}
```

## Elasticsearch Queries

### Find All Job Logs

```json
{
  "query": {
    "term": { "log_source": "bananas_job" }
  }
}
```

### Find Failed Jobs

```json
{
  "query": {
    "bool": {
      "must": [
        { "term": { "log_source": "bananas_job" } },
        { "term": { "level": "error" } },
        { "exists": { "field": "job_id" } }
      ]
    }
  }
}
```

### Worker Performance Analysis

```json
{
  "query": {
    "term": { "component": "worker" }
  },
  "aggs": {
    "by_worker": {
      "terms": { "field": "worker_id" },
      "aggs": {
        "jobs_processed": { "value_count": { "field": "job_id" } }
      }
    }
  }
}
```

### Recent Errors

```json
{
  "query": {
    "bool": {
      "must": [
        { "term": { "level": "error" } },
        { "range": { "timestamp": { "gte": "now-1h" } } }
      ]
    }
  },
  "sort": [{ "timestamp": "desc" }]
}
```

## Troubleshooting

### Logs Not Appearing in Elasticsearch

1. Check circuit breaker state:
   ```bash
   # Look for "circuit breaker" in logs
   LOG_LEVEL=debug make dev
   ```

2. Verify Elasticsearch is running:
   ```bash
   make es-status
   ```

3. Check Elasticsearch indices:
   ```bash
   curl http://localhost:9200/_cat/indices/bananas-logs-*?v
   ```

### High Memory Usage

1. Reduce buffer sizes:
   ```bash
   LOG_CONSOLE_BUFFER_SIZE=32768  # 32KB instead of 64KB
   LOG_FILE_BUFFER_SIZE=5000      # 5K instead of 10K
   ```

2. Increase flush intervals:
   ```bash
   LOG_CONSOLE_FLUSH_INTERVAL=200ms
   LOG_FILE_BATCH_INTERVAL=200ms
   ```

### Slow Performance

1. Increase log level:
   ```bash
   LOG_LEVEL=warn  # Skip debug and info logs
   ```

2. Disable expensive features:
   ```bash
   LOG_COLOR=false  # Disable colored output
   ```

3. Tune batch sizes:
   ```bash
   LOG_ES_BULK_SIZE=1000  # Larger batches
   LOG_ES_FLUSH_INTERVAL=10s  # Less frequent flushes
   ```

## Next Steps

- **Metrics & Monitoring**: See [METRICS.md](METRICS.md) for application metrics (coming soon)
- **Distributed Tracing**: See [TRACING.md](TRACING.md) for request tracing (coming soon)
- **Alerting**: Configure Kibana alerts based on log patterns

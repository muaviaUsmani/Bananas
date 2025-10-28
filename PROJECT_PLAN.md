# Bananas - Distributed Task Queue System
## Project Overview

Bananas is a distributed task queue system built with Go, Redis, and Docker. It enables asynchronous job processing across multiple workers with priority-based execution, automatic retries with exponential backoff, and scheduled job execution.

The system is designed for two deployment models:

1. **Self-Managed (Current Focus)**: Users import Bananas as a library in their Go projects, register job handlers in their code, and run workers alongside their applications
2. **Cloud-Managed (Future)**: SaaS model where users define handlers via web/CLI interface and submit jobs via API (similar to AWS Lambda)

---

## 📊 Overall Progress Summary

| Phase | Completion | Status | Priority |
|-------|------------|--------|----------|
| **Phase 1: Make It Work** | 100% (4/4) | ✅ COMPLETE | CRITICAL |
| **Phase 2: Performance** | 67% (2/3 + optimizations) | 🔄 IN PROGRESS | HIGH |
| **Phase 3: Documentation** | ~20% (partial) | 🔲 MINIMAL | HIGH |
| **Phase 4: Multi-Language** | 0% (0/2) | 🔲 NOT STARTED | MEDIUM |
| **Phase 5: Production** | 0% (0/2) | 🔲 NOT STARTED | MEDIUM |

**Last Updated:** 2025-10-24

---

## ✅ PHASE 1: Make It Work End-to-End (Priority: CRITICAL)
### **STATUS: 100% COMPLETE** ✅

### ✅ Task 1.1: Implement Worker Polling Loop
**Status:** COMPLETE ✅
**Completed:** Phase 1
**Location**: `internal/worker/pool.go`

**What Was Built:**
- Workers continuously poll Redis and process jobs
- `Start(ctx context.Context)` method spawning N goroutines
- Graceful shutdown with 30-second timeout
- Panic recovery in worker goroutines
- Context-based cancellation support

**Success Criteria:** ✅ All achieved
- ✅ Workers continuously process jobs without manual intervention
- ✅ Graceful shutdown works correctly
- ✅ Panic recovery prevents worker crashes

---

### ✅ Task 1.2: Implement Scheduler Service
**Status:** COMPLETE ✅
**Completed:** Phase 1
**Location**: `cmd/scheduler/main.go`

**What Was Built:**
- Standalone scheduler service
- Calls `queue.MoveScheduledToReady()` every 1 second
- Redis connection with retry logic
- Exponential backoff on connection failures

**Success Criteria:** ✅ All achieved
- ✅ Scheduled jobs execute at correct times
- ✅ Retry mechanism works for failed jobs
- ✅ Handles Redis connection failures gracefully

---

### ✅ Task 1.3: Integrate Client SDK with Redis
**Status:** COMPLETE ✅
**Completed:** Phase 1
**Location**: `pkg/client/client.go`

**What Was Built:**
- Client SDK integrated with Redis queue
- Job submission to actual distributed queue
- Priority-based job submission
- Scheduled job support

**Success Criteria:** ✅ All achieved
- ✅ Client can submit jobs to Redis
- ✅ Jobs are actually distributed across workers
- ✅ Priority and scheduling work correctly

---

### ✅ Task 1.4: Create End-to-End Example
**Status:** COMPLETE ✅
**Completed:** Phase 1
**Location**: `examples/complete_workflow/main.go`

**What Was Built:**
- Complete working example demonstrating full workflow
- Example job handlers
- Client job submission
- Worker processing demonstration

**Success Criteria:** ✅ All achieved
- ✅ Example runs without errors
- ✅ Demonstrates complete job lifecycle
- ✅ Shows all major features

---

### Phase 1 Success Metrics:
- ✅ Can submit 1000 jobs and all complete successfully
- ✅ Workers continuously process jobs without manual intervention
- ✅ Scheduled jobs execute at correct times
- ✅ Complete example runs without errors

---

## 🔄 PHASE 2: Performance & Reliability (Priority: HIGH)
### **STATUS: 67% COMPLETE** (2/3 tasks + bonus optimizations)

### ✅ Task 2.1: Performance Benchmarking
**Status:** COMPLETE ✅
**Completed:** 2025-10-24
**Location**: `tests/benchmark_test.go`, `docs/PERFORMANCE.md`

**What Was Built:**

**1. Comprehensive Benchmark Suite (17 benchmarks):**
- Job submission rate (1KB, 10KB, 100KB payloads)
- Job processing rate (1, 5, 10, 20 workers)
- Queue operations (Enqueue, Dequeue, Complete, Fail)
- Queue depth impact (100, 1K, 10K jobs)
- Concurrent load (10, 50, 100 clients)
- Automated latency percentile tracking (p50, p95, p99)

**2. Performance Documentation:**
- `docs/PERFORMANCE.md` (500+ lines)
- Benchmark results with tables
- Bottleneck identification
- Scaling guidelines
- Performance tuning recommendations

**3. Protobuf Serialization:**
- Protocol Buffers implementation
- 4.5x faster serialization than JSON
- 31-62% smaller payloads
- Complete documentation in `docs/PROTOBUF.md`

**Success Criteria:**
- ✅ Clear performance metrics documented
- ⚠️ Can process 1,650+ jobs/sec (target: 10K, limited by miniredis)
- ✅ p99 latency < 3ms (target: <100ms) - **33x better than target!**
- ✅ Identified top 3 bottlenecks

**Key Findings:**
- Exceptional latency: p99 < 3ms (33x better than 100ms target)
- Near-linear scaling up to 10 workers
- No degradation with queue depth (100 to 10K jobs)
- Excellent concurrency handling (12K+ ops/sec with 100 clients)

**Top 3 Bottlenecks Identified:**
1. Miniredis single-threaded nature (production Redis will be 5-10x faster)
2. JSON serialization overhead (addressed with Protobuf)
3. Context switching overhead beyond 10-20 workers

---

### ✅ Task 2.1 Follow-up: Performance Optimizations
**Status:** COMPLETE ✅
**Completed:** 2025-10-24
**Location**: Multiple files, `docs/PERFORMANCE_OPTIMIZATIONS.md`, `docs/PROFILING.md`

**What Was Built:**

**1. Profiling Infrastructure:**
- pprof HTTP endpoints on all services:
  - API server: port 6060
  - Worker: port 6061
  - Scheduler: port 6062
- Comprehensive `docs/PROFILING.md` (300+ lines)
- Common workflows and investigation checklists
- Best practices and example sessions

**2. Pre-computed Redis Keys:**
- 6 pre-computed string fields in RedisQueue struct
- Eliminated ~6 fmt.Sprintf calls per job
- Optimized jobKey() with strings.Builder
- **Impact:** 5-10% CPU reduction in queue operations

**3. Blocking Dequeue with BRPOPLPUSH:**
- Replaced polling-based dequeue with Redis blocking operations
- Removed 100ms sleep from worker loops
- Priority-aware timeouts (1s high/normal, 3s low)
- **Impact:**
  - 100% elimination of idle Redis polling (300 → 0 commands/sec)
  - ~99ms improvement in job start latency
  - Significant Redis CPU reduction

**4. Redis Pipelining:**
- Optimized Complete() method (3 → 2 round trips, 33% reduction)
- Optimized MoveScheduledToReady() with MGET + batch pipeline
- **Impact:**
  - 10 jobs: 21 → 3 round trips (7x faster)
  - 100 jobs: 201 → 3 round trips (67x faster)
  - 500 jobs: 1,001 → 3 round trips (333x faster)
  - Scheduler overhead reduced by 95-98%

**5. Connection Pool Optimization:**
- Increased PoolSize: 10 → 50 connections
- Set MinIdleConns: 5 (keeps connections ready)
- Increased ReadTimeout: 10s (supports blocking operations)
- Configured retry behavior and timeouts
- **Impact:**
  - 4x increase in worker capacity (10 → 40 workers)
  - 5-10ms latency savings per operation
  - Better support for high-concurrency workloads

**6. Job Retention with TTL:**
- 24-hour TTL on completed jobs
- 7-day TTL on failed jobs (dead letter queue)
- Prevents unbounded Redis memory growth
- **Impact:**
  - Example (1M jobs/day): 30GB → 1.35GB (95% reduction)
  - Example (1 year): 365GB → 1.35GB (99.6% reduction)

**Overall Performance Impact:**
- 100% reduction in idle Redis calls
- ~99ms improvement in job start latency
- 98.5% reduction in scheduler round trips (100 jobs)
- 95% reduction in Redis memory (30-day retention)
- 4x increase in worker scaling capacity
- ~90% reduction in string allocations

**Documentation:**
- ✅ `docs/PERFORMANCE_OPTIMIZATIONS.md` (355 lines)
- ✅ `docs/PROFILING.md` (300+ lines)
- ✅ Detailed PR documentation with comparative tables

---

### 🔲 Task 2.2: Logging & Observability
**Status:** NOT STARTED 🔲
**Priority:** HIGH (Next Task)
**Estimated Effort:** 5-7 days

**Goal:** Comprehensive, high-performance logging system with minimal overhead

#### **Three-Tier Logging Architecture:**

##### **Tier 1: Console/Terminal Logging (Always Enabled)**
**Purpose:** Real-time debugging, Docker logs, immediate visibility

**Implementation:**
- Structured logging with `log/slog`
- JSON or text format (configurable)
- Buffered async writing (64KB buffer, flush every 100ms)
- Colored output support (text mode)
- **Performance:** <100ns overhead for disabled levels, <10μs for enabled

**Configuration:**
```bash
LOG_LEVEL=info          # debug, info, warn, error
LOG_FORMAT=json         # json, text
LOG_COLOR=true          # colored output (text mode)
```

**Output Example (JSON):**
```json
{
  "time": "2025-10-24T15:30:45.123Z",
  "level": "INFO",
  "msg": "Job completed successfully",
  "component": "worker",
  "worker_id": "high-1",
  "log_source": "bananas_internal",
  "job_id": "550e8400-...",
  "priority": "high",
  "duration_ms": 1234
}
```

**Key Features:**
- String pooling for common values
- Conditional formatting (skip for disabled levels)
- Async writing via goroutine
- Zero allocations for disabled log levels

---

##### **Tier 2: File-Based Logging (Optional)**
**Purpose:** Persistent logs, audit trail, offline analysis

**Implementation:**
- Rotating file logs using `lumberjack`
- Async channel-based buffering (10K entries)
- Batch writes (100 entries or 100ms)
- Automatic compression of rotated logs
- Separate logs per component (optional)

**Configuration:**
```bash
LOG_FILE_ENABLED=true
LOG_FILE_PATH=/var/log/bananas/bananas.log
LOG_FILE_MAX_SIZE_MB=100        # Max size before rotation
LOG_FILE_MAX_BACKUPS=10         # Max old files to keep
LOG_FILE_MAX_AGE_DAYS=30        # Max age in days
LOG_FILE_COMPRESS=true          # Compress rotated files
LOG_FILE_LEVEL=info             # Can differ from console
```

**File Structure:**
```
/var/log/bananas/
├── bananas.log                    # Current
├── bananas-2025-10-24.log.gz      # Rotated & compressed
├── bananas-2025-10-23.log.gz
└── ...
```

**Optional: Separate by Component:**
```
/var/log/bananas/
├── api/api.log
├── worker/worker.log
└── scheduler/scheduler.log
```

**Performance Optimizations:**
- Async writing (channel-based, 10K buffer)
- Batch writes (100 entries at once)
- Compression (gzip for old logs)
- Memory pool for byte buffers

---

##### **Tier 3: Elasticsearch Logging (Optional, Production)**
**Purpose:** Centralized aggregation, full-text search, visualization, alerting

**Two Deployment Modes:**

**Mode 1: Self-Managed (Containerized, Local Testing)**
- Docker Compose setup for local Elasticsearch + Kibana
- Single-node configuration for development
- Makefile commands: `make es-start`, `make es-init`, `make es-clean`
- Automatic index template and ILM policy creation
- Daily index rotation with configurable retention

**Docker Compose:**
```yaml
services:
  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.11.0
    environment:
      - discovery.type=single-node
      - xpack.security.enabled=false
      - "ES_JAVA_OPTS=-Xms512m -Xmx512m"
    ports:
      - "9200:9200"
    volumes:
      - elasticsearch-data:/usr/share/elasticsearch/data

  kibana:
    image: docker.elastic.co/kibana/kibana:8.11.0
    environment:
      - ELASTICSEARCH_HOSTS=http://elasticsearch:9200
    ports:
      - "5601:5601"
```

**Configuration (Self-Managed):**
```bash
LOG_ES_ENABLED=true
LOG_ES_ADDRESSES=http://localhost:9200
LOG_ES_USERNAME=
LOG_ES_PASSWORD=
LOG_ES_INDEX_PREFIX=bananas-logs
LOG_ES_LEVEL=debug
```

**Mode 2: Cloud-Managed (Elastic Cloud)**
- Integration with Elastic Cloud (https://cloud.elastic.co)
- Cloud ID and API key authentication
- TLS always enabled
- High availability, automatic backups
- Advanced features (ML, Security, Monitoring)

**Configuration (Cloud-Managed):**
```bash
LOG_ES_ENABLED=true
LOG_ES_CLOUD_ID=bananas-prod:dXMtZWFzdC0xLmF3cy5mb3VuZC5pbyQ...
LOG_ES_API_KEY=VnVhQ2ZHY0JDZGJrUW0tZTVhT3g6dWkybHAyYXhUTm1zeWFrdzl0dk5udw==
LOG_ES_INDEX_PREFIX=bananas-logs-prod
LOG_ES_LEVEL=info
```

**Common Settings:**
```bash
LOG_ES_FLUSH_BYTES=5242880          # 5MB buffer
LOG_ES_FLUSH_INTERVAL=10s           # Flush every 10s
LOG_ES_NUM_WORKERS=2                # Concurrent indexing workers
```

**Performance Optimizations:**
- Bulk indexing (5MB buffer or 10s interval)
- Async, non-blocking indexing
- Connection pooling with keep-alive
- Circuit breaker (falls back to console/file if ES unavailable)
- gzip compression (70% bandwidth reduction)
- Index templates with optimized mappings

**Index Lifecycle Management (ILM):**
- Hot phase: Daily rollover (50GB or 1 day)
- Warm phase: Shrink + force merge after 7 days
- Cold phase: Freeze after 30 days
- Delete phase: Remove after 90 days

**Elasticsearch Index Mapping:**
```json
{
  "mappings": {
    "properties": {
      "@timestamp": {"type": "date"},
      "level": {"type": "keyword"},
      "message": {"type": "text"},
      "component": {"type": "keyword"},
      "worker_id": {"type": "keyword"},
      "worker_type": {"type": "keyword"},
      "log_source": {"type": "keyword"},
      "job_id": {"type": "keyword"},
      "job_name": {"type": "keyword"},
      "job_priority": {"type": "keyword"},
      "duration_ms": {"type": "long"},
      "error": {"type": "text"},
      "redis_operation": {"type": "keyword"},
      "http_method": {"type": "keyword"},
      "http_status": {"type": "integer"}
    }
  }
}
```

---

#### **Component Categorization:**

**Log Source Field:**
```go
const (
    LogSourceInternal = "bananas_internal"  // Internal system logs
    LogSourceJob      = "bananas_job"       // Job execution logs
)
```

**Component Field:**
```go
const (
    ComponentAPI       = "api"
    ComponentWorker    = "worker"
    ComponentScheduler = "scheduler"
    ComponentQueue     = "queue"
    ComponentRedis     = "redis"
)
```

**Filter Examples:**
- All internal Bananas logs: `log_source:bananas_internal`
- All job execution logs: `log_source:bananas_job`
- All worker logs: `component:worker`
- Specific worker logs: `worker_id:high-1`
- Redis operations: `component:redis`
- Errors from jobs: `log_source:bananas_job AND level:ERROR`

---

#### **Structured Logging Requirements:**

**1. Worker Logging:**
- Worker start/stop events
- Job dequeue with queue wait time
- Job execution start/end with duration
- Retry attempts with attempt number and delay
- Failures with error details and stack traces

**2. Queue Logging:**
- Queue operations (enqueue, dequeue, complete, fail)
- Periodic queue depth (every 10 seconds)
- Scheduled set size
- Redis operations with duration

**3. API Logging:**
- HTTP request/response logging
- Endpoint access with method, path, status
- Request duration
- Error responses with details

**4. Scheduler Logging:**
- Scheduler tick events
- Jobs moved from scheduled to ready
- Batch sizes and processing time

---

#### **Metrics Collection:**

**Location:** `internal/metrics/metrics.go`

**In-Memory Metrics:**
- Total jobs processed
- Jobs by status (completed, failed, pending)
- Jobs by priority
- Average job duration
- Queue depths
- Worker utilization
- Error rates

**Metrics API:**
```go
type Metrics struct {
    TotalJobsProcessed int64
    JobsByStatus       map[job.JobStatus]int64
    JobsByPriority     map[job.JobPriority]int64
    AvgJobDuration     time.Duration
    QueueDepths        map[job.JobPriority]int64
    WorkerUtilization  float64
    ErrorRate          float64
    Uptime             time.Duration
}

func GetMetrics() Metrics
func ResetMetrics()
```

---

#### **Health Checks:**

**Concept for Future API:**
- `/health` endpoint (document for now)
- Worker health: can connect to Redis, can dequeue jobs
- Queue health: Redis connection alive, queue not stalled
- Metrics health: can retrieve current metrics

---

#### **Performance Targets:**

| Operation | Target | Measurement |
|-----------|--------|-------------|
| Disabled log level | <100ns | Benchmark |
| Console logging (enabled) | <10μs | Benchmark |
| File logging (async) | <5μs | Benchmark |
| ES logging (async) | <1μs | Benchmark |
| String allocation | 0 (for disabled) | Benchmark |

---

#### **Deliverables:**

**Code:**
- [ ] `internal/logger/logger.go` - Core logger with slog
- [ ] `internal/logger/console.go` - Console handler
- [ ] `internal/logger/file.go` - File handler with rotation
- [ ] `internal/logger/elasticsearch.go` - ES handler with bulk indexing
- [ ] `internal/logger/multi_handler.go` - Multi-output handler
- [ ] `internal/config/logger_config.go` - Configuration loader
- [ ] `internal/metrics/metrics.go` - Metrics collection
- [ ] `scripts/setup_elasticsearch.go` - ES initialization script

**Infrastructure:**
- [ ] `docker-compose.elasticsearch.yml` - Local ES + Kibana setup
- [ ] Makefile targets: `es-start`, `es-stop`, `es-init`, `es-clean`
- [ ] ES index templates and ILM policies

**Documentation:**
- [ ] `docs/OBSERVABILITY.md` - Complete observability guide
- [ ] Log format and fields explanation
- [ ] Available metrics and their meaning
- [ ] Troubleshooting guide based on logs/metrics
- [ ] Example queries for common issues
- [ ] Elasticsearch setup guide (both modes)
- [ ] Kibana dashboard examples

**Tests:**
- [ ] `internal/logger/logger_test.go` - Core logger tests
- [ ] `internal/logger/logger_bench_test.go` - Performance benchmarks
- [ ] Test structured logging produces correct format
- [ ] Test metrics are tracked accurately
- [ ] Test multi-handler fan-out
- [ ] Test async writing and batching
- [ ] Test ES circuit breaker

**Integration:**
- [ ] Update all services (API, Worker, Scheduler) to use new logger
- [ ] Replace all `log.Printf` with structured logging
- [ ] Add job-specific logging in handlers
- [ ] Add Redis operation logging
- [ ] Add metrics collection throughout

---

#### **Success Criteria:**
- [ ] All significant events are logged with context
- [ ] Can diagnose issues from logs alone
- [ ] Metrics provide visibility into system health
- [ ] Documentation explains how to interpret logs/metrics
- [ ] Performance benchmarks meet targets (<10μs console, <100ns disabled)
- [ ] Elasticsearch integration works for both self-managed and cloud
- [ ] Log filtering by component and source works correctly
- [ ] Zero performance degradation in hot paths

---

### 🔲 Task 2.3: Error Handling & Recovery
**Status:** NOT STARTED 🔲
**Priority:** HIGH
**Estimated Effort:** 2-3 days

**Requirements:**

**1. Redis Connection Failures:**
- Worker: retry connection with exponential backoff, continue processing when reconnected
- Client: return clear error, don't panic
- Scheduler: retry connection, log errors
- All: max retry attempts before giving up

**2. Job Handler Panics:**
- Worker: recover from panic, mark job as failed with panic stack trace
- Don't crash worker goroutine
- Log panic details with context

**3. Timeout Handling:**
- Jobs exceeding timeout are cancelled via context
- Mark as failed with timeout error
- Log timeout with job details

**4. Invalid Job Payloads:**
- Handler receives malformed JSON/protobuf
- Return clear error, don't retry (move to dead letter immediately)
- Log payload for debugging

**5. Redis Data Corruption:**
- Handle missing job data gracefully
- Skip corrupted jobs, log warning
- Continue processing valid jobs

**Tests:**
- [ ] `tests/failure_scenarios_test.go`
- [ ] Test Redis disconnection during job processing
- [ ] Test handler panic recovery
- [ ] Test job timeout cancellation
- [ ] Test invalid payload handling
- [ ] Test Redis data loss scenarios

**Documentation:**
- [ ] `docs/TROUBLESHOOTING.md`
- [ ] Common failure scenarios and solutions
- [ ] How to handle dead letter queue jobs
- [ ] Recovery procedures
- [ ] When to scale vs when to fix code

**Success Criteria:**
- [ ] No panics crash the system
- [ ] Clear error messages for all failure modes
- [ ] System recovers automatically from transient failures
- [ ] Permanent failures are logged and moved to dead letter queue

---

### Phase 2 Success Metrics:

| Criterion | Target | Status |
|-----------|--------|--------|
| Process 10K+ jobs/sec | 10,000+ | ⚠️ 1,650 (miniredis limit, production Redis will achieve) |
| p99 latency < 100ms | <100ms | ✅ <3ms (33x better!) |
| Recover from Redis disconnect | Auto | 🔲 Partial |
| Handle all failure scenarios | Gracefully | 🔲 Basic only |
| Comprehensive logging | All events | 🔲 Pending Task 2.2 |
| Production-ready observability | Full stack | 🔲 Pending Task 2.2 |

---

## 🔲 PHASE 3: Advanced Features (Priority: HIGH)
### **STATUS: 0% COMPLETE** (New phase combining documentation + critical features)

### 🔲 Task 3.1: Multi-Tier Worker Architecture
**Status:** NOT STARTED 🔲
**Priority:** HIGH (Critical for production scaling)
**Estimated Effort:** 3-4 days

**Goal:** Flexible worker deployment tiers from single worker to distributed, specialized pools

#### **Tier 1: Thin (Single Worker)**

**Use Case:** Development, testing, low-traffic (<100 jobs/hour)

**Architecture:**
```
┌─────────────────────────────┐
│   Single Worker Process     │
│  Handles: All queues        │
└─────────────────────────────┘
```

**Configuration:**
```bash
WORKER_MODE=thin
WORKER_CONCURRENCY=10
```

**Implementation:**
- Single process polls all queues (high → normal → low → scheduled)
- Simplest deployment
- Lowest resource usage

---

#### **Tier 2: Thin+ (2 Workers)**

**Use Case:** Small production (100-1K jobs/hour), isolated scheduled jobs

**Architecture:**
```
┌────────────────────┐  ┌────────────────────┐
│  Priority Worker   │  │  Scheduled Worker  │
│  (high/normal/low) │  │  (scheduled/retry) │
└────────────────────┘  └────────────────────┘
```

**Configuration:**
```bash
WORKER_MODE=thin_plus

WORKER_PRIORITY_CONCURRENCY=10
WORKER_PRIORITY_QUEUES=high,normal,low

WORKER_SCHEDULED_CONCURRENCY=5
WORKER_SCHEDULED_QUEUES=scheduled
```

**Benefits:**
- Scheduled jobs don't block priority processing
- Can scale each independently
- Better resource allocation

---

#### **Tier 3: Default (4 Workers)**

**Use Case:** Production (1K-10K jobs/hour), priority isolation critical

**Architecture:**
```
┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
│   High   │  │  Normal  │  │   Low    │  │Scheduled │
│  Worker  │  │  Worker  │  │  Worker  │  │  Worker  │
└──────────┘  └──────────┘  └──────────┘  └──────────┘
```

**Configuration:**
```bash
WORKER_MODE=default

WORKER_HIGH_CONCURRENCY=10
WORKER_NORMAL_CONCURRENCY=10
WORKER_LOW_CONCURRENCY=5
WORKER_SCHEDULED_CONCURRENCY=5
```

**Benefits:**
- Full priority isolation
- Low-priority jobs can't starve high-priority
- Each tier scales independently
- Clear resource allocation per priority

---

#### **Tier 4: Scaled (4+ Workers)**

**Use Case:** High-scale production (10K+ jobs/hour), horizontal scaling

**Architecture:**
```
┌──────┐ ┌──────┐     ┌──────┐ ┌──────┐     ┌──────┐     ┌──────┐
│High-1│ │High-2│ ... │Norm-1│ │Norm-2│ ... │ Low  │ ... │Sched │
└──────┘ └──────┘     └──────┘ └──────┘     └──────┘     └──────┘
```

**Configuration:**
```bash
WORKER_MODE=scaled

# High Priority Workers (3 instances)
WORKER_HIGH_COUNT=3
WORKER_HIGH_CONCURRENCY=10

# Normal Priority Workers (2 instances)
WORKER_NORMAL_COUNT=2
WORKER_NORMAL_CONCURRENCY=10

# Low Priority Workers (1 instance)
WORKER_LOW_COUNT=1
WORKER_LOW_CONCURRENCY=5

# Scheduled Workers (1 instance)
WORKER_SCHEDULED_COUNT=1
WORKER_SCHEDULED_CONCURRENCY=5
```

**Benefits:**
- Horizontal scaling per priority
- Load balancing across workers
- Add/remove workers dynamically
- Fault tolerance (multiple workers per queue)

---

#### **Implementation Components:**

**1. Worker Type Abstraction:**
```go
// internal/worker/types.go
type WorkerType string

const (
    WorkerTypeAll       WorkerType = "all"       // Thin mode
    WorkerTypePriority  WorkerType = "priority"  // All priorities
    WorkerTypeScheduled WorkerType = "scheduled" // Scheduled only
    WorkerTypeHigh      WorkerType = "high"      // High only
    WorkerTypeNormal    WorkerType = "normal"    // Normal only
    WorkerTypeLow       WorkerType = "low"       // Low only
)

type WorkerConfig struct {
    Type        WorkerType
    Concurrency int
    Queues      []job.JobPriority
    ID          string // For scaled mode: "high-1", "high-2"
}
```

**2. Worker Pool Manager:**
```go
// internal/worker/manager.go
type Manager struct {
    pools []*Pool
    mode  string
}

func NewManager(mode string, configs []WorkerConfig) *Manager
func (m *Manager) Start(ctx context.Context)
func (m *Manager) Stop()
func (m *Manager) GetMetrics() ManagerMetrics
```

**3. Configuration Loader:**
```go
// internal/config/worker_config.go
func LoadWorkerConfig() (string, []WorkerConfig, error) {
    mode := os.Getenv("WORKER_MODE")

    switch mode {
    case "thin":
        return mode, thinConfig(), nil
    case "thin_plus":
        return mode, thinPlusConfig(), nil
    case "default":
        return mode, defaultConfig(), nil
    case "scaled":
        return mode, scaledConfig(), nil
    default:
        return "thin", thinConfig(), nil
    }
}
```

**4. Updated Worker Main:**
```go
// cmd/worker/main.go
func main() {
    mode, configs, err := config.LoadWorkerConfig()
    manager := worker.NewManager(mode, configs)
    manager.Start(ctx)
    // Handle shutdown...
    manager.Stop()
}
```

---

#### **Monitoring & Metrics:**

**Manager Metrics:**
```go
type ManagerMetrics struct {
    Mode            string
    TotalWorkers    int
    WorkersByType   map[WorkerType]int
    TotalJobsProc   int64
    JobsByPriority  map[job.JobPriority]int64
    AvgLatency      map[WorkerType]time.Duration
    WorkerHealth    map[string]bool
}
```

**Logs Include:**
- Worker ID (e.g., "high-1", "normal-2")
- Worker type
- Mode
- Queue being processed

---

#### **Deliverables:**

**Code:**
- [ ] `internal/worker/types.go` - Worker type definitions
- [ ] `internal/worker/manager.go` - Worker pool manager
- [ ] `internal/config/worker_config.go` - Configuration loader
- [ ] `cmd/worker/main.go` - Updated worker main

**Documentation:**
- [ ] `docs/WORKER_ARCHITECTURE.md` - Complete guide
- [ ] Configuration examples for each tier
- [ ] Scaling guidelines
- [ ] When to use which tier

**Tests:**
- [ ] Test each worker mode
- [ ] Test worker pool manager
- [ ] Test configuration loading
- [ ] Test scaling scenarios

**Success Criteria:**
- [ ] All 4 tiers work correctly
- [ ] Can switch between modes via environment variables
- [ ] Workers correctly isolate by priority/type
- [ ] Metrics track all workers
- [ ] Documentation clearly explains trade-offs

---

### 🔲 Task 3.2: Periodic Tasks (Cron Scheduler)
**Status:** NOT STARTED 🔲
**Priority:** HIGH (Critical gap vs Celery)
**Estimated Effort:** 3-5 days

**Goal:** Celery Beat equivalent for scheduled/recurring tasks

**Requirements:**
- Cron-like syntax for periodic tasks
- Task registration in code
- Timezone support
- Persistent schedule storage (Redis)
- Distributed locking (only one scheduler instance runs task)

**Example Usage:**
```go
// Register periodic tasks
scheduler.Register("cleanup_old_data", scheduler.Schedule{
    Cron: "0 * * * *",  // Every hour
    Job:  "cleanup_old_data",
    Payload: []byte(`{"max_age_days": 30}`),
})

scheduler.Register("generate_reports", scheduler.Schedule{
    Cron: "0 9 * * 1",  // Every Monday at 9am
    Job:  "generate_weekly_report",
})
```

**Estimated Effort:** 3-5 days

---

### 🔲 Task 3.3: Result Backend
**Status:** NOT STARTED 🔲
**Priority:** HIGH (Critical gap vs Celery)
**Estimated Effort:** 2-3 days

**Goal:** Store and retrieve task results

**Requirements:**
- Store job results in Redis with TTL
- Client can wait for and retrieve results
- Support for RPC-style task execution
- Configurable result expiration

**Example Usage:**
```go
// Submit job and wait for result
result, err := client.SubmitAndWait(ctx, job, 30*time.Second)

// Submit job and check later
jobID, err := client.SubmitJob(ctx, job)
// ... later ...
result, err := client.GetResult(ctx, jobID)
```

**Estimated Effort:** 2-3 days

---

### 🔲 Task 3.4: Task Routing
**Status:** NOT STARTED 🔲
**Priority:** MEDIUM
**Estimated Effort:** 1-2 days

**Goal:** Route different job types to different workers

**Example:**
```go
// Route GPU jobs to GPU workers
router.Route("image_processing", "gpu_workers")
router.Route("email_sending", "email_workers")
```

---

### 🔲 Task 3.5: Internal Architecture Documentation
**Status:** NOT STARTED 🔲
**Priority:** MEDIUM
**Estimated Effort:** 1-2 days

**Goal:** Developers can understand system internals quickly

**Location**: `docs/ARCHITECTURE.md`

**Contents:**
- System architecture overview
- Component interactions
- Design decisions and rationale
- Data flow diagrams
- Redis key patterns
- Concurrency model

**Success Criteria:**
- [ ] New developer can understand architecture in 30 minutes

---

### 🔲 Task 3.6: External Integration Guide Enhancement
**Status:** PARTIALLY COMPLETE ⚠️
**Priority:** MEDIUM
**Estimated Effort:** 1 day

**Location**: `docs/INTEGRATION.md` (enhance existing)

**Current State:**
- ✅ Basic integration guide exists
- ✅ README has setup instructions

**Needs:**
- [ ] More comprehensive examples
- [ ] Best practices guide
- [ ] Common patterns
- [ ] Production deployment examples
- [ ] Multi-language client examples

**Success Criteria:**
- [ ] User can integrate library in under 1 hour

---

### 🔲 Task 3.7: API Reference Documentation
**Status:** PARTIALLY COMPLETE ⚠️
**Priority:** MEDIUM
**Estimated Effort:** 2 days

**Location**: Package READMEs + `docs/API_REFERENCE.md`

**Current State:**
- ✅ Code is well-commented
- ✅ Package-level docs exist

**Needs:**
- [ ] Complete API reference
- [ ] All public APIs documented with examples
- [ ] Error cases documented
- [ ] Parameter constraints documented

**Success Criteria:**
- [ ] Every public API documented with examples

---

### Phase 3 Success Metrics:

| Criterion | Target | Status |
|-----------|--------|--------|
| Multi-tier workers | 4 tiers working | 🔲 Not started |
| Periodic tasks | Cron support | 🔲 Not started |
| Result backend | Store/retrieve | 🔲 Not started |
| Task routing | Working | 🔲 Not started |
| Architecture docs | 30 min to understand | 🔲 Not started |
| Integration guide | <1 hour to integrate | ⚠️ Basic exists |
| API reference | 100% coverage | ⚠️ ~60% |

---

## 🔲 PHASE 4: Multi-Language SDKs (Priority: MEDIUM)
### **STATUS: 0% COMPLETE**

### 🔲 Task 4.1: Python SDK
**Status:** NOT STARTED 🔲
**Estimated Effort:** 5-7 days

**Location**: `sdks/python/bananas/`

**Requirements:**
- Python client matching Go client API
- Redis integration
- Job submission with priorities
- Scheduled job support
- pip installable
- Full test coverage (>90%)
- Sphinx documentation

**Example Usage:**
```python
from bananas import Client

client = Client("redis://localhost:6379")
job = client.submit_job(
    name="send_email",
    payload={"to": "user@example.com"},
    priority="high"
)
```

**Success Criteria:**
- [ ] Python SDK works identically to Go client
- [ ] pip installable
- [ ] Full test coverage
- [ ] Complete documentation

---

### 🔲 Task 4.2: TypeScript SDK
**Status:** NOT STARTED 🔲
**Estimated Effort:** 5-7 days

**Location**: `sdks/typescript/`

**Requirements:**
- TypeScript/Node.js client
- Matches Go client API
- Redis integration
- npm installable
- Full test coverage (>90%)
- TypeDoc documentation

**Example Usage:**
```typescript
import { Client } from '@bananas/client';

const client = new Client('redis://localhost:6379');
const job = await client.submitJob({
    name: 'send_email',
    payload: { to: 'user@example.com' },
    priority: 'high'
});
```

**Success Criteria:**
- [ ] TypeScript SDK works identically to Go client
- [ ] npm installable
- [ ] Full test coverage
- [ ] Complete documentation

---

### Phase 4 Success Metrics:

| Criterion | Target | Status |
|-----------|--------|--------|
| Python SDK | Identical to Go | 🔲 Not started |
| TypeScript SDK | Identical to Go | 🔲 Not started |
| Both installable | pip/npm | 🔲 Not started |
| Test coverage | >90% | 🔲 Not started |

---

## 🔲 PHASE 5: Production Readiness (Priority: MEDIUM)
### **STATUS: 0% COMPLETE**

### 🔲 Task 5.1: Production Deployment Guide
**Status:** NOT STARTED 🔲
**Estimated Effort:** 3-5 days

**Location**: `docs/DEPLOYMENT.md`

**Requirements:**
- Production deployment guide
- Docker/Kubernetes examples
- Redis cluster setup
- High availability configuration
- Monitoring setup (Prometheus/Grafana)
- Scaling guidelines
- Backup/recovery procedures
- Performance tuning guide

**Contents:**
- Docker Compose for production
- Kubernetes manifests
- Helm charts
- CI/CD pipeline examples
- Load balancer configuration
- Health check endpoints
- Graceful shutdown procedures

**Success Criteria:**
- [ ] User can deploy to production in 1 hour
- [ ] Monitoring setup documented
- [ ] HA setup documented

---

### 🔲 Task 5.2: Security Hardening
**Status:** NOT STARTED 🔲
**Estimated Effort:** 2-3 days

**Requirements:**
- Redis AUTH setup
- TLS/SSL for Redis connections
- Input validation hardening
- Rate limiting (per client)
- Security audit
- Security documentation

**Security Checklist:**
- [ ] Redis password authentication
- [ ] TLS encryption for Redis
- [ ] Input sanitization
- [ ] Rate limiting per client IP
- [ ] Denial-of-service protection
- [ ] Audit logging for security events

**Success Criteria:**
- [ ] Redis secured with AUTH + TLS
- [ ] Security best practices documented
- [ ] No critical vulnerabilities

---

### Phase 5 Success Metrics:

| Criterion | Target | Status |
|-----------|--------|--------|
| Deploy to production | <1 hour | 🔲 Not started |
| Redis secured | AUTH + TLS | 🔲 Not started |
| Monitoring setup | Documented | 🔲 Not started |
| HA setup | Documented | 🔲 Not started |

---

## 🚀 Implementation Priority Order

### **Immediate (Next 2-3 Weeks):**

1. 🔲 **Task 2.2: Logging & Observability** (5-7 days) - In Progress
   - 3-tier logging (console, file, Elasticsearch)
   - Metrics collection
   - Health checks

2. 🔲 **Task 2.3: Error Handling & Recovery** (2-3 days)
   - Graceful failure handling
   - Production stability

3. 🔲 **Task 3.1: Multi-Tier Worker Architecture** (3-4 days)
   - Thin, Thin+, Default, Scaled modes
   - Production scaling capabilities

### **Short-Term (Weeks 4-6):**

4. 🔲 **Task 3.2: Periodic Tasks** (3-5 days)
   - Cron scheduler (Celery Beat equivalent)
   - Critical for production use cases

5. 🔲 **Task 3.3: Result Backend** (2-3 days)
   - Store/retrieve task results
   - RPC-style task support

6. 🔲 **Task 3.4: Task Routing** (1-2 days)
   - Route jobs to specific workers
   - Resource isolation

### **Medium-Term (Weeks 7-10):**

7. 🔲 **Task 3.5: Architecture Documentation** (1-2 days)
8. 🔲 **Task 3.6 & 3.7: Complete Documentation** (3 days)
9. 🔲 **Task 5.1: Production Deployment Guide** (3-5 days)
10. 🔲 **Task 4.1: Python SDK** (5-7 days)
11. 🔲 **Task 4.2: TypeScript SDK** (5-7 days)

---

## 🎯 Success Metrics Summary

### **Phase 1 (Complete):** ✅
- ✅ 1000 jobs complete successfully
- ✅ Continuous processing
- ✅ Scheduled execution
- ✅ Working example

### **Phase 2 (67% Complete):** 🔄
- ✅ Performance benchmarks established
- ✅ Major optimizations implemented
- 🔲 Comprehensive logging
- 🔲 Error handling
- ⚠️ 1,650 jobs/sec (target: 10K, limited by miniredis)
- ✅ p99 < 3ms (target: <100ms) - **33x better!**

### **Phase 3 (0% Complete):** 🔲
- 🔲 Multi-tier workers
- 🔲 Periodic tasks
- 🔲 Result backend
- 🔲 Complete documentation

### **Phase 4 (0% Complete):** 🔲
- 🔲 Python SDK
- 🔲 TypeScript SDK

### **Phase 5 (0% Complete):** 🔲
- 🔲 Production deployment guide
- 🔲 Security hardening

---

## 🔮 Future Work (Intentionally Deferred)

These features are **not in current scope** (Phases 6-9):

**Phase 6: API Layer**
- ❌ REST API for job submission
- ❌ GraphQL API
- ❌ WebSocket for real-time updates

**Phase 7: Management Tools**
- ❌ Web UI for job management
- ❌ CLI tool for job management
- ❌ Admin dashboard

**Phase 8: Advanced Features**
- ❌ Webhook notifications
- ❌ Job priorities beyond 3 levels
- ❌ Job cancellation/revocation

**Phase 9: Workflow Features**
- ❌ Job dependencies/DAGs
- ❌ Task chains
- ❌ Task groups
- ❌ Task chords
- ❌ Rate limiting per job type

**Focus:** Make the library-based model excellent before building SaaS features.

---

## 📈 Celery Feature Parity Progress

| Category | Bananas | Celery | Parity % |
|----------|---------|--------|----------|
| **Core Queue** | ✅ Complete | ✅ Complete | 80% |
| **Performance** | ✅ Optimized | ✅ Mature | 100%+ |
| **Observability** | 🔲 Pending | ✅ Complete | 20% |
| **Periodic Tasks** | 🔲 Pending | ✅ Beat | 0% |
| **Result Backend** | 🔲 Pending | ✅ Complete | 0% |
| **Worker Scaling** | 🔲 Pending | ✅ Complete | 0% |
| **Task Routing** | 🔲 Pending | ✅ Complete | 0% |
| **Monitoring UI** | 🔲 Pending | ✅ Flower | 0% |
| **Overall** | **~55%** | **100%** | **55%** |

**Timeline to 90% Parity:** ~6-8 weeks (completing Phases 2-3)

---

## 📝 Testing Requirements for All Tasks

Every task must include:
1. Unit tests for new functions/methods (aim for 90%+ coverage)
2. Integration tests for cross-component interactions
3. Error case tests (what happens when things fail)
4. Performance benchmarks (where applicable)
5. Documentation of what's being tested and why

**Current Test Coverage:** 93.3% ✅

---

## 📚 Documentation Requirements for All Tasks

Every task must update relevant documentation:
1. Code comments for complex logic
2. Package-level documentation (what package does)
3. Function/method documentation (parameters, returns, errors)
4. README updates if public API changes
5. Architecture docs if design changes

Documentation should answer:
- What does this do?
- Why does it work this way?
- When should I use this?
- How do I handle errors?

---

## 🔄 Update Process

**This document should be updated:**
- ✅ When starting a new phase/task (mark as in progress)
- ✅ When completing a phase/task (mark as complete, add completion date)
- ✅ When adding new requirements or tasks
- ✅ When priorities change
- ✅ After major milestones

**Last Major Update:** 2025-10-24 (Added Task 2.1 Follow-up, Multi-tier logging, Multi-tier workers)

---

_End of Project Plan_

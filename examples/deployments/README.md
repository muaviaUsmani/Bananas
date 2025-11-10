# Bananas Worker Deployment Examples

This directory contains production-ready deployment configurations for the Bananas distributed task queue system in various operational modes.

## Available Deployment Configurations

### Docker Compose Examples

| File | Mode | Use Case | Traffic | Workers |
|------|------|----------|---------|---------|
| `docker-compose-thin.yml` | Thin | Development/Testing | <100 jobs/hour | 5 |
| `docker-compose-default.yml` | Default | Standard Production | 1K-10K jobs/hour | 60 (3×20) |
| `docker-compose-specialized.yml` | Specialized | High Traffic, Priority Isolation | 10K+ jobs/hour | 220 (150+60+10) |
| `docker-compose-job-specialized.yml` | Job-Specialized | Diverse Resource Needs | 10K+ jobs/hour | 215 (varied) |

### Kubernetes Examples

| File | Description |
|------|-------------|
| `k8s-deployment.yml` | Complete Kubernetes deployment with all worker modes |
| `k8s-hpa.yml` | Horizontal Pod Autoscaling configuration |

---

## Quick Start

### Development Setup (Thin Mode)

```bash
# Start development environment
docker-compose -f docker-compose-thin.yml up

# Access API
curl http://localhost:8080/health

# View logs
docker-compose -f docker-compose-thin.yml logs -f worker
```

### Production Setup (Default Mode)

```bash
# Start production environment
docker-compose -f docker-compose-default.yml up -d

# Scale workers dynamically
docker-compose -f docker-compose-default.yml up --scale worker=5

# View metrics
docker-compose -f docker-compose-default.yml logs worker | grep "System metrics"
```

### High Traffic Setup (Specialized Mode)

```bash
# Start with priority-isolated workers
docker-compose -f docker-compose-specialized.yml up -d

# Scale high-priority workers independently
docker-compose -f docker-compose-specialized.yml up --scale worker-high=5

# Monitor specific priority queue
docker-compose -f docker-compose-specialized.yml logs -f worker-high
```

### Job-Type Specialized Setup

```bash
# Start with job-type isolation
docker-compose -f docker-compose-job-specialized.yml up -d

# Scale I/O workers (high concurrency for API calls, file operations)
docker-compose -f docker-compose-job-specialized.yml up --scale worker-io=5

# Scale CPU workers (lower concurrency for compute-intensive jobs)
docker-compose -f docker-compose-job-specialized.yml up --scale worker-cpu=3
```

---

## Worker Modes Explained

### 1. Thin Mode

**Configuration:**
```yaml
environment:
  - WORKER_MODE=thin
  - WORKER_CONCURRENCY=5
  - WORKER_PRIORITIES=high,normal,low
  - ENABLE_SCHEDULER=true
```

**Characteristics:**
- Single worker process handles all job types and priorities
- Built-in scheduler (no separate scheduler process needed)
- Minimal resource footprint
- Suitable for development, testing, very low traffic

**When to Use:**
- Local development
- CI/CD testing
- Proof-of-concept deployments
- Traffic < 100 jobs/hour

---

### 2. Default Mode

**Configuration:**
```yaml
environment:
  - WORKER_MODE=default          # Or omit (default)
  - WORKER_CONCURRENCY=20
  - WORKER_PRIORITIES=high,normal,low
  - ENABLE_SCHEDULER=false       # Use dedicated scheduler
```

**Characteristics:**
- Priority-aware job processing
- Horizontally scalable (multiple instances)
- Separate scheduler process recommended
- Configurable concurrency per instance

**When to Use:**
- Standard production deployments
- Traffic: 1K-10K jobs/hour
- General-purpose workload
- Need horizontal scaling

**Scaling:**
```bash
# Add more worker instances
docker-compose up --scale worker=5

# Increase concurrency per instance
WORKER_CONCURRENCY=50 docker-compose up
```

---

### 3. Specialized Mode (Priority Isolation)

**Configuration:**
```yaml
# High priority workers
worker-high:
  environment:
    - WORKER_MODE=specialized
    - WORKER_PRIORITIES=high
    - WORKER_CONCURRENCY=50

# Normal priority workers
worker-normal:
  environment:
    - WORKER_MODE=specialized
    - WORKER_PRIORITIES=normal
    - WORKER_CONCURRENCY=30

# Low priority workers
worker-low:
  environment:
    - WORKER_MODE=specialized
    - WORKER_PRIORITIES=low
    - WORKER_CONCURRENCY=10
```

**Characteristics:**
- Separate worker pools per priority queue
- Dedicated resources per priority level
- Prevents low-priority jobs from blocking high-priority
- Fine-grained scaling per priority

**When to Use:**
- High traffic (10K+ jobs/hour)
- SLA requirements for high-priority jobs
- Need guaranteed resources per priority
- Different scaling needs per priority

**Resource Allocation:**
```
High:   150 workers (3 instances × 50)  - 4 CPU, 4GB RAM
Normal:  60 workers (2 instances × 30)  - 2 CPU, 2GB RAM
Low:     10 workers (1 instance  × 10)  - 1 CPU, 1GB RAM
```

---

### 4. Job-Specialized Mode (Job Type Isolation)

**Configuration:**
```yaml
# CPU-intensive jobs
worker-cpu:
  environment:
    - WORKER_MODE=job-specialized
    - WORKER_JOB_TYPES=send_email,generate_report,process_analytics
    - WORKER_CONCURRENCY=10

# I/O-intensive jobs
worker-io:
  environment:
    - WORKER_MODE=job-specialized
    - WORKER_JOB_TYPES=fetch_data,process_images,upload_files
    - WORKER_CONCURRENCY=50
```

**Characteristics:**
- Workers handle only specific job types
- Optimize concurrency per job type
- Deploy on appropriate hardware (CPU vs I/O optimized)
- Prevents resource contention between different workload types

**When to Use:**
- Job types have vastly different resource requirements
- Need to optimize infrastructure costs
- Jobs require specialized hardware (GPUs, high-memory, etc.)
- Want to prevent resource contention

**Example Job Categories:**
- **CPU-intensive:** ML inference, video encoding, PDF generation, data analytics
- **I/O-intensive:** File uploads/downloads, API calls, web scraping, image processing
- **Database:** Migrations, backups, cleanup, analytics queries
- **General:** Notifications, webhooks, logging, simple transformations

---

### 5. Scheduler-Only Mode

**Configuration:**
```yaml
scheduler:
  environment:
    - WORKER_MODE=scheduler-only
    - SCHEDULER_INTERVAL=1s
  deploy:
    replicas: 1  # MUST be exactly 1
```

**Characteristics:**
- No job execution (zero worker goroutines)
- Only runs scheduled job processing
- Lightweight process
- Must be single instance (or use leader election)

**When to Use:**
- Using specialized or job-specialized workers (which have ENABLE_SCHEDULER=false)
- High-availability scheduler deployment (with leader election)
- Want dedicated scheduler process

---

## Architecture Patterns

### Pattern 1: Simple Production (Default Mode)

```
┌─────────┐
│  Redis  │
└────┬────┘
     │
     ├─────┬─────┬─────┐
     │     │     │     │
  ┌──▼─┐ ┌─▼──┐ ┌─▼──┐ ┌─▼──────────┐
  │W×20│ │W×20│ │W×20│ │Scheduler×1 │
  └────┘ └────┘ └────┘ └────────────┘
```

**Components:**
- 3 worker instances (20 workers each)
- 1 dedicated scheduler
- All workers process all priorities and job types

---

### Pattern 2: Priority-Isolated (Specialized Mode)

```
┌─────────┐
│  Redis  │
└────┬────┘
     │
     ├─────────┬──────────┬─────────┐
     │         │          │         │
  ┌──▼────┐ ┌─▼──────┐ ┌─▼──────┐ ┌─▼──────────┐
  │High×50│ │Normal×30│ │Low×10  │ │Scheduler×1 │
  │×3 inst│ │×2 inst  │ │×1 inst │ └────────────┘
  └───────┘ └─────────┘ └────────┘
```

**Components:**
- High priority: 3 instances × 50 workers
- Normal priority: 2 instances × 30 workers
- Low priority: 1 instance × 10 workers
- 1 dedicated scheduler

---

### Pattern 3: Job-Type Isolated (Job-Specialized Mode)

```
┌─────────┐
│  Redis  │
└────┬────┘
     │
     ├──────────┬───────────┬──────────┬──────────┐
     │          │           │          │          │
  ┌──▼──────┐ ┌─▼───────┐ ┌─▼──────┐ ┌─▼──────┐ ┌─▼──────────┐
  │CPU×10   │ │I/O×50   │ │DB×5    │ │Gen×20  │ │Scheduler×1 │
  │×2 inst  │ │×3 inst  │ │×1 inst │ │×2 inst │ └────────────┘
  └─────────┘ └─────────┘ └────────┘ └────────┘
  (CPU-opt)   (I/O-opt)   (DB-access) (General)
```

**Components:**
- CPU workers: 2 instances × 10 workers (CPU-optimized nodes)
- I/O workers: 3 instances × 50 workers (I/O-optimized nodes)
- Database workers: 1 instance × 5 workers (database network access)
- General workers: 2 instances × 20 workers (standard nodes)
- 1 dedicated scheduler

---

## Scaling Strategies

### Horizontal Scaling (More Instances)

**When:**
- Worker utilization consistently > 80%
- Queue depth growing
- Job processing latency increasing

**How:**
```bash
# Docker Compose
docker-compose up --scale worker=5

# Kubernetes
kubectl scale deployment worker --replicas=5
```

### Vertical Scaling (More Concurrency)

**When:**
- CPU/memory headroom available on nodes
- Jobs are I/O bound (waiting on network/disk)
- Worker utilization < 50%

**How:**
```bash
# Update environment variable
WORKER_CONCURRENCY=50  # was 20

# Or in docker-compose.yml
environment:
  - WORKER_CONCURRENCY=50
```

### Mode Migration

| From | To | When | Benefits |
|------|----|----|----------|
| Thin | Default | Traffic increases beyond 100 jobs/hour | Better performance, horizontal scaling |
| Default | Specialized | Need priority isolation, high-priority SLAs | Guaranteed resources per priority |
| Default | Job-Specialized | Jobs have different resource needs | Optimize cost, prevent resource contention |
| Specialized | Job-Specialized | Both priority AND job-type isolation needed | Maximum flexibility and optimization |

---

## Monitoring and Observability

### Key Metrics to Track

```bash
# Worker utilization
worker_utilization > 80% → Scale horizontally

# Queue depth
queue_depth > 1000 for 5min → Add more workers

# Job processing time
avg_job_duration increasing → Investigate performance

# Error rate
error_rate > 5% → Check job handlers
```

### Accessing Logs

```bash
# All workers
docker-compose logs -f worker

# Specific worker type
docker-compose logs -f worker-high
docker-compose logs -f worker-io

# Follow metrics logs
docker-compose logs worker | grep "System metrics"

# Scheduler logs
docker-compose logs -f scheduler
```

### Metrics Output Example

```json
{
  "level": "info",
  "msg": "System metrics",
  "jobs_processed": 12450,
  "jobs_completed": 12389,
  "jobs_failed": 61,
  "avg_duration_ms": 247,
  "worker_utilization": "78.3%",
  "error_rate": "0.49%",
  "uptime": "2h15m30s"
}
```

---

## Environment Variables Reference

### Worker Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `WORKER_MODE` | `default` | Worker mode: `thin`, `default`, `specialized`, `job-specialized`, `scheduler-only` |
| `WORKER_CONCURRENCY` | `10` | Number of concurrent worker goroutines |
| `WORKER_PRIORITIES` | `high,normal,low` | Comma-separated priorities to process |
| `WORKER_JOB_TYPES` | (all) | Comma-separated job types to handle (job-specialized mode only) |
| `ENABLE_SCHEDULER` | `true` | Whether to run scheduler loop (set to `false` if using dedicated scheduler) |

### Scheduler Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `SCHEDULER_INTERVAL` | `1s` | How often to check for scheduled jobs (100ms - 1m) |

### Job Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `JOB_TIMEOUT` | `5m` | Maximum time for job execution |
| `MAX_RETRIES` | `3` | Maximum retry attempts for failed jobs |

### Redis Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_URL` | `redis://localhost:6379` | Redis connection URL |

### Logging Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | Log format: `json`, `text` |
| `LOG_CONSOLE_ENABLED` | `true` | Enable console logging |
| `LOG_FILE_ENABLED` | `false` | Enable file logging |
| `LOG_FILE_PATH` | `/var/log/bananas/worker.log` | Log file path |
| `LOG_FILE_MAX_SIZE_MB` | `100` | Max log file size in MB before rotation |
| `LOG_FILE_MAX_BACKUPS` | `5` | Number of rotated log files to keep |

---

## Production Checklist

### Before Deploying

- [ ] Choose appropriate worker mode for your traffic pattern
- [ ] Calculate required worker capacity (throughput × avg_duration)
- [ ] Configure resource limits (CPU, memory) appropriately
- [ ] Set up monitoring and alerting
- [ ] Configure log aggregation
- [ ] Test graceful shutdown behavior
- [ ] Verify Redis connection pooling and timeouts
- [ ] Set appropriate `JOB_TIMEOUT` values
- [ ] Configure `MAX_RETRIES` based on job idempotency

### After Deploying

- [ ] Monitor worker utilization (should be 60-80%)
- [ ] Monitor queue depth (should not grow unbounded)
- [ ] Monitor error rates (should be <1%)
- [ ] Set up alerts for high queue depth
- [ ] Set up alerts for high error rates
- [ ] Test horizontal scaling
- [ ] Verify scheduler is running (only 1 instance)
- [ ] Test graceful shutdown during deployment

---

## Troubleshooting

### Workers not processing jobs

```bash
# Check worker logs
docker-compose logs worker

# Verify Redis connection
docker-compose exec worker redis-cli -h redis ping

# Check queue depth
docker-compose exec redis redis-cli LLEN bananas:queue:high
```

### High queue depth

```bash
# Scale workers
docker-compose up --scale worker=10

# Or increase concurrency
# Update WORKER_CONCURRENCY in docker-compose.yml
```

### Jobs timing out

```bash
# Increase job timeout
# Update JOB_TIMEOUT in docker-compose.yml
# e.g., JOB_TIMEOUT=10m

# Or optimize job handlers
```

### Multiple schedulers running

```bash
# Ensure scheduler has exactly 1 replica
docker-compose ps scheduler

# Should show only 1 instance
# If multiple, set: deploy.replicas: 1
```

### Workers skipping jobs

```bash
# Check job type filters (job-specialized mode)
docker-compose logs worker | grep "Skipping job"

# Ensure WORKER_JOB_TYPES includes the job name
# Or use default mode workers as catch-all
```

---

## Further Reading

- [Worker Architecture Design](../../docs/WORKER_ARCHITECTURE_DESIGN.md) - Detailed design documentation
- [Configuration Reference](../../docs/CONFIGURATION.md) - All configuration options
- [Monitoring Guide](../../docs/MONITORING.md) - Setting up metrics and alerting
- [Performance Tuning](../../docs/PERFORMANCE.md) - Optimization strategies

---

**Last Updated:** 2025-11-10

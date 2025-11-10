# Multi-Tier Worker Architecture - User Guide

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Worker Modes](#worker-modes)
- [Configuration Guide](#configuration-guide)
- [Choosing the Right Mode](#choosing-the-right-mode)
- [Deployment Patterns](#deployment-patterns)
- [Migration Guide](#migration-guide)
- [Best Practices](#best-practices)
- [Monitoring and Metrics](#monitoring-and-metrics)
- [Troubleshooting](#troubleshooting)
- [FAQ](#faq)

---

## Overview

The Bananas task queue supports flexible worker deployment configurations, allowing you to optimize resource allocation and performance based on your workload characteristics.

### Key Benefits

- **Resource Optimization**: Allocate resources based on job priority and type
- **Cost Efficiency**: Right-size worker pools for different workload patterns
- **Guaranteed SLAs**: Ensure high-priority jobs get dedicated resources
- **Horizontal Scaling**: Scale different worker pools independently
- **Workload Isolation**: Prevent resource contention between different job types

### Supported Modes

| Mode | Best For | Workers | Complexity |
|------|----------|---------|------------|
| **Thin** | Development, testing | 1-10 | ⭐ Low |
| **Default** | Standard production | 10-50 per instance | ⭐⭐ Medium |
| **Specialized** | High traffic, priority SLAs | Dedicated per priority | ⭐⭐⭐ High |
| **Job-Specialized** | Diverse resource needs | Dedicated per job type | ⭐⭐⭐ High |
| **Scheduler-Only** | Dedicated scheduler | 0 (scheduler only) | ⭐ Low |

---

## Quick Start

### Development (Thin Mode)

Perfect for local development and testing:

```bash
# Set environment variables
export WORKER_MODE=thin
export WORKER_CONCURRENCY=5

# Start worker
./bananas-worker
```

### Production (Default Mode)

For standard production deployments:

```bash
# Worker instances
export WORKER_MODE=default
export WORKER_CONCURRENCY=20
export ENABLE_SCHEDULER=false  # Use dedicated scheduler

# Dedicated scheduler
export WORKER_MODE=scheduler-only
export SCHEDULER_INTERVAL=1s
```

### High Traffic (Specialized Mode)

For priority-isolated workers:

```bash
# High priority workers
export WORKER_MODE=specialized
export WORKER_PRIORITIES=high
export WORKER_CONCURRENCY=50

# Normal priority workers
export WORKER_MODE=specialized
export WORKER_PRIORITIES=normal
export WORKER_CONCURRENCY=30

# Low priority workers
export WORKER_MODE=specialized
export WORKER_PRIORITIES=low
export WORKER_CONCURRENCY=10
```

---

## Worker Modes

### 1. Thin Mode

**Use Case**: Development, testing, very low traffic

```bash
WORKER_MODE=thin
WORKER_CONCURRENCY=5
```

**Architecture**:
```
┌─────────────────────────────┐
│   Single Worker Process     │
│  ┌─────────────────────┐    │
│  │ 5 Worker Goroutines │    │
│  │ All Priorities      │    │
│  │ All Job Types       │    │
│  │ Built-in Scheduler  │    │
│  └─────────────────────┘    │
└─────────────────────────────┘
```

**Characteristics**:
- ✅ Simplest deployment (single process)
- ✅ Minimal resource usage
- ✅ Built-in scheduler (no separate process)
- ❌ Limited throughput (<100 jobs/hour)
- ❌ No horizontal scaling

**When to Use**:
- Local development and testing
- CI/CD pipelines
- Proof-of-concept deployments
- Very low traffic environments

**Example**:
```yaml
# docker-compose.yml
services:
  worker:
    image: bananas-worker:latest
    environment:
      - WORKER_MODE=thin
      - WORKER_CONCURRENCY=5
      - REDIS_URL=redis://redis:6379
```

---

### 2. Default Mode

**Use Case**: Standard production deployments

```bash
WORKER_MODE=default          # or omit (default)
WORKER_CONCURRENCY=20
WORKER_PRIORITIES=high,normal,low
ENABLE_SCHEDULER=false       # use dedicated scheduler
```

**Architecture**:
```
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│  Worker #1   │  │  Worker #2   │  │  Worker #3   │
│  20 Workers  │  │  20 Workers  │  │  20 Workers  │
│  All Queues  │  │  All Queues  │  │  All Queues  │
└──────────────┘  └──────────────┘  └──────────────┘
       │                 │                 │
       └─────────────────┴─────────────────┘
                         │
                  ┌──────▼───────┐
                  │  Scheduler   │
                  │  (Separate)  │
                  └──────────────┘
```

**Characteristics**:
- ✅ Priority-aware processing (high → normal → low)
- ✅ Horizontally scalable (add more instances)
- ✅ Configurable concurrency (10-50 workers)
- ✅ Handles all job types
- ✅ Production-ready
- ⚠️  Low-priority jobs may be delayed if high traffic

**When to Use**:
- Standard production deployments
- Traffic: 1K-10K jobs/hour
- General-purpose workloads
- Need horizontal scaling
- Don't need priority isolation

**Scaling**:
```bash
# Horizontal scaling (add more instances)
docker-compose up --scale worker=5

# Vertical scaling (more workers per instance)
WORKER_CONCURRENCY=50
```

**Example**:
```yaml
# docker-compose.yml
services:
  worker:
    environment:
      - WORKER_MODE=default
      - WORKER_CONCURRENCY=20
      - ENABLE_SCHEDULER=false
    deploy:
      replicas: 3  # 3 instances × 20 workers = 60 total

  scheduler:
    environment:
      - WORKER_MODE=scheduler-only
    deploy:
      replicas: 1  # exactly 1
```

---

### 3. Specialized Mode (Priority Isolation)

**Use Case**: High traffic, SLA requirements for priority levels

```bash
# High priority worker pool
WORKER_MODE=specialized
WORKER_PRIORITIES=high
WORKER_CONCURRENCY=50

# Normal priority worker pool
WORKER_MODE=specialized
WORKER_PRIORITIES=normal
WORKER_CONCURRENCY=30

# Low priority worker pool
WORKER_MODE=specialized
WORKER_PRIORITIES=low
WORKER_CONCURRENCY=10
```

**Architecture**:
```
┌─────────────────┐
│ High Priority   │  ← 150 workers dedicated to high-priority jobs
│ 3 × 50 workers  │
└─────────────────┘

┌─────────────────┐
│ Normal Priority │  ← 60 workers dedicated to normal-priority jobs
│ 2 × 30 workers  │
└─────────────────┘

┌─────────────────┐
│ Low Priority    │  ← 10 workers dedicated to low-priority jobs
│ 1 × 10 workers  │
└─────────────────┘
```

**Characteristics**:
- ✅ **Priority Isolation**: High-priority jobs never wait for low-priority
- ✅ **Guaranteed Resources**: Each priority has dedicated workers
- ✅ **Independent Scaling**: Scale each priority level separately
- ✅ **SLA Compliance**: Meet strict SLAs for critical jobs
- ✅ **Fine-Grained Control**: Different concurrency per priority
- ⚠️  More complex deployment (3 separate worker pools)
- ⚠️  Higher baseline resource usage

**When to Use**:
- High traffic (10K+ jobs/hour)
- Strict SLAs for high-priority jobs
- Need guaranteed resources per priority
- Low-priority jobs causing delays for high-priority
- Different scaling needs per priority level

**Example**:
```yaml
# docker-compose.yml
services:
  worker-high:
    environment:
      - WORKER_MODE=specialized
      - WORKER_PRIORITIES=high
      - WORKER_CONCURRENCY=50
    deploy:
      replicas: 3
    resources:
      limits:
        cpus: '4'
        memory: 4G

  worker-normal:
    environment:
      - WORKER_MODE=specialized
      - WORKER_PRIORITIES=normal
      - WORKER_CONCURRENCY=30
    deploy:
      replicas: 2

  worker-low:
    environment:
      - WORKER_MODE=specialized
      - WORKER_PRIORITIES=low
      - WORKER_CONCURRENCY=10
    deploy:
      replicas: 1
```

**Scaling Example**:
```bash
# Scale high-priority workers independently
docker-compose up --scale worker-high=5

# Reduce low-priority workers during peak hours
docker-compose up --scale worker-low=0
```

---

### 4. Job-Specialized Mode

**Use Case**: Jobs with vastly different resource requirements

```bash
# CPU-intensive workers
WORKER_MODE=job-specialized
WORKER_JOB_TYPES=send_email,generate_report,process_analytics
WORKER_CONCURRENCY=10

# I/O-intensive workers
WORKER_MODE=job-specialized
WORKER_JOB_TYPES=fetch_data,process_images,upload_files
WORKER_CONCURRENCY=50
```

**Architecture**:
```
┌─────────────────┐
│ CPU Workers     │  ← 10 workers, high CPU, compute-intensive jobs
│ Job Types:      │
│  • Reports      │
│  • Analytics    │
└─────────────────┘
      Deploy on CPU-optimized nodes

┌─────────────────┐
│ I/O Workers     │  ← 50 workers, high concurrency, I/O jobs
│ Job Types:      │
│  • API calls    │
│  • File uploads │
└─────────────────┘
      Deploy on I/O-optimized nodes

┌─────────────────┐
│ DB Workers      │  ← 5 workers, low concurrency, DB jobs
│ Job Types:      │
│  • Migrations   │
│  • Backups      │
└─────────────────┘
      Deploy near database
```

**Characteristics**:
- ✅ **Workload Isolation**: Prevent resource contention
- ✅ **Optimized Concurrency**: Right-size per job type
- ✅ **Infrastructure Optimization**: Deploy on appropriate hardware
- ✅ **Cost Efficiency**: Pay only for resources you need
- ✅ **Independent Scaling**: Scale each job type separately
- ⚠️  Requires understanding of job resource requirements
- ⚠️  Most complex deployment pattern

**When to Use**:
- Jobs have vastly different resource needs
- Want to optimize infrastructure costs
- Jobs require specialized hardware (GPUs, high-memory, etc.)
- Prevent resource contention between workloads
- Deploy different job types in different regions/zones

**Job Type Categories**:

| Category | Example Jobs | Concurrency | Resources |
|----------|-------------|-------------|-----------|
| CPU-Intensive | PDF generation, video encoding, ML inference | Low (5-10) | High CPU |
| I/O-Intensive | API calls, file uploads, web scraping | High (50-100) | High memory |
| Database | Migrations, backups, analytics queries | Very Low (1-5) | DB access |
| General | Notifications, webhooks, logging | Medium (10-30) | Balanced |

**Example**:
```yaml
# docker-compose.yml
services:
  worker-cpu:
    environment:
      - WORKER_MODE=job-specialized
      - WORKER_JOB_TYPES=generate_report,process_analytics
      - WORKER_CONCURRENCY=10
    deploy:
      resources:
        limits:
          cpus: '4'
          memory: 2G

  worker-io:
    environment:
      - WORKER_MODE=job-specialized
      - WORKER_JOB_TYPES=fetch_data,upload_files
      - WORKER_CONCURRENCY=50
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 4G

  # Catch-all worker for unspecified job types
  worker-general:
    environment:
      - WORKER_MODE=default  # processes all job types
      - WORKER_CONCURRENCY=20
```

---

### 5. Scheduler-Only Mode

**Use Case**: Dedicated scheduler for scheduled/recurring jobs

```bash
WORKER_MODE=scheduler-only
SCHEDULER_INTERVAL=1s
```

**Architecture**:
```
┌──────────────────┐
│   Scheduler      │
│                  │
│  • 0 Workers     │
│  • Every 1s:     │
│    Move scheduled│
│    → ready queue │
└──────────────────┘
```

**Characteristics**:
- ✅ Dedicated process for scheduled jobs
- ✅ Minimal resource usage
- ✅ Decoupled from job execution
- ⚠️  **Must be exactly 1 instance** (or use leader election)

**When to Use**:
- Using specialized or job-specialized workers (with `ENABLE_SCHEDULER=false`)
- Need high availability for scheduler
- Want isolated scheduler monitoring

**Example**:
```yaml
# docker-compose.yml
services:
  scheduler:
    environment:
      - WORKER_MODE=scheduler-only
      - SCHEDULER_INTERVAL=1s
    deploy:
      replicas: 1  # MUST be exactly 1
    resources:
      limits:
        cpus: '0.5'
        memory: 512M
```

---

## Configuration Guide

### Environment Variables

#### Worker Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `WORKER_MODE` | string | `default` | Worker mode: `thin`, `default`, `specialized`, `job-specialized`, `scheduler-only` |
| `WORKER_CONCURRENCY` | int | `10` | Number of concurrent worker goroutines (1-1000) |
| `WORKER_PRIORITIES` | string | `high,normal,low` | Comma-separated priorities to process |
| `WORKER_JOB_TYPES` | string | (all) | Comma-separated job types (job-specialized mode only) |
| `ENABLE_SCHEDULER` | bool | `true` | Run scheduler loop (set to `false` if using dedicated scheduler) |

#### Scheduler Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `SCHEDULER_INTERVAL` | duration | `1s` | Check interval for scheduled jobs (100ms - 1m) |

#### Job Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `JOB_TIMEOUT` | duration | `5m` | Maximum job execution time |
| `MAX_RETRIES` | int | `3` | Maximum retry attempts |

#### Redis Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `REDIS_URL` | string | `redis://localhost:6379` | Redis connection URL |

### Configuration Examples

#### Thin Mode (Development)
```bash
export WORKER_MODE=thin
export WORKER_CONCURRENCY=5
export REDIS_URL=redis://localhost:6379
```

#### Default Mode (Production)
```bash
# Worker
export WORKER_MODE=default
export WORKER_CONCURRENCY=20
export ENABLE_SCHEDULER=false

# Scheduler (separate process)
export WORKER_MODE=scheduler-only
```

#### Specialized Mode (Priority Isolation)
```bash
# High priority
export WORKER_MODE=specialized
export WORKER_PRIORITIES=high
export WORKER_CONCURRENCY=50

# Normal priority
export WORKER_MODE=specialized
export WORKER_PRIORITIES=normal
export WORKER_CONCURRENCY=30

# Low priority
export WORKER_MODE=specialized
export WORKER_PRIORITIES=low
export WORKER_CONCURRENCY=10
```

#### Job-Specialized Mode
```bash
# CPU workers
export WORKER_MODE=job-specialized
export WORKER_JOB_TYPES=generate_report,process_analytics
export WORKER_CONCURRENCY=10

# I/O workers
export WORKER_MODE=job-specialized
export WORKER_JOB_TYPES=fetch_data,upload_files
export WORKER_CONCURRENCY=50
```

---

## Choosing the Right Mode

### Decision Tree

```
Start Here
    │
    ├─ Development or Testing?
    │   └─> Use THIN MODE
    │       • Single process
    │       • 5 workers
    │       • All priorities and job types
    │
    ├─ Traffic < 1K jobs/hour?
    │   └─> Use DEFAULT MODE
    │       • 1-2 instances
    │       • 10-20 workers per instance
    │       • Horizontal scaling if needed
    │
    ├─ Traffic 1K-10K jobs/hour?
    │   └─> Use DEFAULT MODE
    │       • 3-5 instances
    │       • 20-30 workers per instance
    │       • Scale horizontally as needed
    │
    ├─ Traffic > 10K jobs/hour?
    │   │
    │   ├─ Have strict SLAs for high-priority jobs?
    │   │   └─> Use SPECIALIZED MODE (Priority Isolation)
    │   │       • Dedicated workers per priority
    │   │       • High priority: 3-5 instances × 50 workers
    │   │       • Normal priority: 2-3 instances × 30 workers
    │   │       • Low priority: 1-2 instances × 10 workers
    │   │
    │   ├─ Jobs have different resource requirements?
    │   │   └─> Use JOB-SPECIALIZED MODE
    │   │       • CPU-intensive: Low concurrency, high CPU
    │   │       • I/O-intensive: High concurrency, high memory
    │   │       • Database: Very low concurrency
    │   │       • General: Medium concurrency
    │   │
    │   └─ General high traffic?
    │       └─> Use DEFAULT MODE
    │           • Many instances (5-10+)
    │           • Load balancer
    │           • Autoscaling
    │
    └─ Need dedicated scheduler?
        └─> Add SCHEDULER-ONLY MODE
            • Always exactly 1 instance
            • Used with specialized/job-specialized workers
```

### Comparison Matrix

| Criteria | Thin | Default | Specialized | Job-Specialized |
|----------|------|---------|-------------|-----------------|
| **Traffic** | <100/hr | 1K-10K/hr | 10K+/hr | 10K+/hr |
| **Deployment Complexity** | ⭐ | ⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐ |
| **Resource Efficiency** | ⭐⭐⭐ | ⭐⭐ | ⭐ | ⭐⭐⭐ |
| **Priority Isolation** | ❌ | ❌ | ✅ | ❌ |
| **Job Type Isolation** | ❌ | ❌ | ❌ | ✅ |
| **Horizontal Scaling** | ❌ | ✅ | ✅ | ✅ |
| **SLA Guarantees** | ❌ | ⚠️ | ✅ | ⚠️ |
| **Cost Optimization** | ✅ | ⭐⭐ | ⭐ | ✅ |

---

## Deployment Patterns

### Pattern 1: Simple Production (Default Mode)

**Best for**: Standard production, general workloads

```
Architecture:
┌─────┐
│Redis│
└──┬──┘
   │
   ├───┬───┬───┬──────────┐
   │   │   │   │          │
 W×20 W×20 W×20 Scheduler API
```

**Setup**:
```yaml
services:
  redis:
    image: redis:7-alpine

  worker:
    environment:
      - WORKER_MODE=default
      - WORKER_CONCURRENCY=20
      - ENABLE_SCHEDULER=false
    deploy:
      replicas: 3

  scheduler:
    environment:
      - WORKER_MODE=scheduler-only
    deploy:
      replicas: 1

  api:
    deploy:
      replicas: 2
```

**Capacity**: 60 workers, ~5K jobs/hour

---

### Pattern 2: Priority-Isolated (Specialized Mode)

**Best for**: High traffic, strict SLAs, priority guarantees

```
Architecture:
┌─────┐
│Redis│
└──┬──┘
   │
   ├──────┬───────┬────────┬──────────┐
   │      │       │        │          │
High×50 Norm×30 Low×10 Scheduler  API
 ×3      ×2      ×1       ×1        ×2
```

**Setup**:
```yaml
services:
  worker-high:
    environment:
      - WORKER_MODE=specialized
      - WORKER_PRIORITIES=high
      - WORKER_CONCURRENCY=50
    deploy:
      replicas: 3
    resources:
      limits:
        cpus: '4'
        memory: 4G

  worker-normal:
    environment:
      - WORKER_MODE=specialized
      - WORKER_PRIORITIES=normal
      - WORKER_CONCURRENCY=30
    deploy:
      replicas: 2

  worker-low:
    environment:
      - WORKER_MODE=specialized
      - WORKER_PRIORITIES=low
      - WORKER_CONCURRENCY=10
    deploy:
      replicas: 1
```

**Capacity**: 220 workers (150 high + 60 normal + 10 low), ~20K jobs/hour

---

### Pattern 3: Job-Type Isolated (Job-Specialized Mode)

**Best for**: Diverse resource requirements, cost optimization

```
Architecture:
┌─────┐
│Redis│
└──┬──┘
   │
   ├──────┬──────┬──────┬────────┬──────────┐
   │      │      │      │        │          │
CPU×10  IO×50  DB×5  Gen×20  Scheduler  API
 ×2      ×3     ×1     ×2       ×1        ×2
(High   (High  (DB    (Std)
 CPU)    Mem)   Net)
```

**Setup**:
```yaml
services:
  worker-cpu:
    environment:
      - WORKER_MODE=job-specialized
      - WORKER_JOB_TYPES=generate_report,process_analytics
      - WORKER_CONCURRENCY=10
    deploy:
      replicas: 2
      placement:
        constraints:
          - node.labels.workload == cpu-optimized

  worker-io:
    environment:
      - WORKER_MODE=job-specialized
      - WORKER_JOB_TYPES=fetch_data,upload_files
      - WORKER_CONCURRENCY=50
    deploy:
      replicas: 3

  worker-general:
    environment:
      - WORKER_MODE=default
      - WORKER_CONCURRENCY=20
    deploy:
      replicas: 2
```

**Capacity**: 215 workers (20 CPU + 150 I/O + 5 DB + 40 general), ~20K jobs/hour

---

## Migration Guide

### From Thin to Default Mode

**When**: Traffic exceeds 100 jobs/hour

**Steps**:
1. Deploy dedicated scheduler:
   ```yaml
   scheduler:
     environment:
       - WORKER_MODE=scheduler-only
     deploy:
       replicas: 1
   ```

2. Update worker to default mode:
   ```yaml
   worker:
     environment:
       - WORKER_MODE=default
       - WORKER_CONCURRENCY=20
       - ENABLE_SCHEDULER=false  # Use dedicated scheduler
     deploy:
       replicas: 3
   ```

3. Monitor queue depth and worker utilization
4. Scale horizontally if needed: `replicas: 5`

**Benefits**:
- ✅ 60x capacity increase (5 → 300 workers)
- ✅ Horizontal scaling capability
- ✅ Better resource utilization

---

### From Default to Specialized Mode

**When**: High-priority jobs experiencing delays

**Steps**:
1. Deploy high-priority workers:
   ```yaml
   worker-high:
     environment:
       - WORKER_MODE=specialized
       - WORKER_PRIORITIES=high
       - WORKER_CONCURRENCY=50
     deploy:
       replicas: 3
   ```

2. Deploy normal and low priority workers:
   ```yaml
   worker-normal:
     environment:
       - WORKER_MODE=specialized
       - WORKER_PRIORITIES=normal
       - WORKER_CONCURRENCY=30
     deploy:
       replicas: 2

   worker-low:
     environment:
       - WORKER_MODE=specialized
       - WORKER_PRIORITIES=low
       - WORKER_CONCURRENCY=10
     deploy:
       replicas: 1
   ```

3. Gradually reduce default mode workers
4. Remove default mode workers once stable

**Benefits**:
- ✅ Guaranteed resources for high-priority jobs
- ✅ Independent scaling per priority
- ✅ SLA compliance
- ✅ Prevent low-priority jobs from blocking high-priority

---

### From Default to Job-Specialized Mode

**When**: Different job types have different resource requirements

**Steps**:
1. Analyze job types and resource usage:
   ```bash
   # Group jobs by resource pattern
   CPU-intensive: generate_report, process_analytics
   I/O-intensive: fetch_data, upload_files, process_images
   Database: run_migrations, backup_data
   ```

2. Deploy specialized workers:
   ```yaml
   worker-cpu:
     environment:
       - WORKER_MODE=job-specialized
       - WORKER_JOB_TYPES=generate_report,process_analytics
       - WORKER_CONCURRENCY=10

   worker-io:
     environment:
       - WORKER_MODE=job-specialized
       - WORKER_JOB_TYPES=fetch_data,upload_files,process_images
       - WORKER_CONCURRENCY=50
   ```

3. Keep default mode as catch-all:
   ```yaml
   worker-general:
     environment:
       - WORKER_MODE=default
       - WORKER_CONCURRENCY=20
   ```

4. Monitor job distribution and adjust

**Benefits**:
- ✅ Optimized concurrency per job type
- ✅ Deploy on appropriate hardware
- ✅ Cost optimization
- ✅ Prevent resource contention

---

## Best Practices

### 1. Resource Allocation

#### CPU-Bound Jobs
```yaml
worker-cpu:
  environment:
    - WORKER_CONCURRENCY=10  # Low concurrency
  resources:
    limits:
      cpus: '4'              # High CPU
      memory: 2G
```

#### I/O-Bound Jobs
```yaml
worker-io:
  environment:
    - WORKER_CONCURRENCY=50  # High concurrency
  resources:
    limits:
      cpus: '2'              # Lower CPU
      memory: 4G             # High memory for caching
```

---

### 2. Scheduler Configuration

**Always Use Dedicated Scheduler in Production**:
```yaml
# Workers
worker:
  environment:
    - ENABLE_SCHEDULER=false  # ✅ Disable built-in scheduler

# Dedicated scheduler
scheduler:
  environment:
    - WORKER_MODE=scheduler-only
  deploy:
    replicas: 1  # ⚠️ MUST be exactly 1
```

**Why**:
- ✅ Prevents duplicate scheduled jobs
- ✅ Lighter weight workers
- ✅ Easier monitoring and debugging

---

### 3. Graceful Shutdown

Workers handle `SIGTERM` gracefully:
1. Stop accepting new jobs
2. Wait for active jobs to complete
3. Shutdown cleanly

**Kubernetes**:
```yaml
spec:
  terminationGracePeriodSeconds: 120  # Allow 2 minutes for jobs to finish
```

**Docker Compose**:
```yaml
services:
  worker:
    stop_grace_period: 2m
```

---

### 4. Health Checks

**Liveness Probe** (is worker alive?):
```yaml
livenessProbe:
  httpGet:
    path: /debug/pprof/
    port: 6061
  initialDelaySeconds: 30
  periodSeconds: 30
```

**Readiness Probe** (is worker ready?):
```yaml
readinessProbe:
  httpGet:
    path: /debug/pprof/
    port: 6061
  initialDelaySeconds: 10
  periodSeconds: 10
```

---

### 5. Monitoring

**Log Metrics Every 30 Seconds**:
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

**Key Metrics to Monitor**:
- **Worker Utilization**: Should be 60-80% (too low = over-provisioned, too high = under-provisioned)
- **Queue Depth**: Should not grow unbounded
- **Error Rate**: Should be <1%
- **Job Duration**: Track P50, P95, P99

---

### 6. Scaling Guidelines

#### When to Scale Horizontally (Add More Instances)
- Worker utilization consistently > 80%
- Queue depth growing
- Job processing latency increasing

```bash
# Docker Compose
docker-compose up --scale worker=5

# Kubernetes
kubectl scale deployment worker --replicas=5
```

#### When to Scale Vertically (Increase Concurrency)
- CPU/memory headroom available
- Jobs are I/O bound (waiting on network/disk)
- Worker utilization < 50%

```bash
WORKER_CONCURRENCY=50  # was 20
```

---

## Monitoring and Metrics

### Built-in Metrics

Workers log metrics every 30 seconds:

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

### Prometheus Integration

Workers expose pprof endpoint on port 6061:
```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "6061"
```

### Key Metrics

| Metric | Target | Action if Outside Range |
|--------|--------|------------------------|
| Worker Utilization | 60-80% | <60%: Reduce workers<br>>80%: Add workers |
| Queue Depth | <1000 | Scale workers up |
| Error Rate | <1% | Investigate job handlers |
| Avg Job Duration | Stable | Increasing: Performance issue |

### Alerting Rules

```yaml
# High priority queue depth critical
alert: HighPriorityQueueDepth
expr: bananas_queue_depth{priority="high"} > 100
for: 2m

# Worker utilization high
alert: WorkerUtilizationHigh
expr: bananas_worker_utilization > 90
for: 5m

# Error rate high
alert: ErrorRateHigh
expr: bananas_error_rate > 5
for: 5m
```

---

## Troubleshooting

### Problem: Workers Not Processing Jobs

**Symptoms**:
- Queue depth growing
- Workers idle
- No job logs

**Diagnosis**:
```bash
# Check worker logs
docker-compose logs worker

# Check Redis connection
docker-compose exec worker redis-cli -h redis ping

# Check queue depth
docker-compose exec redis redis-cli LLEN bananas:queue:high
```

**Solutions**:
1. Verify Redis connectivity
2. Check worker mode and priorities match queued jobs
3. Verify job handlers are registered
4. Check for job type filters (job-specialized mode)

---

### Problem: High Queue Depth

**Symptoms**:
- Queue depth > 1000
- Job latency increasing
- Workers at 100% utilization

**Diagnosis**:
```bash
# Check worker utilization
docker-compose logs worker | grep "worker_utilization"

# Check queue depth
docker-compose exec redis redis-cli LLEN bananas:queue:high
```

**Solutions**:
1. **Scale Horizontally**:
   ```bash
   docker-compose up --scale worker=10
   ```

2. **Increase Concurrency**:
   ```yaml
   environment:
     - WORKER_CONCURRENCY=50  # was 20
   ```

3. **Add Priority-Specific Workers** (specialized mode):
   ```bash
   docker-compose up --scale worker-high=5
   ```

---

### Problem: Jobs Timing Out

**Symptoms**:
- Jobs failing with timeout errors
- Job duration > `JOB_TIMEOUT`

**Diagnosis**:
```bash
# Check average job duration
docker-compose logs worker | grep "avg_duration_ms"

# Check timeout setting
echo $JOB_TIMEOUT
```

**Solutions**:
1. **Increase Timeout**:
   ```yaml
   environment:
     - JOB_TIMEOUT=10m  # was 5m
   ```

2. **Optimize Job Handlers**:
   - Profile slow jobs
   - Add caching
   - Optimize database queries
   - Use async I/O

---

### Problem: Multiple Schedulers Running

**Symptoms**:
- Scheduled jobs running multiple times
- Duplicate job execution logs

**Diagnosis**:
```bash
# Check scheduler replicas
docker-compose ps scheduler
kubectl get pods -l app=scheduler
```

**Solutions**:
1. **Ensure Exactly 1 Scheduler**:
   ```yaml
   scheduler:
     deploy:
       replicas: 1  # MUST be 1
   ```

2. **Use Leader Election** (advanced):
   - Implement Redis-based locking
   - Use Kubernetes leader election

---

### Problem: Workers Skipping Jobs

**Symptoms**:
- Jobs remain in queue
- Logs show "Skipping job due to job-type filter"

**Diagnosis**:
```bash
# Check worker job type filters
docker-compose logs worker | grep "Skipping job"

# Check WORKER_JOB_TYPES setting
docker-compose config | grep WORKER_JOB_TYPES
```

**Solutions**:
1. **Add Catch-All Worker**:
   ```yaml
   worker-general:
     environment:
       - WORKER_MODE=default  # Processes ALL job types
   ```

2. **Update Job Type Filter**:
   ```yaml
   worker-cpu:
     environment:
       - WORKER_JOB_TYPES=generate_report,process_analytics,new_job_type
   ```

---

## FAQ

### Q: Can I mix worker modes?

**A**: Yes! Common patterns:
- **Specialized + Default**: Priority-isolated workers + catch-all workers
- **Job-Specialized + Default**: Job-type workers + catch-all workers
- **Specialized + Job-Specialized**: Both priority AND job-type isolation

Example:
```yaml
# High priority workers (specialized)
worker-high:
  environment:
    - WORKER_MODE=specialized
    - WORKER_PRIORITIES=high

# CPU workers (job-specialized)
worker-cpu:
  environment:
    - WORKER_MODE=job-specialized
    - WORKER_JOB_TYPES=generate_report

# Catch-all workers (default)
worker-general:
  environment:
    - WORKER_MODE=default
```

---

### Q: How many workers do I need?

**A**: Calculate based on throughput and job duration:

```
Required Workers = (Jobs/Hour ÷ 3600) × Avg Job Duration (seconds)
```

**Example**:
- Traffic: 10,000 jobs/hour
- Avg duration: 2 seconds
- Required workers: (10,000 ÷ 3600) × 2 = 5.6 ≈ **6 workers**

**Add 30-50% buffer for peaks**: 6 × 1.4 = **8-9 workers**

---

### Q: Should I use dedicated scheduler?

**A**:

| Deployment | Use Dedicated Scheduler? |
|------------|-------------------------|
| Thin mode | ❌ No (built-in) |
| Single default worker | ❌ No (built-in) |
| Multiple workers | ✅ Yes |
| Specialized mode | ✅ Yes (required) |
| Job-specialized mode | ✅ Yes (required) |

**Why**: Prevents duplicate scheduled jobs, lighter workers

---

### Q: How do I handle job retries?

**A**: Configure `MAX_RETRIES`:

```yaml
environment:
  - MAX_RETRIES=3  # Retry up to 3 times
```

**Best Practices**:
- Idempotent jobs: `MAX_RETRIES=3`
- Non-idempotent jobs: `MAX_RETRIES=0`
- Database operations: `MAX_RETRIES=1`

---

### Q: Can workers auto-scale?

**A**: Yes, using Kubernetes HPA (Horizontal Pod Autoscaler):

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: worker-hpa
spec:
  scaleTargetRef:
    kind: Deployment
    name: worker
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

See [k8s-hpa.yml](../examples/deployments/k8s-hpa.yml) for complete examples.

---

### Q: How do I debug slow jobs?

**A**:

1. **Enable Debug Logging**:
   ```yaml
   environment:
     - LOG_LEVEL=debug
   ```

2. **Access pprof**:
   ```bash
   # CPU profile
   go tool pprof http://worker:6061/debug/pprof/profile?seconds=30

   # Memory profile
   go tool pprof http://worker:6061/debug/pprof/heap
   ```

3. **Check Metrics**:
   ```bash
   docker-compose logs worker | grep "avg_duration_ms"
   ```

---

## Further Reading

- [Worker Architecture Design](WORKER_ARCHITECTURE_DESIGN.md) - Technical design details
- [Deployment Examples](../examples/deployments/README.md) - Complete deployment configurations
- [Configuration Reference](CONFIGURATION.md) - All configuration options
- [API Documentation](API.md) - Job enqueuing and management

---

**Last Updated**: 2025-11-10
**Version**: 1.0.0

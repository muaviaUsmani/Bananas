# Troubleshooting Guide

This guide covers common failure scenarios, error messages, and recovery procedures for the Bananas distributed task queue.

## Table of Contents

- [Common Failure Scenarios](#common-failure-scenarios)
- [Error Messages and Solutions](#error-messages-and-solutions)
- [Dead Letter Queue Management](#dead-letter-queue-management)
- [Redis Connection Issues](#redis-connection-issues)
- [Job Handler Issues](#job-handler-issues)
- [Performance Issues](#performance-issues)
- [Recovery Procedures](#recovery-procedures)

---

## Common Failure Scenarios

### 1. Job Handler Panics

**Symptoms:**
- Jobs fail with "PANIC" in error message
- Stack traces in logs
- Jobs moved to dead letter queue

**Root Causes:**
- Nil pointer dereference
- Array/slice index out of bounds
- Division by zero
- Unhandled edge cases in handler code

**Solution:**
```go
// BAD: Can panic
func HandleBadJob(ctx context.Context, j *job.Job) error {
    data := j.Payload.(map[string]interface{})
    value := data["key"].(int) // Panic if wrong type or nil
    return nil
}

// GOOD: Safe type assertions
func HandleGoodJob(ctx context.Context, j *job.Job) error {
    data, ok := j.Payload.(map[string]interface{})
    if !ok {
        return fmt.Errorf("invalid payload type")
    }

    value, ok := data["key"].(int)
    if !ok {
        return fmt.Errorf("missing or invalid 'key' field")
    }

    return nil
}
```

**Automatic Recovery:**
- Worker catches panic and logs stack trace
- Job marked as failed and retried (up to MaxRetries)
- After max retries, moved to dead letter queue
- Worker continues processing other jobs

---

### 2. Redis Connection Failures

**Symptoms:**
- Workers log "Redis connection error - retrying with backoff"
- Increasing backoff delays: 2s, 4s, 8s, 16s, 30s (max)
- Jobs stop being processed

**Root Causes:**
- Redis server down or restarting
- Network connectivity issues
- Redis out of memory
- Connection pool exhausted

**Automatic Recovery:**
- Workers retry with exponential backoff
- Connection automatically restored when Redis recovers
- Workers log "Redis connection recovered" when restored

**Manual Intervention:**

```bash
# Check Redis status
redis-cli ping

# Check Redis memory usage
redis-cli INFO memory

# Check connection count
redis-cli INFO clients

# Restart Redis if needed
sudo systemctl restart redis

# Check worker logs
tail -f /var/log/bananas/worker.log
```

---

### 3. Job Timeouts

**Symptoms:**
- Jobs fail with "context cancelled" or "context deadline exceeded"
- Job duration in metrics matches timeout value

**Root Causes:**
- Job takes longer than configured timeout
- External API calls too slow
- Database queries too slow
- Insufficient resources (CPU/memory)

**Solution:**

```go
// Option 1: Respect context in handler
func HandleLongJob(ctx context.Context, j *job.Job) error {
    for i := 0; i < 1000; i++ {
        // Check if job was cancelled
        select {
        case <-ctx.Done():
            return ctx.Err() // Gracefully handle cancellation
        default:
            // Continue processing
        }

        // Do work...
        processItem(i)
    }
    return nil
}

// Option 2: Increase timeout for specific job types
// In configuration:
JOB_TIMEOUT=300s  // 5 minutes instead of default 30s
```

**When to Scale vs Fix Code:**

| Symptom | Action |
|---------|--------|
| All jobs timing out | Increase timeout or optimize code |
| Only complex jobs timing out | Increase timeout for those jobs |
| Timeouts started recently | Check system resources, external dependencies |
| Random timeouts | Network/Redis issues, add retries |

---

### 4. Corrupted Job Data

**Symptoms:**
- "Failed to unmarshal job" errors
- "Job data not found" errors
- Jobs immediately moved to dead letter queue (no retry)

**Root Causes:**
- Manual Redis data corruption
- Incomplete writes to Redis
- Redis server crash during write
- Client/server version mismatch

**Automatic Handling:**
- Corrupted jobs immediately moved to dead letter queue
- Corrupted data logged (first 500 chars) for debugging
- Worker continues processing other jobs
- No retries (permanent failure)

**Recovery:**

```bash
# Inspect dead letter queue
redis-cli LRANGE bananas:queue:dead 0 -1

# Get corrupted job data
redis-cli GET bananas:job:<job-id>

# If job data is recoverable, fix and re-enqueue
redis-cli GET bananas:job:<job-id> | jq '.' | # fix JSON
redis-cli SET bananas:job:<job-id> '<fixed-json>'
redis-cli RPOPLPUSH bananas:queue:dead bananas:queue:normal
```

---

### 5. High Error Rates

**Symptoms:**
- Metrics show error_rate > 5%
- Many jobs in dead letter queue
- Logs filled with job failures

**Diagnosis:**

```bash
# Check error rate in metrics
# Look for "System metrics" log entries:
# error_rate: 15.23%  # This is high!

# Count jobs in dead letter queue
redis-cli LLEN bananas:queue:dead

# Sample failed jobs
redis-cli LRANGE bananas:queue:dead 0 10
redis-cli GET bananas:job:<job-id-from-dlq>
```

**Common Causes:**

| Error Rate | Likely Cause | Action |
|------------|--------------|--------|
| 0-2% | Normal failures | Monitor, no action needed |
| 2-10% | External service issues | Check dependencies |
| 10-50% | Code bug in handler | Review recent code changes |
| >50% | Critical system issue | Immediate investigation |

**Solutions:**
1. **Code bug:** Fix handler code, redeploy
2. **External dependency:** Add retries with backoff, circuit breaker
3. **Invalid data:** Add input validation, clear dead letter queue
4. **Resource exhaustion:** Scale workers, optimize handlers

---

## Dead Letter Queue Management

### Inspecting Dead Letter Queue

```bash
# Count jobs in DLQ
redis-cli LLEN bananas:queue:dead

# List job IDs (first 100)
redis-cli LRANGE bananas:queue:dead 0 99

# Get job details
redis-cli GET bananas:job:<job-id>
```

### Analyzing Failed Jobs

```go
// Tool to analyze DLQ jobs
package main

import (
    "context"
    "fmt"
    "github.com/muaviaUsmani/bananas/internal/queue"
)

func analyzeDLQ() {
    ctx := context.Background()
    q, _ := queue.NewRedisQueue("redis://localhost:6379")

    length, _ := q.DeadLetterQueueLength(ctx)
    fmt.Printf("Dead letter queue has %d jobs\n", length)

    // TODO: Add code to fetch and analyze failed jobs
}
```

### Re-enqueueing Failed Jobs

**After fixing the issue:**

```bash
# Move all DLQ jobs back to normal queue
while true; do
    JOB_ID=$(redis-cli RPOPLPUSH bananas:queue:dead bananas:queue:normal)
    if [ "$JOB_ID" == "" ]; then break; fi
    echo "Re-enqueued: $JOB_ID"
done

# Or move to specific priority queue
redis-cli RPOPLPUSH bananas:queue:dead bananas:queue:high
```

### Clearing Dead Letter Queue

**CAUTION:** This permanently deletes failed jobs!

```bash
# Backup DLQ before clearing
redis-cli LRANGE bananas:queue:dead 0 -1 > dlq_backup.json

# Clear DLQ
redis-cli DEL bananas:queue:dead

# Verify
redis-cli LLEN bananas:queue:dead  # Should be 0
```

---

## Error Messages and Solutions

### "context deadline exceeded"

**Meaning:** Job exceeded configured timeout

**Solution:**
- Increase `JOB_TIMEOUT` environment variable
- Optimize handler code to run faster
- Add progress checkpoints to save work
- Break large jobs into smaller chunks

### "PANIC: runtime error: invalid memory address"

**Meaning:** Nil pointer dereference in handler

**Solution:**
- Add nil checks before dereferencing pointers
- Use safe type assertions with `ok` return value
- Review stack trace to find exact location

### "Failed to unmarshal job: invalid character"

**Meaning:** Corrupted job data in Redis

**Solution:**
- Job automatically moved to DLQ
- Inspect with `redis-cli GET bananas:job:<id>`
- Fix data corruption source (client code, Redis config)
- Manually fix and re-enqueue if valuable

### "Redis connection error - retrying with backoff"

**Meaning:** Cannot connect to Redis

**Solution:**
- Check Redis server status: `redis-cli ping`
- Verify network connectivity
- Check Redis logs: `/var/log/redis/redis-server.log`
- Worker will automatically reconnect when Redis recovers

### "Job data not found for ID"

**Meaning:** Job ID in queue but data missing (orphaned)

**Solution:**
- Job automatically moved to DLQ
- Likely cause: manual Redis data deletion
- Cannot recover - job data is lost
- Review processes that might delete Redis keys

---

## Redis Connection Issues

### Connection Pool Exhausted

**Symptoms:**
```
Error: connection pool timeout: no available connection
```

**Diagnosis:**
```bash
# Check current connections
redis-cli CLIENT LIST | wc -l

# Check pool configuration
# In code: opts.PoolSize = 50 (default)
```

**Solution:**
- Increase pool size: Modify `opts.PoolSize` in code
- Reduce worker concurrency if pool size can't increase
- Check for connection leaks (not closing connections)

### Redis Out of Memory

**Symptoms:**
```
Error: OOM command not allowed when used memory > 'maxmemory'
```

**Diagnosis:**
```bash
redis-cli INFO memory | grep used_memory_human
redis-cli CONFIG GET maxmemory
```

**Solution:**
```bash
# Increase Redis memory limit
redis-cli CONFIG SET maxmemory 2gb

# Enable eviction policy for old data
redis-cli CONFIG SET maxmemory-policy allkeys-lru

# Or clean up old completed jobs
redis-cli --scan --pattern "bananas:job:*" | xargs redis-cli DEL
```

---

## Performance Issues

### Slow Job Processing

**Diagnosis:**
1. Check metrics: `avg_duration_ms` increasing
2. Check worker utilization: Should be 70-90%
3. Profile handlers: Add timing logs

**Solutions:**

| Symptom | Solution |
|---------|----------|
| High avg_duration, low utilization | Optimize handler code |
| High avg_duration, high utilization | Add more workers |
| Low utilization | Increase job throughput (more enqueuers) |
| Queue depth growing | Critical: Add workers immediately |

### Memory Leaks

**Symptoms:**
- Worker memory usage constantly growing
- Eventually crashes with OOM

**Diagnosis:**
```bash
# Monitor worker memory
ps aux | grep worker
top -p <worker-pid>

# Use pprof for detailed analysis
curl http://localhost:6061/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

**Common Causes:**
- Not closing database connections
- Accumulating data in global variables
- Not releasing large allocations

---

## Recovery Procedures

### System-Wide Failure Recovery

1. **Stop all workers gracefully:**
   ```bash
   # Workers handle SIGTERM gracefully
   pkill -TERM worker
   ```

2. **Check Redis status:**
   ```bash
   redis-cli ping
   redis-cli INFO
   ```

3. **Verify queue integrity:**
   ```bash
   redis-cli LLEN bananas:queue:high
   redis-cli LLEN bananas:queue:normal
   redis-cli LLEN bananas:queue:low
   redis-cli LLEN bananas:queue:processing
   redis-cli LLEN bananas:queue:dead
   ```

4. **Review logs:**
   ```bash
   tail -100 /var/log/bananas/worker.log
   tail -100 /var/log/bananas/api.log
   ```

5. **Restart workers:**
   ```bash
   systemctl start bananas-worker
   ```

6. **Monitor recovery:**
   ```bash
   # Watch metrics in logs
   tail -f /var/log/bananas/worker.log | grep "System metrics"
   ```

### Redis Data Corruption Recovery

1. **Stop all workers**
2. **Backup Redis data:**
   ```bash
   redis-cli BGSAVE
   cp /var/lib/redis/dump.rdb /backup/redis-$(date +%Y%m%d).rdb
   ```

3. **Analyze corruption:**
   ```bash
   redis-cli --scan --pattern "bananas:job:*" | while read key; do
       redis-cli GET $key > /dev/null 2>&1 || echo "Corrupted: $key"
   done
   ```

4. **Remove corrupted keys:**
   ```bash
   # Manually or with script
   redis-cli DEL <corrupted-key>
   ```

5. **Restart workers and monitor**

---

## When to Scale vs When to Fix Code

### Scale Horizontally (Add Workers)

- ✅ Queue depth consistently > 1000 jobs
- ✅ Worker utilization consistently > 90%
- ✅ Jobs are already optimized
- ✅ External APIs are the bottleneck

### Scale Vertically (Bigger Machines)

- ✅ CPU usage consistently > 80%
- ✅ Memory usage growing
- ✅ Redis on same machine as workers

### Fix Code

- ✅ Error rate > 5%
- ✅ Jobs timing out frequently
- ✅ Memory leaks detected
- ✅ Obvious inefficiencies in handlers
- ✅ N+1 database queries

### Optimize Configuration

- ✅ Worker utilization < 50%
- ✅ Many short jobs (< 100ms)
- ✅ Connection pool timeouts
- ✅ Increase job timeout for specific handlers

---

## Monitoring Best Practices

### Key Metrics to Watch

| Metric | Healthy | Warning | Critical |
|--------|---------|---------|----------|
| Error Rate | < 2% | 2-10% | > 10% |
| Worker Utilization | 70-90% | > 95% or < 50% | 100% for > 5min |
| Avg Job Duration | Stable | +50% from baseline | +100% from baseline |
| Queue Depth | < 100 | 100-1000 | > 1000 |
| DLQ Size | < 10 | 10-100 | > 100 |

### Alert Thresholds

```yaml
alerts:
  - name: HighErrorRate
    condition: error_rate > 10%
    duration: 5m
    severity: critical

  - name: QueueBacklog
    condition: queue_depth > 1000
    duration: 2m
    severity: warning

  - name: DLQGrowing
    condition: dlq_size > 100
    duration: 10m
    severity: warning

  - name: RedisDown
    condition: consecutive_failures > 5
    severity: critical
```

---

## Getting Help

If you can't resolve an issue:

1. **Collect diagnostic information:**
   ```bash
   # Worker logs
   tail -500 /var/log/bananas/worker.log > logs.txt

   # Redis info
   redis-cli INFO > redis-info.txt

   # Queue stats
   redis-cli LLEN bananas:queue:high
   redis-cli LLEN bananas:queue:dead

   # System metrics from logs
   grep "System metrics" /var/log/bananas/worker.log | tail -20
   ```

2. **Check documentation:**
   - [Main README](../README.md)
   - [Logging Guide](LOGGING.md)
   - [Performance Guide](PERFORMANCE.md)

3. **Search existing issues:**
   - GitHub Issues: https://github.com/muaviaUsmani/bananas/issues

4. **File a new issue with:**
   - Description of the problem
   - Steps to reproduce
   - Relevant logs and metrics
   - Configuration (redact sensitive data)
   - Environment (OS, Go version, Redis version)

---

## Appendix: Useful Redis Commands

```bash
# Queue inspection
redis-cli LRANGE bananas:queue:high 0 9        # First 10 high priority jobs
redis-cli LRANGE bananas:queue:processing 0 -1  # All processing jobs
redis-cli LRANGE bananas:queue:dead 0 -1        # All DLQ jobs

# Job inspection
redis-cli GET bananas:job:<job-id>              # Get job details
redis-cli KEYS "bananas:job:*" | wc -l          # Count all jobs

# Queue management
redis-cli LPUSH bananas:queue:high <job-id>     # Add job to high queue
redis-cli RPOPLPUSH bananas:queue:dead bananas:queue:normal  # Move DLQ job back

# Cleanup
redis-cli DEL bananas:queue:dead                # Clear DLQ
redis-cli KEYS "bananas:job:*" | xargs redis-cli DEL  # Delete all jobs (DANGEROUS!)

# Monitoring
redis-cli --scan --pattern "bananas:*" | wc -l  # Count all Bananas keys
redis-cli MEMORY USAGE bananas:queue:high       # Memory used by queue
```

---

**Last Updated:** 2025-11-10

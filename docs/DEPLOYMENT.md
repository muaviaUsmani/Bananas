# Production Deployment Guide

> **Last Updated:** 2025-11-10
> **Status:** Production-Ready

Complete guide for deploying Bananas in production environments.

## Table of Contents

- [Overview](#overview)
- [Architecture Decisions](#architecture-decisions)
- [Infrastructure Requirements](#infrastructure-requirements)
- [Deployment Patterns](#deployment-patterns)
- [Configuration](#configuration)
- [Monitoring & Observability](#monitoring--observability)
- [Security](#security)
- [Scaling](#scaling)
- [Troubleshooting](#troubleshooting)
- [Disaster Recovery](#disaster-recovery)

---

## Overview

Bananas is designed for production deployment with high availability, scalability, and reliability in mind.

### Deployment Checklist

- [ ] Redis cluster setup (HA configuration)
- [ ] Worker pools configured for routing keys
- [ ] Scheduler deployed (with distributed locking)
- [ ] Monitoring and metrics configured
- [ ] Logging centralized
- [ ] Security hardened (Redis AUTH, TLS)
- [ ] Backup and recovery procedures
- [ ] Load testing completed
- [ ] Runbooks created

---

## Architecture Decisions

### Single Data Center

```
┌─────────────────────────────────────────────────────────────┐
│                      Load Balancer                          │
│                    (API Traffic - Optional)                 │
└────────────────────────┬────────────────────────────────────┘
                         │
         ┌───────────────┼───────────────┐
         │               │               │
         ▼               ▼               ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│ API Server 1 │ │ API Server 2 │ │ API Server N │
└──────┬───────┘ └──────┬───────┘ └──────┬───────┘
       │                │                │
       └────────────────┼────────────────┘
                        │
                        ▼
        ┌───────────────────────────────┐
        │   Redis Sentinel Cluster      │
        │  ┌──────────┐  ┌───────────┐  │
        │  │  Master  │  │  Replica  │  │
        │  └──────────┘  └───────────┘  │
        │  ┌───────────┐                │
        │  │  Replica  │                │
        │  └───────────┘                │
        └───────────────────────────────┘
                        │
         ┌──────────────┼──────────────┐
         │              │              │
         ▼              ▼              ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│ Worker Pool  │ │ Worker Pool  │ │  Scheduler   │
│ (GPU Jobs)   │ │ (Email Jobs) │ │   Process    │
│ - 10 workers │ │ - 5 workers  │ │              │
│ - gpu route  │ │ - email route│ │ - Cron jobs  │
└──────────────┘ └──────────────┘ └──────────────┘
```

**Pros:**
- Simpler configuration
- Lower latency
- Easier to manage

**Cons:**
- Single point of failure (data center)
- Limited geographic distribution

### Multi-Region

```
┌──────────── Region US-EAST-1 ────────────┐  ┌──────────── Region EU-WEST-1 ────────────┐
│                                          │  │                                          │
│  ┌────────────┐      ┌────────────┐     │  │  ┌────────────┐      ┌────────────┐     │
│  │ Redis      │      │ Workers    │     │  │  │ Redis      │      │ Workers    │     │
│  │ Master     │◄─────┤ (US Jobs)  │     │  │  │ Replica    │◄─────┤ (EU Jobs)  │     │
│  └────────────┘      └────────────┘     │  │  └────────────┘      └────────────┘     │
│        │                                 │  │        ▲                                 │
│        │ Replication                     │  │        │                                 │
│        │                                 │  │        │                                 │
└────────┼─────────────────────────────────┘  └────────┼─────────────────────────────────┘
         │                                               │
         └───────────────────────────────────────────────┘
                     Cross-region replication
```

**Routing Strategy:**
```bash
# US workers
WORKER_ROUTING_KEYS=us-east-1,default

# EU workers
WORKER_ROUTING_KEYS=eu-west-1,default
```

**Pros:**
- Geographic distribution
- Lower latency for users
- Fault tolerance

**Cons:**
- Replication lag
- More complex configuration
- Higher costs

---

## Infrastructure Requirements

### Minimum Production Setup

**For ~1000 jobs/hour:**

| Component | Specs | Quantity |
|-----------|-------|----------|
| Redis | 2 CPU, 4GB RAM | 1 master + 1 replica |
| Workers | 1 CPU, 2GB RAM | 2-3 machines |
| Scheduler | 1 CPU, 1GB RAM | 1 machine |
| API (optional) | 1 CPU, 2GB RAM | 2 machines |

**For ~10,000 jobs/hour:**

| Component | Specs | Quantity |
|-----------|-------|----------|
| Redis | 4 CPU, 8GB RAM | 1 master + 2 replicas |
| Workers | 2 CPU, 4GB RAM | 5-10 machines |
| Scheduler | 1 CPU, 1GB RAM | 1 machine |
| API (optional) | 2 CPU, 4GB RAM | 3-5 machines |

**For ~100,000+ jobs/hour:**

| Component | Specs | Quantity |
|-----------|-------|----------|
| Redis Cluster | 8 CPU, 16GB RAM | 3+ shards, 2 replicas each |
| Workers | 4 CPU, 8GB RAM | 20-50+ machines |
| Scheduler | 1 CPU, 2GB RAM | 1 machine |
| API (optional) | 4 CPU, 8GB RAM | 10+ machines |

### Redis Configuration

**Production `redis.conf`:**
```conf
# Memory
maxmemory 8gb
maxmemory-policy allkeys-lru

# Persistence (RDB + AOF for durability)
save 900 1
save 300 10
save 60 10000
appendonly yes
appendfsync everysec

# Replication
min-replicas-to-write 1
min-replicas-max-lag 10

# Network
timeout 300
tcp-keepalive 60

# Security
requirepass your_strong_password_here
protected-mode yes

# Performance
tcp-backlog 511
maxclients 10000
```

**Redis Sentinel Configuration:**
```conf
sentinel monitor bananas-master 10.0.1.10 6379 2
sentinel auth-pass bananas-master your_strong_password_here
sentinel down-after-milliseconds bananas-master 5000
sentinel parallel-syncs bananas-master 1
sentinel failover-timeout bananas-master 60000
```

---

## Deployment Patterns

### Pattern 1: Docker Compose (Simple)

**Use Case:** Development, staging, small production

```yaml
version: '3.8'

services:
  redis:
    image: redis:7-alpine
    command: redis-server --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis-data:/data
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

  worker-default:
    image: bananas:latest
    command: /app/worker
    environment:
      REDIS_URL: redis://:${REDIS_PASSWORD}@redis:6379/0
      WORKER_MODE: default
      WORKER_CONCURRENCY: 10
      WORKER_ROUTING_KEYS: default
      LOG_LEVEL: info
    depends_on:
      redis:
        condition: service_healthy
    deploy:
      replicas: 3
      restart_policy:
        condition: on-failure
        delay: 5s
        max_attempts: 3

  worker-gpu:
    image: bananas:latest
    command: /app/worker
    environment:
      REDIS_URL: redis://:${REDIS_PASSWORD}@redis:6379/0
      WORKER_MODE: default
      WORKER_CONCURRENCY: 5
      WORKER_ROUTING_KEYS: gpu,default
      LOG_LEVEL: info
    depends_on:
      redis:
        condition: service_healthy
    deploy:
      replicas: 2
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]

  scheduler:
    image: bananas:latest
    command: /app/scheduler
    environment:
      REDIS_URL: redis://:${REDIS_PASSWORD}@redis:6379/0
      SCHEDULER_INTERVAL: 1s
      LOG_LEVEL: info
    depends_on:
      redis:
        condition: service_healthy
    deploy:
      replicas: 1

  api:
    image: bananas:latest
    command: /app/api
    environment:
      REDIS_URL: redis://:${REDIS_PASSWORD}@redis:6379/0
      API_PORT: 8080
      LOG_LEVEL: info
    ports:
      - "8080:8080"
    depends_on:
      redis:
        condition: service_healthy
    deploy:
      replicas: 3

volumes:
  redis-data:
```

### Pattern 2: Kubernetes (Scalable)

**Namespace:**
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: bananas
```

**Redis StatefulSet:**
```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: redis
  namespace: bananas
spec:
  serviceName: redis
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        args:
          - --requirepass
          - $(REDIS_PASSWORD)
          - --appendonly
          - "yes"
        env:
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: redis-secret
              key: password
        ports:
        - containerPort: 6379
          name: redis
        volumeMounts:
        - name: redis-data
          mountPath: /data
        resources:
          requests:
            cpu: 2
            memory: 4Gi
          limits:
            cpu: 4
            memory: 8Gi
        livenessProbe:
          exec:
            command:
            - redis-cli
            - ping
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          exec:
            command:
            - redis-cli
            - ping
          initialDelaySeconds: 5
          periodSeconds: 5
  volumeClaimTemplates:
  - metadata:
      name: redis-data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 50Gi
```

**Worker Deployment (Default):**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker-default
  namespace: bananas
spec:
  replicas: 5
  selector:
    matchLabels:
      app: worker
      routing: default
  template:
    metadata:
      labels:
        app: worker
        routing: default
    spec:
      containers:
      - name: worker
        image: bananas:latest
        command: ["/app/worker"]
        env:
        - name: REDIS_URL
          value: "redis://:$(REDIS_PASSWORD)@redis:6379/0"
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: redis-secret
              key: password
        - name: WORKER_MODE
          value: "default"
        - name: WORKER_CONCURRENCY
          value: "10"
        - name: WORKER_ROUTING_KEYS
          value: "default"
        - name: LOG_LEVEL
          value: "info"
        resources:
          requests:
            cpu: 1
            memory: 2Gi
          limits:
            cpu: 2
            memory: 4Gi
        livenessProbe:
          exec:
            command:
            - /bin/sh
            - -c
            - pgrep -x worker
          initialDelaySeconds: 30
          periodSeconds: 10
```

**Worker Deployment (GPU):**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker-gpu
  namespace: bananas
spec:
  replicas: 2
  selector:
    matchLabels:
      app: worker
      routing: gpu
  template:
    metadata:
      labels:
        app: worker
        routing: gpu
    spec:
      nodeSelector:
        gpu: "true"
      containers:
      - name: worker
        image: bananas:latest
        command: ["/app/worker"]
        env:
        - name: REDIS_URL
          value: "redis://:$(REDIS_PASSWORD)@redis:6379/0"
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: redis-secret
              key: password
        - name: WORKER_MODE
          value: "default"
        - name: WORKER_CONCURRENCY
          value: "5"
        - name: WORKER_ROUTING_KEYS
          value: "gpu,default"
        - name: LOG_LEVEL
          value: "info"
        resources:
          requests:
            cpu: 2
            memory: 4Gi
            nvidia.com/gpu: 1
          limits:
            cpu: 4
            memory: 8Gi
            nvidia.com/gpu: 1
```

**Scheduler Deployment:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: scheduler
  namespace: bananas
spec:
  replicas: 1  # Only 1 scheduler needed (distributed locking)
  selector:
    matchLabels:
      app: scheduler
  template:
    metadata:
      labels:
        app: scheduler
    spec:
      containers:
      - name: scheduler
        image: bananas:latest
        command: ["/app/scheduler"]
        env:
        - name: REDIS_URL
          value: "redis://:$(REDIS_PASSWORD)@redis:6379/0"
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: redis-secret
              key: password
        - name: SCHEDULER_INTERVAL
          value: "1s"
        - name: LOG_LEVEL
          value: "info"
        resources:
          requests:
            cpu: 500m
            memory: 1Gi
          limits:
            cpu: 1
            memory: 2Gi
```

**HorizontalPodAutoscaler:**
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: worker-default-hpa
  namespace: bananas
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: worker-default
  minReplicas: 5
  maxReplicas: 50
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Pods
    pods:
      metric:
        name: queue_depth
      target:
        type: AverageValue
        averageValue: "100"
```

### Pattern 3: Systemd (Bare Metal/VMs)

**Worker Service (`/etc/systemd/system/bananas-worker@.service`):**
```ini
[Unit]
Description=Bananas Worker (%i)
After=network.target redis.service
Wants=redis.service

[Service]
Type=simple
User=bananas
Group=bananas
WorkingDirectory=/opt/bananas
Environment="REDIS_URL=redis://:password@localhost:6379/0"
Environment="WORKER_MODE=default"
Environment="WORKER_CONCURRENCY=10"
Environment="WORKER_ROUTING_KEYS=%i"
Environment="LOG_LEVEL=info"
ExecStart=/opt/bananas/bin/worker
Restart=always
RestartSec=5s
StandardOutput=append:/var/log/bananas/worker-%i.log
StandardError=append:/var/log/bananas/worker-%i-error.log

[Install]
WantedBy=multi-user.target
```

**Usage:**
```bash
# Start default workers
systemctl start bananas-worker@default
systemctl enable bananas-worker@default

# Start GPU workers
systemctl start bananas-worker@gpu
systemctl enable bananas-worker@gpu

# Status
systemctl status bananas-worker@default
```

---

## Configuration

### Environment Variables

**Complete Configuration:**
```bash
# Redis Connection
REDIS_URL=redis://username:password@host:6379/0

# Worker Configuration
WORKER_MODE=default
WORKER_CONCURRENCY=10
WORKER_PRIORITIES=high,normal,low
WORKER_ROUTING_KEYS=default
WORKER_JOB_TYPES=  # Empty = all types

# Scheduler
SCHEDULER_INTERVAL=1s
ENABLE_SCHEDULER=true

# Result Backend
RESULT_BACKEND_ENABLED=true
RESULT_BACKEND_TTL_SUCCESS=1h
RESULT_BACKEND_TTL_FAILURE=24h

# Logging
LOG_LEVEL=info  # debug, info, warn, error
LOG_FORMAT=json  # json, text
LOG_OUTPUT=stdout  # stdout, file, elasticsearch

# Metrics (Prometheus)
METRICS_ENABLED=true
METRICS_PORT=9090

# API Server (if using)
API_PORT=8080
API_CORS_ORIGINS=*
```

### Configuration Management

**Using Secrets:**
```bash
# Kubernetes
kubectl create secret generic redis-secret \
  --from-literal=password=your_strong_password \
  -n bananas

# Docker Swarm
docker secret create redis_password /path/to/password/file
```

**Using ConfigMaps:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: worker-config
  namespace: bananas
data:
  worker.env: |
    WORKER_MODE=default
    WORKER_CONCURRENCY=10
    WORKER_ROUTING_KEYS=default
    LOG_LEVEL=info
```

---

## Monitoring & Observability

### Metrics (Prometheus)

**Exposed Metrics:**
```
# Job metrics
bananas_jobs_enqueued_total{priority,routing_key}
bananas_jobs_completed_total{priority,routing_key}
bananas_jobs_failed_total{priority,routing_key}
bananas_job_duration_seconds{priority,routing_key}

# Queue metrics
bananas_queue_depth{priority,routing_key}
bananas_processing_queue_depth
bananas_dead_letter_queue_depth

# Worker metrics
bananas_worker_active
bananas_worker_total
bananas_worker_utilization
```

**Prometheus Configuration:**
```yaml
scrape_configs:
  - job_name: 'bananas-workers'
    static_configs:
      - targets:
        - worker-1:9090
        - worker-2:9090
        - worker-3:9090
    metrics_path: /metrics
    scrape_interval: 15s

  - job_name: 'bananas-api'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - bananas
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: keep
        regex: api
```

**Grafana Dashboards:**
```json
{
  "dashboard": {
    "title": "Bananas Queue Metrics",
    "panels": [
      {
        "title": "Job Processing Rate",
        "targets": [
          {
            "expr": "rate(bananas_jobs_completed_total[5m])"
          }
        ]
      },
      {
        "title": "Queue Depth",
        "targets": [
          {
            "expr": "bananas_queue_depth"
          }
        ]
      },
      {
        "title": "Worker Utilization",
        "targets": [
          {
            "expr": "bananas_worker_utilization"
          }
        ]
      }
    ]
  }
}
```

### Logging

**Structured Logging (JSON):**
```json
{
  "time": "2025-11-10T12:00:00Z",
  "level": "info",
  "msg": "Job completed",
  "job_id": "abc-123",
  "job_name": "send_email",
  "priority": "normal",
  "routing_key": "email",
  "duration_ms": 150,
  "worker_id": "worker-5"
}
```

**Centralized Logging (ELK Stack):**
```yaml
# Filebeat configuration
filebeat.inputs:
  - type: log
    enabled: true
    paths:
      - /var/log/bananas/*.log
    json.keys_under_root: true
    json.add_error_key: true

output.elasticsearch:
  hosts: ["https://elasticsearch:9200"]
  index: "bananas-logs-%{+yyyy.MM.dd}"
```

### Alerting

**Prometheus Alerts:**
```yaml
groups:
  - name: bananas
    interval: 30s
    rules:
      - alert: HighQueueDepth
        expr: bananas_queue_depth > 1000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High queue depth ({{ $value }} jobs)"
          description: "Queue depth has been above 1000 for 5 minutes"

      - alert: HighFailureRate
        expr: rate(bananas_jobs_failed_total[5m]) > 10
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "High job failure rate"
          description: "More than 10 jobs failing per minute"

      - alert: WorkerDown
        expr: up{job="bananas-workers"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Worker is down"
          description: "Worker {{ $labels.instance }} is unreachable"

      - alert: RedisDown
        expr: redis_up == 0
        for: 30s
        labels:
          severity: critical
        annotations:
          summary: "Redis is down"
          description: "Redis instance is unreachable"
```

---

## Security

### Redis Security

**1. Authentication:**
```conf
# redis.conf
requirepass your_very_strong_password_here

# Connection string
redis://:your_very_strong_password_here@localhost:6379/0
```

**2. TLS/SSL:**
```conf
# redis.conf
port 0
tls-port 6380
tls-cert-file /path/to/redis.crt
tls-key-file /path/to/redis.key
tls-ca-cert-file /path/to/ca.crt
tls-auth-clients optional
```

**Connection with TLS:**
```go
opts, _ := redis.ParseURL("rediss://:password@localhost:6380")
opts.TLSConfig = &tls.Config{
    MinVersion: tls.VersionTLS12,
}
client := redis.NewClient(opts)
```

**3. Network Security:**
```conf
# Bind to specific interfaces
bind 127.0.0.1 ::1 10.0.1.10

# Protected mode
protected-mode yes

# Rename dangerous commands
rename-command FLUSHDB ""
rename-command FLUSHALL ""
rename-command CONFIG "CONFIG_abc123xyz"
```

**4. Firewall Rules:**
```bash
# Allow only worker IPs
iptables -A INPUT -p tcp --dport 6379 -s 10.0.1.0/24 -j ACCEPT
iptables -A INPUT -p tcp --dport 6379 -j DROP
```

### Application Security

**1. Input Validation:**
```go
// Validate routing keys
if err := job.ValidateRoutingKey(routingKey); err != nil {
    return fmt.Errorf("invalid routing key: %w", err)
}

// Sanitize job names
validJobName := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
if !validJobName.MatchString(jobName) {
    return errors.New("invalid job name")
}
```

**2. Rate Limiting (API):**
```go
import "golang.org/x/time/rate"

limiter := rate.NewLimiter(100, 200) // 100 req/s, burst 200

func RateLimitMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !limiter.Allow() {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

**3. Secrets Management:**
```bash
# Using HashiCorp Vault
vault kv put secret/bananas/redis password=your_password

# In application
export REDIS_PASSWORD=$(vault kv get -field=password secret/bananas/redis)
```

---

## Scaling

### Horizontal Scaling Strategies

**1. Scale Workers by Routing Key:**
```bash
# Current load: GPU queue has 5000 jobs
# Action: Add more GPU workers

# Kubernetes
kubectl scale deployment worker-gpu --replicas=10 -n bananas

# Docker Swarm
docker service scale bananas_worker-gpu=10
```

**2. Scale Workers by Priority:**
```bash
# High priority queue backing up
# Deploy high-priority-only workers

# Environment
WORKER_MODE=specialized
WORKER_PRIORITIES=high
WORKER_CONCURRENCY=20
```

**3. Auto-scaling (Kubernetes):**
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: worker-autoscaler
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: worker-default
  minReplicas: 5
  maxReplicas: 50
  metrics:
  - type: Pods
    pods:
      metric:
        name: queue_depth
      target:
        type: AverageValue
        averageValue: "100"
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 0
      policies:
      - type: Percent
        value: 100
        periodSeconds: 15
      - type: Pods
        value: 4
        periodSeconds: 15
      selectPolicy: Max
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 50
        periodSeconds: 60
```

### Vertical Scaling (Redis)

**When to Scale:**
- Memory usage > 75%
- CPU usage > 70%
- Slow query log showing latency

**How to Scale:**
```bash
# AWS ElastiCache
aws elasticache modify-cache-cluster \
  --cache-cluster-id bananas-redis \
  --cache-node-type cache.m5.xlarge

# Manually
# 1. Add larger Redis replica
# 2. Promote replica to master
# 3. Decommission old master
```

---

## Troubleshooting

### Common Issues

**1. High Queue Depth**

**Symptoms:**
- Queue depth growing continuously
- Jobs taking long to process

**Diagnosis:**
```bash
# Check queue depths
redis-cli LLEN bananas:route:default:queue:high
redis-cli LLEN bananas:route:default:queue:normal

# Check worker count
kubectl get pods -l app=worker -n bananas
```

**Solutions:**
- Scale workers horizontally
- Increase worker concurrency
- Check for slow handlers
- Review job distribution

**2. Jobs Stuck in Processing**

**Symptoms:**
- Processing queue growing
- Jobs not completing

**Diagnosis:**
```bash
# Check processing queue
redis-cli LLEN bananas:queue:processing

# Check worker logs
kubectl logs -l app=worker -n bananas --tail=100
```

**Solutions:**
- Restart stuck workers
- Increase job timeout
- Fix handler panics
- Check Redis connectivity

**3. Dead Letter Queue Growing**

**Symptoms:**
- Many jobs in DLQ
- High failure rate

**Diagnosis:**
```bash
# Check DLQ
redis-cli LLEN bananas:queue:dead

# Get failed jobs
redis-cli LRANGE bananas:queue:dead 0 10
```

**Solutions:**
- Review job error messages
- Fix handler bugs
- Increase max retries
- Add better error handling

**4. Redis Memory Issues**

**Symptoms:**
- Redis memory usage high
- Evictions occurring

**Diagnosis:**
```bash
# Check memory
redis-cli INFO memory

# Check key counts
redis-cli DBSIZE
```

**Solutions:**
- Adjust TTLs (reduce retention)
- Scale Redis vertically
- Clean up old jobs
- Use Redis Cluster

---

## Disaster Recovery

### Backup Strategy

**Redis Backup:**
```bash
# RDB backup (snapshot)
redis-cli --rdb /backup/dump.rdb

# AOF backup
cp /var/lib/redis/appendonly.aof /backup/appendonly.aof

# Automated daily backups
0 2 * * * /usr/bin/redis-cli --rdb /backup/redis-$(date +\%Y\%m\%d).rdb
```

**S3 Backup:**
```bash
#!/bin/bash
# Backup to S3
DATE=$(date +%Y%m%d-%H%M%S)
redis-cli --rdb /tmp/dump-$DATE.rdb
gzip /tmp/dump-$DATE.rdb
aws s3 cp /tmp/dump-$DATE.rdb.gz s3://bananas-backups/redis/
rm /tmp/dump-$DATE.rdb.gz
```

### Recovery Procedures

**1. Redis Failure:**
```bash
# Promote replica to master
redis-cli -h replica SLAVEOF NO ONE

# Update connection strings
kubectl set env deployment/worker-default REDIS_URL=redis://replica:6379

# Rebuild original master and add as replica
redis-cli -h new-replica SLAVEOF master-ip 6379
```

**2. Restore from Backup:**
```bash
# Stop Redis
systemctl stop redis

# Restore RDB
cp /backup/dump.rdb /var/lib/redis/dump.rdb
chown redis:redis /var/lib/redis/dump.rdb

# Start Redis
systemctl start redis
```

**3. Complete Cluster Failure:**
```bash
# 1. Restore Redis from backup
# 2. Restart scheduler (will resume cron jobs)
# 3. Restart workers (will resume processing)
# 4. Jobs in processing queue will retry (exponential backoff)
```

---

## Related Documentation

- [Architecture Overview](./ARCHITECTURE.md)
- [API Reference](./API_REFERENCE.md)
- [Performance Tuning](./PERFORMANCE.md)
- [Troubleshooting Guide](./TROUBLESHOOTING.md)

---

**Next:** [Contributing Guide](../CONTRIBUTING.md) | [Examples](../examples/)

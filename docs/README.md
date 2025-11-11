# Bananas Documentation

**Last Updated:** 2025-11-11

Welcome to the Bananas distributed task queue documentation. This guide will help you find the right documentation for your needs.

## Quick Start

- **New to Bananas?** Start with the [Main README](../README.md)
- **Ready to integrate?** See the [Integration Guide](../INTEGRATION_GUIDE.md)
- **Deploying to production?** Check the [Deployment Guide](DEPLOYMENT.md)

## Documentation Index

### Getting Started

| Document | Description |
|----------|-------------|
| [README](../README.md) | Project overview, features, and quick start |
| [Integration Guide](../INTEGRATION_GUIDE.md) | Comprehensive integration patterns and examples |
| [Contributing](../CONTRIBUTING.md) | Developer contribution guide and workflow |

### Core Concepts

| Document | Description |
|----------|-------------|
| [Architecture](ARCHITECTURE.md) | System architecture, components, and design decisions |
| [API Reference](API_REFERENCE.md) | Complete API documentation for all components |
| [Task Routing](TASK_ROUTING_USAGE.md) | Route jobs to specialized worker pools |
| [Periodic Tasks](PERIODIC_TASKS.md) | Cron-based scheduled jobs |

### Operations & Deployment

| Document | Description |
|----------|-------------|
| [Deployment Guide](DEPLOYMENT.md) | Production deployment patterns and best practices |
| [Monitoring](MONITORING.md) | Metrics, logging, and observability (coming soon) |
| [Troubleshooting](TROUBLESHOOTING.md) | Common issues and solutions |

### Performance & Optimization

| Document | Description |
|----------|-------------|
| [Performance](PERFORMANCE.md) | Benchmark results and performance characteristics |
| [Profiling](PROFILING.md) | How to profile and optimize your workers |

### Design Documents

| Document | Description |
|----------|-------------|
| [Worker Architecture Design](WORKER_ARCHITECTURE_DESIGN.md) | Worker pool design and concurrency model |
| [Result Backend Design](RESULT_BACKEND_DESIGN.md) | Synchronous job execution design |
| [Periodic Tasks Design](PERIODIC_TASKS_DESIGN.md) | Cron scheduler architecture |
| [Multi-Tier Workers](MULTI_TIER_WORKERS.md) | Worker deployment modes and patterns |

### Advanced Topics

| Document | Description |
|----------|-------------|
| [Logging](LOGGING.md) | Structured logging and log management |
| [Protocol Buffers](PROTOBUF.md) | Protobuf serialization (if enabled) |
| [Performance Optimizations](PERFORMANCE_OPTIMIZATIONS.md) | Detailed optimization techniques |

## Documentation by Role

### For Developers

1. [Integration Guide](../INTEGRATION_GUIDE.md) - How to integrate Bananas into your application
2. [API Reference](API_REFERENCE.md) - Complete API documentation
3. [Task Routing](TASK_ROUTING_USAGE.md) - Route jobs to specialized workers
4. [Periodic Tasks](PERIODIC_TASKS.md) - Schedule recurring jobs
5. [Contributing](../CONTRIBUTING.md) - How to contribute to Bananas

### For Operations Engineers

1. [Deployment Guide](DEPLOYMENT.md) - Deploy Bananas to production
2. [Architecture](ARCHITECTURE.md) - Understand system architecture
3. [Monitoring](MONITORING.md) - Set up monitoring and alerting (coming soon)
4. [Troubleshooting](TROUBLESHOOTING.md) - Diagnose and fix issues
5. [Performance](PERFORMANCE.md) - Performance characteristics and tuning

### For Architects

1. [Architecture](ARCHITECTURE.md) - System architecture and design decisions
2. [Worker Architecture Design](WORKER_ARCHITECTURE_DESIGN.md) - Worker pool design
3. [Result Backend Design](RESULT_BACKEND_DESIGN.md) - Synchronous job execution
4. [Periodic Tasks Design](PERIODIC_TASKS_DESIGN.md) - Cron scheduler design
5. [Performance](PERFORMANCE.md) - Performance benchmarks

## Common Tasks

### Submitting Jobs

```go
import (
    "github.com/muaviaUsmani/bananas/pkg/client"
    "github.com/muaviaUsmani/bananas/internal/job"
)

c, _ := client.NewClient("redis://localhost:6379")
defer c.Close()

// Basic job submission
jobID, err := c.SubmitJob(
    "send_email",
    map[string]string{"to": "user@example.com"},
    job.PriorityHigh,
)

// Job with routing key
jobID, err = c.SubmitJobWithRoute(
    "process_image",
    payload,
    job.PriorityNormal,
    "gpu",  // routing key
)

// Wait for result
result, err := c.SubmitAndWait(
    ctx,
    "generate_report",
    payload,
    job.PriorityNormal,
    5*time.Minute,
)
```

See [Integration Guide](../INTEGRATION_GUIDE.md#client-sdk-guide) for details.

### Creating Workers

```go
import (
    "github.com/muaviaUsmani/bananas/internal/worker"
    "github.com/muaviaUsmani/bananas/internal/queue"
)

// Create registry and register handlers
registry := worker.NewRegistry()
registry.Register("send_email", handleSendEmail)
registry.Register("process_image", handleProcessImage)

// Create executor and pool
executor := worker.NewExecutor(registry, redisQueue, 10)
pool := worker.NewPool(executor, redisQueue, 10, 5*time.Minute)

// Start processing
pool.Start(ctx)
```

See [Integration Guide](../INTEGRATION_GUIDE.md#creating-job-handlers) for details.

### Scheduling Periodic Tasks

```go
import "github.com/muaviaUsmani/bananas/internal/scheduler"

cronScheduler := scheduler.NewCronScheduler(redisQueue)

cronScheduler.Schedule(scheduler.Schedule{
    ID:       "daily-report",
    Name:     "generate_report",
    Cron:     "0 2 * * *",  // Daily at 2 AM
    Payload:  json.RawMessage(`{"type": "sales"}`),
    Priority: job.PriorityNormal,
})

cronScheduler.Start(ctx)
```

See [Periodic Tasks](PERIODIC_TASKS.md) for details.

### Using Task Routing

```go
// Submit GPU job to GPU workers
c.SubmitJobWithRoute(
    "process_video",
    payload,
    job.PriorityHigh,
    "gpu",
)

// Configure GPU worker
// WORKER_ROUTING_KEYS=gpu ./worker

// Configure multi-key worker (GPU + default)
// WORKER_ROUTING_KEYS=gpu,default ./worker
```

See [Task Routing Usage Guide](TASK_ROUTING_USAGE.md) for details.

## External Resources

- **GitHub Repository**: https://github.com/muaviaUsmani/bananas
- **Issue Tracker**: https://github.com/muaviaUsmani/bananas/issues
- **Discussions**: https://github.com/muaviaUsmani/bananas/discussions
- **Cron Expression Tester**: https://crontab.guru/
- **IANA Time Zones**: https://en.wikipedia.org/wiki/List_of_tz_database_time_zones

## Feature Parity with Celery

Bananas provides 90% feature parity with Celery:

| Feature | Celery | Bananas | Status |
|---------|--------|---------|--------|
| Priority queues | ✅ | ✅ | Complete |
| Task routing | ✅ | ✅ | Complete |
| Result backend | ✅ | ✅ | Complete |
| Periodic tasks (Beat) | ✅ | ✅ | Complete |
| Retry with backoff | ✅ | ✅ | Complete |
| Dead letter queue | ✅ | ✅ | Complete |
| Worker pools | ✅ | ✅ | Complete |
| Task chains | ✅ | ⏳ | Planned |
| Task groups | ✅ | ⏳ | Planned |

## Contributing to Documentation

Found a typo or want to improve the documentation? See our [Contributing Guide](../CONTRIBUTING.md#documentation).

## License

Bananas is released under the MIT License. See [LICENSE](../LICENSE) for details.

---

**Need help?** Check the [Troubleshooting Guide](TROUBLESHOOTING.md) or open an issue on [GitHub](https://github.com/muaviaUsmani/bananas/issues).

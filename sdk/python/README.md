# Bananas Python Client

Python client library for the Bananas distributed task queue system.

## Installation

```bash
pip install bananas-client
```

Or install from source:

```bash
cd sdk/python
pip install -e .
```

## Quick Start

```python
from bananas import Client, JobPriority
from datetime import timedelta

# Create client
client = Client("redis://localhost:6379/0")

# Submit a job
job_id = client.submit_job(
    "send_email",
    {"to": "user@example.com", "subject": "Hello"},
    JobPriority.HIGH,
    description="Welcome email to new user"
)

print(f"Job submitted: {job_id}")

# Get job status
job = client.get_job(job_id)
print(f"Job status: {job.status}")

# Get result (when available)
result = client.get_result(job_id)
if result and result.is_success():
    print(f"Result: {result.result}")

# Or use submit_and_wait for RPC-style execution
result = client.submit_and_wait(
    "generate_report",
    {"report_type": "sales"},
    JobPriority.NORMAL,
    timeout=timedelta(minutes=5)
)

if result and result.is_success():
    print(f"Report: {result.result}")

client.close()
```

## Context Manager Usage

```python
from bananas import Client, JobPriority

with Client("redis://localhost:6379/0") as client:
    result = client.submit_and_wait(
        "process_data",
        {"data": [1, 2, 3]},
        JobPriority.HIGH
    )
    print(result.result)
```

## Scheduled Jobs

```python
from datetime import datetime, timedelta, timezone
from bananas import Client, JobPriority

client = Client("redis://localhost:6379/0")

# Schedule job for 1 hour from now
scheduled_time = datetime.now(timezone.utc) + timedelta(hours=1)
job_id = client.submit_job_scheduled(
    "send_reminder",
    {"user_id": 123},
    JobPriority.NORMAL,
    scheduled_time
)

client.close()
```

## Task Routing

```python
from bananas import Client, JobPriority

client = Client("redis://localhost:6379/0")

# Route job to GPU workers
job_id = client.submit_job_with_route(
    "process_video",
    {"video_url": "https://..."},
    JobPriority.HIGH,
    "gpu"  # routing key
)

client.close()
```

## API Reference

### Client

**`Client(redis_url, success_ttl=timedelta(hours=1), failure_ttl=timedelta(hours=24))`**

Create a new client instance.

- `redis_url`: Redis connection URL (e.g., `redis://localhost:6379/0`)
- `success_ttl`: TTL for successful results (default: 1 hour)
- `failure_ttl`: TTL for failed results (default: 24 hours)

**`submit_job(name, payload, priority, description="", routing_key="")`**

Submit a new job to the queue.

- `name`: Job name (identifies the handler)
- `payload`: Job data as a dictionary
- `priority`: JobPriority enum value (HIGH, NORMAL, LOW)
- `description`: Optional description
- `routing_key`: Optional routing key for task routing
- Returns: Job ID (UUID string)

**`submit_job_scheduled(name, payload, priority, scheduled_for, description="", routing_key="")`**

Submit a job scheduled for future execution.

- `scheduled_for`: datetime when the job should execute
- Other parameters same as `submit_job()`
- Returns: Job ID (UUID string)

**`get_job(job_id)`**

Retrieve a job by its ID.

- Returns: Job instance or None

**`get_result(job_id)`**

Retrieve the result of a completed job.

- Returns: JobResult instance or None

**`submit_and_wait(name, payload, priority, timeout=timedelta(minutes=5), description="", routing_key="")`**

Submit a job and wait for its result (RPC-style).

- `timeout`: Maximum time to wait
- Returns: JobResult instance or None

**`close()`**

Close all Redis connections.

### Enums

**`JobPriority`**

- `JobPriority.HIGH`: High priority
- `JobPriority.NORMAL`: Normal priority
- `JobPriority.LOW`: Low priority

**`JobStatus`**

- `JobStatus.PENDING`: Waiting to be processed
- `JobStatus.PROCESSING`: Currently being processed
- `JobStatus.COMPLETED`: Successfully completed
- `JobStatus.FAILED`: Failed (no more retries)
- `JobStatus.SCHEDULED`: Scheduled for future execution

## Development

### Running Tests

```bash
# Install dev dependencies
pip install -e ".[dev]"

# Run tests
pytest

# Run with coverage
pytest --cov=bananas --cov-report=html

# Run specific test file
pytest tests/test_client.py
```

### Code Quality

```bash
# Format code
black bananas tests

# Sort imports
isort bananas tests

# Type checking
mypy bananas

# Linting
pylint bananas
```

## Requirements

- Python 3.8+
- Redis 5.0+

## License

MIT

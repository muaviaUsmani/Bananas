# Phase 4: Multi-Language SDKs - Implementation Plan

## Overview

This document provides a comprehensive implementation plan for creating Python and TypeScript client SDKs for the Bananas distributed task queue system. Both SDKs will maintain API compatibility with the Go client while following language-specific best practices.

## Architecture Overview

### Core Components

Both SDKs will implement the following core components:

1. **Client**: Main entry point for submitting and managing jobs
2. **Job Models**: Data structures for jobs and job results
3. **Queue Interface**: Redis queue operations
4. **Result Backend**: Result storage and retrieval with pub/sub support
5. **Type Definitions**: Enums for job status and priority
6. **Error Handling**: Custom exceptions/errors for various failure modes

### Redis Integration Strategy

Both SDKs will interact directly with Redis using the same key patterns and data structures as the Go implementation:

**Key Patterns:**
```
bananas:job:{jobID}              # Job data (JSON string)
bananas:queue:high               # High priority queue (list)
bananas:queue:normal             # Normal priority queue (list)
bananas:queue:low                # Low priority queue (list)
bananas:queue:processing         # Currently processing jobs (list)
bananas:queue:dead               # Dead letter queue (list)
bananas:queue:scheduled          # Scheduled jobs (sorted set, score=timestamp)
bananas:result:{jobID}           # Job result (hash)
bananas:result:notify:{jobID}    # Result notification (pubsub channel)
```

**Job Serialization Format:**
```json
{
  "id": "uuid-v4-string",
  "name": "job_name",
  "description": "optional description",
  "payload": {"arbitrary": "json"},
  "status": "pending|processing|completed|failed|scheduled",
  "priority": "high|normal|low",
  "created_at": "2025-01-15T10:30:00Z",
  "updated_at": "2025-01-15T10:30:05Z",
  "scheduled_for": "2025-01-15T12:00:00Z",
  "attempts": 0,
  "max_retries": 3,
  "error": ""
}
```

**Result Storage Format (Redis Hash):**
```
status: "completed" or "failed"
completed_at: "2025-01-15T10:30:05Z" (RFC3339)
duration_ms: "1234" (milliseconds as string)
result: "{\"key\": \"value\"}" (JSON string, only if successful)
error: "error message" (only if failed)
```

---

## Task 4.1: Python SDK

### Directory Structure

```
sdk/python/
├── bananas/                    # Main package
│   ├── __init__.py            # Package initialization, exports
│   ├── client.py              # Main Client class
│   ├── models.py              # Job, JobResult, enums
│   ├── queue.py               # RedisQueue class
│   ├── result_backend.py      # RedisResultBackend class
│   ├── exceptions.py          # Custom exceptions
│   └── py.typed               # PEP 561 marker for type checking
├── tests/                     # Test suite
│   ├── __init__.py
│   ├── conftest.py            # Pytest fixtures (Redis mock, etc.)
│   ├── test_client.py         # Client tests
│   ├── test_models.py         # Model tests
│   ├── test_queue.py          # Queue tests
│   ├── test_result_backend.py # Result backend tests
│   └── integration/           # Integration tests
│       ├── __init__.py
│       └── test_end_to_end.py # Full workflow tests
├── examples/                  # Example usage
│   ├── basic_usage.py
│   ├── scheduled_jobs.py
│   ├── result_backend.py
│   └── submit_and_wait.py
├── docs/                      # Documentation
│   ├── conf.py               # Sphinx configuration
│   ├── index.rst             # Documentation index
│   ├── quickstart.rst        # Quick start guide
│   ├── api.rst               # API reference
│   └── examples.rst          # Examples
├── pyproject.toml            # Modern Python packaging
├── setup.py                  # Backward compatibility
├── README.md                 # Package README
├── CHANGELOG.md              # Version history
├── LICENSE                   # License file
├── .gitignore                # Git ignore
└── tox.ini                   # Tox configuration for multi-version testing
```

### Dependencies

**Production Dependencies:**
```toml
[project]
dependencies = [
    "redis>=5.0.0",           # Redis client with async support
    "typing-extensions>=4.0",  # Backport of typing features
]

[project.optional-dependencies]
dev = [
    "pytest>=7.0",
    "pytest-asyncio>=0.21",
    "pytest-cov>=4.0",
    "fakeredis>=2.0",          # Mock Redis for testing
    "black>=23.0",             # Code formatting
    "isort>=5.12",             # Import sorting
    "mypy>=1.0",               # Static type checking
    "pylint>=2.17",            # Linting
    "sphinx>=6.0",             # Documentation
    "sphinx-rtd-theme>=1.2",   # Read the Docs theme
]
```

### Class and Method Design

#### 1. `bananas/models.py`

```python
from enum import Enum
from typing import Optional, Any, Dict
from datetime import datetime
import json
from uuid import uuid4

class JobStatus(str, Enum):
    """Job status enumeration"""
    PENDING = "pending"
    PROCESSING = "processing"
    COMPLETED = "completed"
    FAILED = "failed"
    SCHEDULED = "scheduled"

class JobPriority(str, Enum):
    """Job priority enumeration"""
    HIGH = "high"
    NORMAL = "normal"
    LOW = "low"

class Job:
    """Represents a task queue job"""
    
    def __init__(
        self,
        id: str,
        name: str,
        payload: Dict[str, Any],
        status: JobStatus,
        priority: JobPriority,
        created_at: datetime,
        updated_at: datetime,
        description: str = "",
        scheduled_for: Optional[datetime] = None,
        attempts: int = 0,
        max_retries: int = 3,
        error: str = "",
    ):
        self.id = id
        self.name = name
        self.description = description
        self.payload = payload
        self.status = status
        self.priority = priority
        self.created_at = created_at
        self.updated_at = updated_at
        self.scheduled_for = scheduled_for
        self.attempts = attempts
        self.max_retries = max_retries
        self.error = error
    
    @classmethod
    def create(
        cls,
        name: str,
        payload: Dict[str, Any],
        priority: JobPriority,
        description: str = "",
    ) -> "Job":
        """Create a new job with default values"""
        now = datetime.utcnow()
        return cls(
            id=str(uuid4()),
            name=name,
            payload=payload,
            status=JobStatus.PENDING,
            priority=priority,
            created_at=now,
            updated_at=now,
            description=description,
            max_retries=3,
        )
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert job to dictionary for JSON serialization"""
        data = {
            "id": self.id,
            "name": self.name,
            "payload": self.payload,
            "status": self.status.value,
            "priority": self.priority.value,
            "created_at": self.created_at.isoformat() + "Z",
            "updated_at": self.updated_at.isoformat() + "Z",
            "attempts": self.attempts,
            "max_retries": self.max_retries,
        }
        if self.description:
            data["description"] = self.description
        if self.scheduled_for:
            data["scheduled_for"] = self.scheduled_for.isoformat() + "Z"
        if self.error:
            data["error"] = self.error
        return data
    
    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Job":
        """Create job from dictionary (from Redis)"""
        # Parse timestamps
        created_at = datetime.fromisoformat(data["created_at"].replace("Z", "+00:00"))
        updated_at = datetime.fromisoformat(data["updated_at"].replace("Z", "+00:00"))
        scheduled_for = None
        if "scheduled_for" in data and data["scheduled_for"]:
            scheduled_for = datetime.fromisoformat(
                data["scheduled_for"].replace("Z", "+00:00")
            )
        
        return cls(
            id=data["id"],
            name=data["name"],
            payload=data["payload"],
            status=JobStatus(data["status"]),
            priority=JobPriority(data["priority"]),
            created_at=created_at,
            updated_at=updated_at,
            description=data.get("description", ""),
            scheduled_for=scheduled_for,
            attempts=data.get("attempts", 0),
            max_retries=data.get("max_retries", 3),
            error=data.get("error", ""),
        )
    
    def to_json(self) -> str:
        """Serialize job to JSON string"""
        return json.dumps(self.to_dict())
    
    @classmethod
    def from_json(cls, json_str: str) -> "Job":
        """Deserialize job from JSON string"""
        return cls.from_dict(json.loads(json_str))


class JobResult:
    """Represents the result of a completed job"""
    
    def __init__(
        self,
        job_id: str,
        status: JobStatus,
        completed_at: datetime,
        duration_ms: int,
        result: Optional[Dict[str, Any]] = None,
        error: str = "",
    ):
        self.job_id = job_id
        self.status = status
        self.completed_at = completed_at
        self.duration_ms = duration_ms
        self.result = result
        self.error = error
    
    @property
    def is_success(self) -> bool:
        """Returns True if job completed successfully"""
        return self.status == JobStatus.COMPLETED
    
    @property
    def is_failed(self) -> bool:
        """Returns True if job failed"""
        return self.status == JobStatus.FAILED
    
    def get_result(self, result_type: type = dict) -> Any:
        """
        Get the job result, optionally converting to a specific type.
        
        Raises:
            JobFailedError: If the job failed
            ValueError: If result is None
        """
        if self.is_failed:
            raise JobFailedError(self.error)
        if self.result is None:
            raise ValueError("No result data available")
        return self.result
```

#### 2. `bananas/exceptions.py`

```python
class BananasError(Exception):
    """Base exception for all Bananas errors"""
    pass

class ConnectionError(BananasError):
    """Failed to connect to Redis"""
    pass

class JobNotFoundError(BananasError):
    """Job not found in queue"""
    pass

class JobFailedError(BananasError):
    """Job execution failed"""
    pass

class TimeoutError(BananasError):
    """Operation timed out"""
    pass

class SerializationError(BananasError):
    """Failed to serialize/deserialize data"""
    pass
```

#### 3. `bananas/queue.py`

```python
from typing import Optional
import redis
from .models import Job

class RedisQueue:
    """Redis-backed job queue"""
    
    def __init__(self, redis_client: redis.Redis, key_prefix: str = "bananas:"):
        self.client = redis_client
        self.key_prefix = key_prefix
        # Pre-compute keys
        self.queue_high = f"{key_prefix}queue:high"
        self.queue_normal = f"{key_prefix}queue:normal"
        self.queue_low = f"{key_prefix}queue:low"
        self.processing = f"{key_prefix}queue:processing"
        self.dead_letter = f"{key_prefix}queue:dead"
        self.scheduled_set = f"{key_prefix}queue:scheduled"
    
    def _job_key(self, job_id: str) -> str:
        """Generate Redis key for job data"""
        return f"{self.key_prefix}job:{job_id}"
    
    def _queue_key(self, priority: JobPriority) -> str:
        """Get queue key for priority"""
        if priority == JobPriority.HIGH:
            return self.queue_high
        elif priority == JobPriority.LOW:
            return self.queue_low
        return self.queue_normal
    
    def enqueue(self, job: Job) -> None:
        """Enqueue a job to the appropriate priority queue"""
        # Store job data
        job_key = self._job_key(job.id)
        self.client.set(job_key, job.to_json())
        
        # Add to priority queue
        queue_key = self._queue_key(job.priority)
        self.client.lpush(queue_key, job.id)
    
    def get_job(self, job_id: str) -> Optional[Job]:
        """Retrieve a job by ID"""
        job_key = self._job_key(job_id)
        job_data = self.client.get(job_key)
        if job_data is None:
            return None
        return Job.from_json(job_data.decode('utf-8'))
    
    def close(self) -> None:
        """Close Redis connection"""
        self.client.close()
```

#### 4. `bananas/result_backend.py`

```python
from typing import Optional
import redis
import json
import time
from datetime import datetime, timedelta
from .models import JobResult, JobStatus
from .exceptions import TimeoutError

class RedisResultBackend:
    """Redis-backed result storage with pub/sub support"""
    
    def __init__(
        self,
        redis_client: redis.Redis,
        success_ttl: timedelta = timedelta(hours=1),
        failure_ttl: timedelta = timedelta(hours=24),
    ):
        self.client = redis_client
        self.success_ttl = success_ttl
        self.failure_ttl = failure_ttl
    
    def _result_key(self, job_id: str) -> str:
        """Generate Redis key for result"""
        return f"bananas:result:{job_id}"
    
    def _notify_channel(self, job_id: str) -> str:
        """Generate pub/sub channel for result notifications"""
        return f"bananas:result:notify:{job_id}"
    
    def store_result(self, result: JobResult) -> None:
        """Store job result in Redis"""
        key = self._result_key(result.job_id)
        channel = self._notify_channel(result.job_id)
        
        # Prepare hash data
        data = {
            "status": result.status.value,
            "completed_at": result.completed_at.isoformat() + "Z",
            "duration_ms": str(result.duration_ms),
        }
        
        if result.is_success and result.result:
            data["result"] = json.dumps(result.result)
        
        if result.is_failed and result.error:
            data["error"] = result.error
        
        # Determine TTL
        ttl = self.success_ttl if result.is_success else self.failure_ttl
        
        # Store in Redis with pipeline
        pipe = self.client.pipeline()
        pipe.hset(key, mapping=data)
        pipe.expire(key, int(ttl.total_seconds()))
        pipe.publish(channel, "ready")
        pipe.execute()
    
    def get_result(self, job_id: str) -> Optional[JobResult]:
        """Retrieve job result from Redis"""
        key = self._result_key(job_id)
        data = self.client.hgetall(key)
        
        if not data:
            return None
        
        # Decode bytes to strings
        data = {k.decode('utf-8'): v.decode('utf-8') for k, v in data.items()}
        
        # Parse result
        status = JobStatus(data["status"])
        completed_at = datetime.fromisoformat(data["completed_at"].replace("Z", "+00:00"))
        duration_ms = int(data["duration_ms"])
        
        result = None
        if "result" in data:
            result = json.loads(data["result"])
        
        error = data.get("error", "")
        
        return JobResult(
            job_id=job_id,
            status=status,
            completed_at=completed_at,
            duration_ms=duration_ms,
            result=result,
            error=error,
        )
    
    def wait_for_result(
        self, job_id: str, timeout: timedelta
    ) -> Optional[JobResult]:
        """
        Wait for a job result using pub/sub.
        
        Returns:
            JobResult if available within timeout, None if timeout reached
        
        Raises:
            TimeoutError: If waiting fails
        """
        # First check if result already exists
        result = self.get_result(job_id)
        if result:
            return result
        
        # Subscribe to notification channel
        channel = self._notify_channel(job_id)
        pubsub = self.client.pubsub()
        pubsub.subscribe(channel)
        
        try:
            # Wait for notification with timeout
            timeout_secs = timeout.total_seconds()
            start_time = time.time()
            
            for message in pubsub.listen():
                if message["type"] == "message" and message["data"] == b"ready":
                    # Notification received, fetch result
                    result = self.get_result(job_id)
                    return result
                
                # Check timeout
                if time.time() - start_time > timeout_secs:
                    # One final check before giving up
                    result = self.get_result(job_id)
                    return result
        finally:
            pubsub.unsubscribe(channel)
            pubsub.close()
    
    def delete_result(self, job_id: str) -> None:
        """Delete a job result"""
        key = self._result_key(job_id)
        self.client.delete(key)
    
    def close(self) -> None:
        """Close Redis connection"""
        self.client.close()
```

#### 5. `bananas/client.py`

```python
from typing import Optional, Dict, Any
from datetime import datetime, timedelta
import redis
from .models import Job, JobResult, JobPriority, JobStatus
from .queue import RedisQueue
from .result_backend import RedisResultBackend
from .exceptions import ConnectionError, SerializationError

class Client:
    """
    Bananas task queue client.
    
    Provides a simple API for submitting and managing jobs.
    
    Example:
        >>> client = Client("redis://localhost:6379")
        >>> job_id = client.submit_job(
        ...     "send_email",
        ...     {"to": "user@example.com", "subject": "Hello"},
        ...     JobPriority.NORMAL
        ... )
        >>> result = client.get_result(job_id)
    """
    
    def __init__(
        self,
        redis_url: str,
        success_ttl: timedelta = timedelta(hours=1),
        failure_ttl: timedelta = timedelta(hours=24),
    ):
        """
        Create a new Bananas client.
        
        Args:
            redis_url: Redis connection URL (e.g., "redis://localhost:6379")
            success_ttl: TTL for successful job results (default: 1 hour)
            failure_ttl: TTL for failed job results (default: 24 hours)
        
        Raises:
            ConnectionError: If unable to connect to Redis
        """
        try:
            # Parse Redis URL and create client
            self._redis_client = redis.from_url(
                redis_url,
                decode_responses=False,  # We handle decoding manually
            )
            # Test connection
            self._redis_client.ping()
        except Exception as e:
            raise ConnectionError(f"Failed to connect to Redis: {e}") from e
        
        # Create queue and result backend
        self._queue = RedisQueue(self._redis_client)
        self._result_backend = RedisResultBackend(
            self._redis_client,
            success_ttl=success_ttl,
            failure_ttl=failure_ttl,
        )
    
    def submit_job(
        self,
        name: str,
        payload: Dict[str, Any],
        priority: JobPriority = JobPriority.NORMAL,
        description: str = "",
    ) -> str:
        """
        Submit a new job for processing.
        
        Args:
            name: Job type/name (e.g., "send_email", "process_image")
            payload: Job payload as a dictionary
            priority: Job priority (default: NORMAL)
            description: Optional human-readable description
        
        Returns:
            str: The job ID
        
        Raises:
            SerializationError: If payload cannot be serialized
        """
        try:
            # Create job
            job = Job.create(
                name=name,
                payload=payload,
                priority=priority,
                description=description,
            )
            
            # Enqueue to Redis
            self._queue.enqueue(job)
            
            return job.id
        except Exception as e:
            raise SerializationError(f"Failed to submit job: {e}") from e
    
    def submit_job_scheduled(
        self,
        name: str,
        payload: Dict[str, Any],
        priority: JobPriority,
        scheduled_for: datetime,
        description: str = "",
    ) -> str:
        """
        Submit a job scheduled for future execution.
        
        NOTE: This is a simplified implementation. The full scheduling logic
        requires enqueuing, dequeuing, and failing the job to move it to the
        scheduled set. For production use, this should use a direct ZADD to
        the scheduled set.
        
        Args:
            name: Job type/name
            payload: Job payload as a dictionary
            priority: Job priority
            scheduled_for: When the job should be executed
            description: Optional description
        
        Returns:
            str: The job ID
        """
        # Create job
        job = Job.create(
            name=name,
            payload=payload,
            priority=priority,
            description=description,
        )
        job.scheduled_for = scheduled_for
        job.status = JobStatus.SCHEDULED
        
        # For now, use simplified approach: directly add to scheduled set
        # In production, this matches the Go implementation's workaround
        self._redis_client.zadd(
            "bananas:queue:scheduled",
            {job.id: int(scheduled_for.timestamp())},
        )
        
        # Store job data
        self._redis_client.set(
            f"bananas:job:{job.id}",
            job.to_json(),
        )
        
        return job.id
    
    def get_job(self, job_id: str) -> Optional[Job]:
        """
        Retrieve a job by its ID.
        
        Args:
            job_id: The job ID
        
        Returns:
            Job if found, None otherwise
        """
        return self._queue.get_job(job_id)
    
    def get_result(self, job_id: str) -> Optional[JobResult]:
        """
        Retrieve the result of a completed job.
        
        Args:
            job_id: The job ID
        
        Returns:
            JobResult if available, None if not yet complete or expired
        """
        return self._result_backend.get_result(job_id)
    
    def submit_and_wait(
        self,
        name: str,
        payload: Dict[str, Any],
        priority: JobPriority = JobPriority.NORMAL,
        timeout: timedelta = timedelta(seconds=30),
    ) -> JobResult:
        """
        Submit a job and wait for its result (RPC-style execution).
        
        Args:
            name: Job type/name
            payload: Job payload as a dictionary
            priority: Job priority (default: NORMAL)
            timeout: How long to wait for result (default: 30 seconds)
        
        Returns:
            JobResult: The job result
        
        Raises:
            TimeoutError: If job doesn't complete within timeout
        """
        # Submit job
        job_id = self.submit_job(name, payload, priority)
        
        # Wait for result
        result = self._result_backend.wait_for_result(job_id, timeout)
        
        if result is None:
            from .exceptions import TimeoutError
            raise TimeoutError(
                f"Job {job_id} did not complete within {timeout.total_seconds()}s"
            )
        
        return result
    
    def close(self) -> None:
        """Close all Redis connections."""
        if self._queue:
            self._queue.close()
        if self._result_backend:
            self._result_backend.close()
    
    def __enter__(self):
        """Context manager entry"""
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        """Context manager exit"""
        self.close()
```

#### 6. `bananas/__init__.py`

```python
"""
Bananas - A distributed task queue system
"""

__version__ = "0.1.0"

from .client import Client
from .models import Job, JobResult, JobStatus, JobPriority
from .exceptions import (
    BananasError,
    ConnectionError,
    JobNotFoundError,
    JobFailedError,
    TimeoutError,
    SerializationError,
)

__all__ = [
    "Client",
    "Job",
    "JobResult",
    "JobStatus",
    "JobPriority",
    "BananasError",
    "ConnectionError",
    "JobNotFoundError",
    "JobFailedError",
    "TimeoutError",
    "SerializationError",
]
```

### Testing Strategy

#### Unit Tests (90%+ coverage target)

1. **test_models.py**
   - Job creation and serialization
   - JobResult creation and methods
   - Enum values
   - Edge cases (null values, missing fields)

2. **test_queue.py**
   - Enqueue operations
   - Job retrieval
   - Key generation
   - Priority queue selection
   - Use fakeredis for mocking

3. **test_result_backend.py**
   - Store and retrieve results
   - TTL configuration
   - Pub/sub notifications
   - Timeout handling
   - Use fakeredis for mocking

4. **test_client.py**
   - All public methods
   - Error handling
   - Connection failures
   - Context manager
   - Use fakeredis for mocking

#### Integration Tests

5. **test_end_to_end.py**
   - Requires real Redis instance (or testcontainers)
   - Full workflow: submit -> process -> retrieve result
   - Scheduled jobs
   - Priority ordering
   - Retry logic
   - Dead letter queue

#### Test Configuration

```python
# tests/conftest.py
import pytest
import fakeredis

@pytest.fixture
def redis_client():
    """Provide a fake Redis client for testing"""
    return fakeredis.FakeRedis()

@pytest.fixture
def client(redis_client):
    """Provide a test client"""
    # Inject fake redis instead of creating real connection
    from bananas.client import Client
    client = Client.__new__(Client)
    client._redis_client = redis_client
    from bananas.queue import RedisQueue
    from bananas.result_backend import RedisResultBackend
    from datetime import timedelta
    client._queue = RedisQueue(redis_client)
    client._result_backend = RedisResultBackend(
        redis_client,
        success_ttl=timedelta(hours=1),
        failure_ttl=timedelta(hours=24),
    )
    yield client
    client.close()
```

### Documentation Strategy

#### Docstrings

- Use Google-style docstrings for all public APIs
- Include type hints in all function signatures
- Provide examples in docstrings

#### Sphinx Documentation

1. **docs/quickstart.rst** - Getting started guide
2. **docs/api.rst** - Complete API reference (auto-generated)
3. **docs/examples.rst** - Code examples
4. **docs/advanced.rst** - Advanced usage (TTL, connection pooling)

#### README.md

```markdown
# Bananas Python SDK

Python client for the Bananas distributed task queue system.

## Installation

```bash
pip install bananas-queue
```

## Quick Start

```python
from bananas import Client, JobPriority

# Create client
client = Client("redis://localhost:6379")

# Submit a job
job_id = client.submit_job(
    name="send_email",
    payload={"to": "user@example.com", "subject": "Hello!"},
    priority=JobPriority.NORMAL
)

print(f"Submitted job: {job_id}")

# Get result
result = client.get_result(job_id)
if result and result.is_success:
    print(f"Job completed: {result.result}")

# Or use RPC-style
result = client.submit_and_wait(
    "send_email",
    {"to": "user@example.com"},
    timeout=timedelta(seconds=30)
)
```

## Features

- ✓ Priority-based job processing (high, normal, low)
- ✓ Scheduled job execution
- ✓ Job result storage with configurable TTL
- ✓ RPC-style synchronous job execution
- ✓ Comprehensive error handling
- ✓ Type hints for IDE support
- ✓ Context manager support
```

### Packaging

#### pyproject.toml

```toml
[build-system]
requires = ["setuptools>=65.0", "wheel"]
build-backend = "setuptools.build_meta"

[project]
name = "bananas-queue"
version = "0.1.0"
description = "Python client for the Bananas distributed task queue"
readme = "README.md"
license = {text = "MIT"}
authors = [
    {name = "Bananas Team", email = "team@bananas.dev"}
]
classifiers = [
    "Development Status :: 4 - Beta",
    "Intended Audience :: Developers",
    "License :: OSI Approved :: MIT License",
    "Programming Language :: Python :: 3",
    "Programming Language :: Python :: 3.8",
    "Programming Language :: Python :: 3.9",
    "Programming Language :: Python :: 3.10",
    "Programming Language :: Python :: 3.11",
    "Programming Language :: Python :: 3.12",
]
requires-python = ">=3.8"
dependencies = [
    "redis>=5.0.0",
    "typing-extensions>=4.0; python_version<'3.10'",
]

[project.optional-dependencies]
dev = [
    "pytest>=7.0",
    "pytest-asyncio>=0.21",
    "pytest-cov>=4.0",
    "fakeredis>=2.0",
    "black>=23.0",
    "isort>=5.12",
    "mypy>=1.0",
    "pylint>=2.17",
]
docs = [
    "sphinx>=6.0",
    "sphinx-rtd-theme>=1.2",
]

[project.urls]
Homepage = "https://github.com/muaviaUsmani/bananas"
Documentation = "https://bananas.readthedocs.io"
Repository = "https://github.com/muaviaUsmani/bananas"

[tool.setuptools.packages.find]
where = ["."]
include = ["bananas*"]

[tool.pytest.ini_options]
testpaths = ["tests"]
python_files = ["test_*.py"]
addopts = "--cov=bananas --cov-report=html --cov-report=term-missing"

[tool.black]
line-length = 88
target-version = ['py38', 'py39', 'py310', 'py311']

[tool.isort]
profile = "black"
line_length = 88

[tool.mypy]
python_version = "3.8"
warn_return_any = true
warn_unused_configs = true
disallow_untyped_defs = true
```

---

## Task 4.2: TypeScript SDK

### Directory Structure

```
sdk/typescript/
├── src/                       # Source code
│   ├── client.ts             # Main Client class
│   ├── models.ts             # Job, JobResult, enums
│   ├── queue.ts              # RedisQueue class
│   ├── resultBackend.ts      # RedisResultBackend class
│   ├── exceptions.ts         # Custom error classes
│   ├── types.ts              # TypeScript type definitions
│   └── index.ts              # Package exports
├── test/                     # Test suite
│   ├── client.test.ts        # Client tests
│   ├── models.test.ts        # Model tests
│   ├── queue.test.ts         # Queue tests
│   ├── resultBackend.test.ts # Result backend tests
│   └── integration/          # Integration tests
│       └── endToEnd.test.ts  # Full workflow tests
├── examples/                 # Example usage
│   ├── basicUsage.ts
│   ├── scheduledJobs.ts
│   ├── resultBackend.ts
│   └── submitAndWait.ts
├── docs/                     # Documentation
│   └── api.md               # API documentation
├── dist/                     # Compiled output (gitignored)
├── package.json             # NPM package configuration
├── tsconfig.json            # TypeScript configuration
├── tsconfig.build.json      # Build-specific TS config
├── jest.config.js           # Jest test configuration
├── .eslintrc.js             # ESLint configuration
├── .prettierrc              # Prettier configuration
├── README.md                # Package README
├── CHANGELOG.md             # Version history
├── LICENSE                  # License file
└── .gitignore               # Git ignore
```

### Dependencies

```json
{
  "name": "@bananas/client",
  "version": "0.1.0",
  "description": "TypeScript client for the Bananas distributed task queue",
  "main": "dist/index.js",
  "types": "dist/index.d.ts",
  "dependencies": {
    "ioredis": "^5.3.0"
  },
  "devDependencies": {
    "@types/node": "^20.0.0",
    "@types/jest": "^29.0.0",
    "@typescript-eslint/eslint-plugin": "^6.0.0",
    "@typescript-eslint/parser": "^6.0.0",
    "eslint": "^8.50.0",
    "eslint-config-prettier": "^9.0.0",
    "eslint-plugin-prettier": "^5.0.0",
    "jest": "^29.0.0",
    "ts-jest": "^29.0.0",
    "ts-node": "^10.9.0",
    "typedoc": "^0.25.0",
    "typescript": "^5.2.0",
    "prettier": "^3.0.0",
    "ioredis-mock": "^8.9.0"
  }
}
```

### Class and Method Design

#### 1. `src/types.ts`

```typescript
/**
 * Job status enumeration
 */
export enum JobStatus {
  Pending = "pending",
  Processing = "processing",
  Completed = "completed",
  Failed = "failed",
  Scheduled = "scheduled",
}

/**
 * Job priority enumeration
 */
export enum JobPriority {
  High = "high",
  Normal = "normal",
  Low = "low",
}

/**
 * Configuration options for the Bananas client
 */
export interface ClientConfig {
  /**
   * Redis connection URL
   * @example "redis://localhost:6379"
   */
  redisUrl: string;

  /**
   * TTL for successful job results (milliseconds)
   * @default 3600000 (1 hour)
   */
  successTTL?: number;

  /**
   * TTL for failed job results (milliseconds)
   * @default 86400000 (24 hours)
   */
  failureTTL?: number;
}
```

#### 2. `src/models.ts`

```typescript
import { v4 as uuidv4 } from "uuid";
import { JobStatus, JobPriority } from "./types";

/**
 * Represents a task queue job
 */
export class Job {
  public id: string;
  public name: string;
  public description: string;
  public payload: Record<string, any>;
  public status: JobStatus;
  public priority: JobPriority;
  public createdAt: Date;
  public updatedAt: Date;
  public scheduledFor?: Date;
  public attempts: number;
  public maxRetries: number;
  public error: string;

  constructor(data: {
    id: string;
    name: string;
    payload: Record<string, any>;
    status: JobStatus;
    priority: JobPriority;
    createdAt: Date;
    updatedAt: Date;
    description?: string;
    scheduledFor?: Date;
    attempts?: number;
    maxRetries?: number;
    error?: string;
  }) {
    this.id = data.id;
    this.name = data.name;
    this.description = data.description || "";
    this.payload = data.payload;
    this.status = data.status;
    this.priority = data.priority;
    this.createdAt = data.createdAt;
    this.updatedAt = data.updatedAt;
    this.scheduledFor = data.scheduledFor;
    this.attempts = data.attempts || 0;
    this.maxRetries = data.maxRetries || 3;
    this.error = data.error || "";
  }

  /**
   * Create a new job with default values
   */
  static create(
    name: string,
    payload: Record<string, any>,
    priority: JobPriority,
    description = ""
  ): Job {
    const now = new Date();
    return new Job({
      id: uuidv4(),
      name,
      payload,
      status: JobStatus.Pending,
      priority,
      createdAt: now,
      updatedAt: now,
      description,
      maxRetries: 3,
      attempts: 0,
    });
  }

  /**
   * Convert job to plain object for serialization
   */
  toJSON(): Record<string, any> {
    const data: Record<string, any> = {
      id: this.id,
      name: this.name,
      payload: this.payload,
      status: this.status,
      priority: this.priority,
      created_at: this.createdAt.toISOString(),
      updated_at: this.updatedAt.toISOString(),
      attempts: this.attempts,
      max_retries: this.maxRetries,
    };

    if (this.description) {
      data.description = this.description;
    }
    if (this.scheduledFor) {
      data.scheduled_for = this.scheduledFor.toISOString();
    }
    if (this.error) {
      data.error = this.error;
    }

    return data;
  }

  /**
   * Create job from plain object (from Redis)
   */
  static fromJSON(data: Record<string, any>): Job {
    return new Job({
      id: data.id,
      name: data.name,
      payload: data.payload,
      status: data.status as JobStatus,
      priority: data.priority as JobPriority,
      createdAt: new Date(data.created_at),
      updatedAt: new Date(data.updated_at),
      description: data.description,
      scheduledFor: data.scheduled_for ? new Date(data.scheduled_for) : undefined,
      attempts: data.attempts,
      maxRetries: data.max_retries,
      error: data.error,
    });
  }
}

/**
 * Represents the result of a completed job
 */
export class JobResult {
  public jobId: string;
  public status: JobStatus;
  public completedAt: Date;
  public durationMs: number;
  public result?: Record<string, any>;
  public error: string;

  constructor(data: {
    jobId: string;
    status: JobStatus;
    completedAt: Date;
    durationMs: number;
    result?: Record<string, any>;
    error?: string;
  }) {
    this.jobId = data.jobId;
    this.status = data.status;
    this.completedAt = data.completedAt;
    this.durationMs = data.durationMs;
    this.result = data.result;
    this.error = data.error || "";
  }

  /**
   * Returns true if job completed successfully
   */
  get isSuccess(): boolean {
    return this.status === JobStatus.Completed;
  }

  /**
   * Returns true if job failed
   */
  get isFailed(): boolean {
    return this.status === JobStatus.Failed;
  }

  /**
   * Get the job result, throwing if failed
   */
  getResult<T = Record<string, any>>(): T {
    if (this.isFailed) {
      throw new Error(`Job failed: ${this.error}`);
    }
    if (!this.result) {
      throw new Error("No result data available");
    }
    return this.result as T;
  }
}
```

#### 3. `src/exceptions.ts`

```typescript
/**
 * Base error class for Bananas errors
 */
export class BananasError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "BananasError";
    Object.setPrototypeOf(this, BananasError.prototype);
  }
}

/**
 * Failed to connect to Redis
 */
export class ConnectionError extends BananasError {
  constructor(message: string) {
    super(message);
    this.name = "ConnectionError";
    Object.setPrototypeOf(this, ConnectionError.prototype);
  }
}

/**
 * Job not found in queue
 */
export class JobNotFoundError extends BananasError {
  constructor(jobId: string) {
    super(`Job not found: ${jobId}`);
    this.name = "JobNotFoundError";
    Object.setPrototypeOf(this, JobNotFoundError.prototype);
  }
}

/**
 * Job execution failed
 */
export class JobFailedError extends BananasError {
  constructor(message: string) {
    super(message);
    this.name = "JobFailedError";
    Object.setPrototypeOf(this, JobFailedError.prototype);
  }
}

/**
 * Operation timed out
 */
export class TimeoutError extends BananasError {
  constructor(message: string) {
    super(message);
    this.name = "TimeoutError";
    Object.setPrototypeOf(this, TimeoutError.prototype);
  }
}

/**
 * Serialization/deserialization error
 */
export class SerializationError extends BananasError {
  constructor(message: string) {
    super(message);
    this.name = "SerializationError";
    Object.setPrototypeOf(this, SerializationError.prototype);
  }
}
```

#### 4. `src/queue.ts`

```typescript
import Redis from "ioredis";
import { Job, JobPriority } from "./models";

/**
 * Redis-backed job queue
 */
export class RedisQueue {
  private client: Redis;
  private keyPrefix: string;
  private queueHigh: string;
  private queueNormal: string;
  private queueLow: string;
  private processing: string;
  private deadLetter: string;
  private scheduledSet: string;

  constructor(client: Redis, keyPrefix = "bananas:") {
    this.client = client;
    this.keyPrefix = keyPrefix;
    this.queueHigh = `${keyPrefix}queue:high`;
    this.queueNormal = `${keyPrefix}queue:normal`;
    this.queueLow = `${keyPrefix}queue:low`;
    this.processing = `${keyPrefix}queue:processing`;
    this.deadLetter = `${keyPrefix}queue:dead`;
    this.scheduledSet = `${keyPrefix}queue:scheduled`;
  }

  /**
   * Generate Redis key for job data
   */
  private jobKey(jobId: string): string {
    return `${this.keyPrefix}job:${jobId}`;
  }

  /**
   * Get queue key for priority
   */
  private queueKey(priority: JobPriority): string {
    switch (priority) {
      case JobPriority.High:
        return this.queueHigh;
      case JobPriority.Low:
        return this.queueLow;
      default:
        return this.queueNormal;
    }
  }

  /**
   * Enqueue a job to the appropriate priority queue
   */
  async enqueue(job: Job): Promise<void> {
    const jobKey = this.jobKey(job.id);
    const queueKey = this.queueKey(job.priority);
    const jobData = JSON.stringify(job.toJSON());

    // Use pipeline for atomicity
    const pipeline = this.client.pipeline();
    pipeline.set(jobKey, jobData);
    pipeline.lpush(queueKey, job.id);
    await pipeline.exec();
  }

  /**
   * Retrieve a job by ID
   */
  async getJob(jobId: string): Promise<Job | null> {
    const jobKey = this.jobKey(jobId);
    const jobData = await this.client.get(jobKey);

    if (!jobData) {
      return null;
    }

    return Job.fromJSON(JSON.parse(jobData));
  }

  /**
   * Close Redis connection
   */
  close(): void {
    this.client.disconnect();
  }
}
```

#### 5. `src/resultBackend.ts`

```typescript
import Redis from "ioredis";
import { JobResult, JobStatus } from "./models";
import { TimeoutError } from "./exceptions";

/**
 * Redis-backed result storage with pub/sub support
 */
export class RedisResultBackend {
  private client: Redis;
  private successTTL: number;
  private failureTTL: number;

  constructor(
    client: Redis,
    successTTL = 3600000, // 1 hour in ms
    failureTTL = 86400000 // 24 hours in ms
  ) {
    this.client = client;
    this.successTTL = successTTL;
    this.failureTTL = failureTTL;
  }

  /**
   * Generate Redis key for result
   */
  private resultKey(jobId: string): string {
    return `bananas:result:${jobId}`;
  }

  /**
   * Generate pub/sub channel for result notifications
   */
  private notifyChannel(jobId: string): string {
    return `bananas:result:notify:${jobId}`;
  }

  /**
   * Store job result in Redis
   */
  async storeResult(result: JobResult): Promise<void> {
    const key = this.resultKey(result.jobId);
    const channel = this.notifyChannel(result.jobId);

    // Prepare hash data
    const data: Record<string, string> = {
      status: result.status,
      completed_at: result.completedAt.toISOString(),
      duration_ms: result.durationMs.toString(),
    };

    if (result.isSuccess && result.result) {
      data.result = JSON.stringify(result.result);
    }

    if (result.isFailed && result.error) {
      data.error = result.error;
    }

    // Determine TTL
    const ttl = result.isSuccess
      ? Math.floor(this.successTTL / 1000)
      : Math.floor(this.failureTTL / 1000);

    // Store in Redis with pipeline
    const pipeline = this.client.pipeline();
    pipeline.hset(key, data);
    pipeline.expire(key, ttl);
    pipeline.publish(channel, "ready");
    await pipeline.exec();
  }

  /**
   * Retrieve job result from Redis
   */
  async getResult(jobId: string): Promise<JobResult | null> {
    const key = this.resultKey(jobId);
    const data = await this.client.hgetall(key);

    if (Object.keys(data).length === 0) {
      return null;
    }

    // Parse result
    const status = data.status as JobStatus;
    const completedAt = new Date(data.completed_at);
    const durationMs = parseInt(data.duration_ms, 10);

    let result: Record<string, any> | undefined;
    if (data.result) {
      result = JSON.parse(data.result);
    }

    const error = data.error || "";

    return new JobResult({
      jobId,
      status,
      completedAt,
      durationMs,
      result,
      error,
    });
  }

  /**
   * Wait for a job result using pub/sub
   */
  async waitForResult(jobId: string, timeoutMs: number): Promise<JobResult | null> {
    // First check if result already exists
    const existing = await this.getResult(jobId);
    if (existing) {
      return existing;
    }

    // Subscribe to notification channel
    const channel = this.notifyChannel(jobId);
    const subscriber = this.client.duplicate();
    await subscriber.subscribe(channel);

    // Create timeout promise
    const timeoutPromise = new Promise<null>((resolve) => {
      setTimeout(() => resolve(null), timeoutMs);
    });

    // Create notification promise
    const notificationPromise = new Promise<JobResult | null>((resolve) => {
      subscriber.on("message", async (ch, msg) => {
        if (ch === channel && msg === "ready") {
          const result = await this.getResult(jobId);
          await subscriber.quit();
          resolve(result);
        }
      });
    });

    // Race between timeout and notification
    const result = await Promise.race([timeoutPromise, notificationPromise]);

    // Cleanup subscriber if still connected
    if (subscriber.status === "ready") {
      await subscriber.quit();
    }

    return result;
  }

  /**
   * Delete a job result
   */
  async deleteResult(jobId: string): Promise<void> {
    const key = this.resultKey(jobId);
    await this.client.del(key);
  }

  /**
   * Close Redis connection
   */
  close(): void {
    this.client.disconnect();
  }
}
```

#### 6. `src/client.ts`

```typescript
import Redis from "ioredis";
import { Job, JobResult, JobPriority, JobStatus } from "./models";
import { RedisQueue } from "./queue";
import { RedisResultBackend } from "./resultBackend";
import { ConnectionError, SerializationError, TimeoutError } from "./exceptions";
import { ClientConfig } from "./types";

/**
 * Bananas task queue client
 *
 * Provides a simple API for submitting and managing jobs.
 *
 * @example
 * ```typescript
 * const client = new Client({ redisUrl: "redis://localhost:6379" });
 *
 * const jobId = await client.submitJob(
 *   "send_email",
 *   { to: "user@example.com", subject: "Hello" },
 *   JobPriority.Normal
 * );
 *
 * const result = await client.getResult(jobId);
 * ```
 */
export class Client {
  private redisClient: Redis;
  private queue: RedisQueue;
  private resultBackend: RedisResultBackend;

  /**
   * Create a new Bananas client
   *
   * @param config - Client configuration
   * @throws {ConnectionError} If unable to connect to Redis
   */
  constructor(config: ClientConfig) {
    try {
      // Create Redis client
      this.redisClient = new Redis(config.redisUrl);

      // Test connection
      this.redisClient.ping().catch((err) => {
        throw new ConnectionError(`Failed to connect to Redis: ${err.message}`);
      });

      // Create queue and result backend
      this.queue = new RedisQueue(this.redisClient);
      this.resultBackend = new RedisResultBackend(
        this.redisClient,
        config.successTTL,
        config.failureTTL
      );
    } catch (err: any) {
      throw new ConnectionError(`Failed to create client: ${err.message}`);
    }
  }

  /**
   * Submit a new job for processing
   *
   * @param name - Job type/name (e.g., "send_email", "process_image")
   * @param payload - Job payload as an object
   * @param priority - Job priority (default: NORMAL)
   * @param description - Optional human-readable description
   * @returns The job ID
   * @throws {SerializationError} If payload cannot be serialized
   */
  async submitJob(
    name: string,
    payload: Record<string, any>,
    priority: JobPriority = JobPriority.Normal,
    description = ""
  ): Promise<string> {
    try {
      // Create job
      const job = Job.create(name, payload, priority, description);

      // Enqueue to Redis
      await this.queue.enqueue(job);

      return job.id;
    } catch (err: any) {
      throw new SerializationError(`Failed to submit job: ${err.message}`);
    }
  }

  /**
   * Submit a job scheduled for future execution
   *
   * @param name - Job type/name
   * @param payload - Job payload as an object
   * @param priority - Job priority
   * @param scheduledFor - When the job should be executed
   * @param description - Optional description
   * @returns The job ID
   */
  async submitJobScheduled(
    name: string,
    payload: Record<string, any>,
    priority: JobPriority,
    scheduledFor: Date,
    description = ""
  ): Promise<string> {
    // Create job
    const job = Job.create(name, payload, priority, description);
    job.scheduledFor = scheduledFor;
    job.status = JobStatus.Scheduled;

    // Add to scheduled set
    await this.redisClient.zadd(
      "bananas:queue:scheduled",
      Math.floor(scheduledFor.getTime() / 1000),
      job.id
    );

    // Store job data
    await this.redisClient.set(
      `bananas:job:${job.id}`,
      JSON.stringify(job.toJSON())
    );

    return job.id;
  }

  /**
   * Retrieve a job by its ID
   *
   * @param jobId - The job ID
   * @returns Job if found, null otherwise
   */
  async getJob(jobId: string): Promise<Job | null> {
    return this.queue.getJob(jobId);
  }

  /**
   * Retrieve the result of a completed job
   *
   * @param jobId - The job ID
   * @returns JobResult if available, null if not yet complete or expired
   */
  async getResult(jobId: string): Promise<JobResult | null> {
    return this.resultBackend.getResult(jobId);
  }

  /**
   * Submit a job and wait for its result (RPC-style execution)
   *
   * @param name - Job type/name
   * @param payload - Job payload as an object
   * @param priority - Job priority (default: NORMAL)
   * @param timeoutMs - How long to wait for result in milliseconds (default: 30000)
   * @returns The job result
   * @throws {TimeoutError} If job doesn't complete within timeout
   */
  async submitAndWait(
    name: string,
    payload: Record<string, any>,
    priority: JobPriority = JobPriority.Normal,
    timeoutMs = 30000
  ): Promise<JobResult> {
    // Submit job
    const jobId = await this.submitJob(name, payload, priority);

    // Wait for result
    const result = await this.resultBackend.waitForResult(jobId, timeoutMs);

    if (!result) {
      throw new TimeoutError(
        `Job ${jobId} did not complete within ${timeoutMs}ms`
      );
    }

    return result;
  }

  /**
   * Close all Redis connections
   */
  close(): void {
    this.queue.close();
    this.resultBackend.close();
    this.redisClient.disconnect();
  }
}
```

#### 7. `src/index.ts`

```typescript
/**
 * @packageDocumentation
 * Bananas - A distributed task queue system
 */

export { Client } from "./client";
export { Job, JobResult } from "./models";
export { JobStatus, JobPriority, ClientConfig } from "./types";
export {
  BananasError,
  ConnectionError,
  JobNotFoundError,
  JobFailedError,
  TimeoutError,
  SerializationError,
} from "./exceptions";
```

### Testing Strategy

#### Unit Tests (90%+ coverage target)

1. **models.test.ts**
   - Job creation and serialization
   - JobResult creation and methods
   - Enum values
   - Edge cases

2. **queue.test.ts**
   - Enqueue operations
   - Job retrieval
   - Key generation
   - Priority queue selection
   - Use ioredis-mock

3. **resultBackend.test.ts**
   - Store and retrieve results
   - TTL configuration
   - Pub/sub notifications
   - Timeout handling
   - Use ioredis-mock

4. **client.test.ts**
   - All public methods
   - Error handling
   - Connection failures
   - Use ioredis-mock

#### Integration Tests

5. **endToEnd.test.ts**
   - Requires real Redis instance
   - Full workflow tests
   - Scheduled jobs
   - Priority ordering

#### Test Configuration

```javascript
// jest.config.js
module.exports = {
  preset: "ts-jest",
  testEnvironment: "node",
  roots: ["<rootDir>/test"],
  testMatch: ["**/*.test.ts"],
  collectCoverageFrom: [
    "src/**/*.ts",
    "!src/**/*.d.ts",
    "!src/index.ts",
  ],
  coverageThreshold: {
    global: {
      branches: 80,
      functions: 80,
      lines: 80,
      statements: 80,
    },
  },
};
```

### Documentation Strategy

#### TSDoc Comments

- Use TSDoc format for all public APIs
- Include `@example` blocks in docstrings
- Provide type definitions for all parameters

#### TypeDoc Generation

```bash
# Generate API documentation
npx typedoc --out docs/api src/index.ts
```

#### README.md

```markdown
# Bananas TypeScript SDK

TypeScript/JavaScript client for the Bananas distributed task queue system.

## Installation

```bash
npm install @bananas/client
# or
yarn add @bananas/client
```

## Quick Start

```typescript
import { Client, JobPriority } from "@bananas/client";

// Create client
const client = new Client({ redisUrl: "redis://localhost:6379" });

// Submit a job
const jobId = await client.submitJob(
  "send_email",
  { to: "user@example.com", subject: "Hello!" },
  JobPriority.Normal
);

console.log(`Submitted job: ${jobId}`);

// Get result
const result = await client.getResult(jobId);
if (result?.isSuccess) {
  console.log(`Job completed:`, result.result);
}

// Or use RPC-style
const result = await client.submitAndWait(
  "send_email",
  { to: "user@example.com" },
  JobPriority.Normal,
  30000 // timeout in ms
);

// Clean up
client.close();
```

## Features

- ✓ Full TypeScript support with type definitions
- ✓ Priority-based job processing (high, normal, low)
- ✓ Scheduled job execution
- ✓ Job result storage with configurable TTL
- ✓ RPC-style synchronous job execution
- ✓ Promise-based async API
- ✓ Comprehensive error handling
```

### Packaging

#### package.json

```json
{
  "name": "@bananas/client",
  "version": "0.1.0",
  "description": "TypeScript client for the Bananas distributed task queue",
  "main": "dist/index.js",
  "types": "dist/index.d.ts",
  "files": [
    "dist",
    "README.md",
    "LICENSE"
  ],
  "scripts": {
    "build": "tsc -p tsconfig.build.json",
    "test": "jest",
    "test:coverage": "jest --coverage",
    "lint": "eslint src test --ext .ts",
    "format": "prettier --write \"src/**/*.ts\" \"test/**/*.ts\"",
    "docs": "typedoc --out docs/api src/index.ts",
    "prepublishOnly": "npm run build"
  },
  "keywords": [
    "task-queue",
    "job-queue",
    "redis",
    "distributed",
    "background-jobs"
  ],
  "author": "Bananas Team",
  "license": "MIT",
  "repository": {
    "type": "git",
    "url": "https://github.com/muaviaUsmani/bananas.git",
    "directory": "sdk/typescript"
  },
  "bugs": {
    "url": "https://github.com/muaviaUsmani/bananas/issues"
  },
  "homepage": "https://github.com/muaviaUsmani/bananas#readme",
  "dependencies": {
    "ioredis": "^5.3.0"
  },
  "devDependencies": {
    "@types/node": "^20.0.0",
    "@types/jest": "^29.0.0",
    "@typescript-eslint/eslint-plugin": "^6.0.0",
    "@typescript-eslint/parser": "^6.0.0",
    "eslint": "^8.50.0",
    "eslint-config-prettier": "^9.0.0",
    "eslint-plugin-prettier": "^5.0.0",
    "jest": "^29.0.0",
    "ts-jest": "^29.0.0",
    "ts-node": "^10.9.0",
    "typedoc": "^0.25.0",
    "typescript": "^5.2.0",
    "prettier": "^3.0.0",
    "ioredis-mock": "^8.9.0"
  },
  "engines": {
    "node": ">=16.0.0"
  }
}
```

#### tsconfig.json

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "lib": ["ES2020"],
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true,
    "outDir": "./dist",
    "rootDir": "./src",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "moduleResolution": "node",
    "resolveJsonModule": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist", "test"]
}
```

---

## Implementation Order

### Phase 1: Python SDK Foundation (Week 1)

1. **Day 1-2**: Setup and Models
   - Create directory structure
   - Setup pyproject.toml, pytest, etc.
   - Implement models.py (Job, JobResult, enums)
   - Write model tests

2. **Day 3-4**: Queue and Result Backend
   - Implement queue.py (RedisQueue)
   - Implement result_backend.py (RedisResultBackend)
   - Write unit tests with fakeredis

3. **Day 5-7**: Client and Integration
   - Implement client.py (Client class)
   - Write client unit tests
   - Write integration tests
   - Create examples

### Phase 2: Python SDK Documentation (Week 2)

4. **Day 1-2**: Documentation
   - Write docstrings for all public APIs
   - Setup Sphinx
   - Create quickstart guide
   - Write API reference

5. **Day 3-4**: Packaging and Polish
   - Test packaging with pip install -e
   - Run linters (black, isort, mypy, pylint)
   - Ensure 90%+ test coverage
   - Create README and CHANGELOG

6. **Day 5**: Publishing Preparation
   - Test package installation
   - Verify examples work
   - Prepare for PyPI publishing

### Phase 3: TypeScript SDK Foundation (Week 3)

7. **Day 1-2**: Setup and Models
   - Create directory structure
   - Setup package.json, jest, etc.
   - Implement models.ts (Job, JobResult, enums)
   - Write model tests

8. **Day 3-4**: Queue and Result Backend
   - Implement queue.ts (RedisQueue)
   - Implement resultBackend.ts (RedisResultBackend)
   - Write unit tests with ioredis-mock

9. **Day 5-7**: Client and Integration
   - Implement client.ts (Client class)
   - Write client unit tests
   - Write integration tests
   - Create examples

### Phase 4: TypeScript SDK Documentation (Week 4)

10. **Day 1-2**: Documentation
    - Write TSDoc for all public APIs
    - Generate TypeDoc documentation
    - Create README
    - Write usage examples

11. **Day 3-4**: Packaging and Polish
    - Test package build
    - Run linters (ESLint, Prettier)
    - Ensure 90%+ test coverage
    - Create CHANGELOG

12. **Day 5**: Publishing Preparation
    - Test npm package installation
    - Verify examples work
    - Prepare for NPM publishing

---

## Common Considerations

### API Compatibility Matrix

| Go Client Method | Python Client Method | TypeScript Client Method |
|------------------|----------------------|--------------------------|
| `NewClient(url)` | `Client(url)` | `new Client({redisUrl})` |
| `NewClientWithConfig(url, success, failure)` | `Client(url, success_ttl, failure_ttl)` | `new Client({redisUrl, successTTL, failureTTL})` |
| `SubmitJob(name, payload, priority, desc)` | `submit_job(name, payload, priority, description)` | `submitJob(name, payload, priority, description)` |
| `SubmitJobScheduled(...)` | `submit_job_scheduled(...)` | `submitJobScheduled(...)` |
| `GetJob(jobID)` | `get_job(job_id)` | `getJob(jobId)` |
| `GetResult(ctx, jobID)` | `get_result(job_id)` | `getResult(jobId)` |
| `SubmitAndWait(ctx, ...)` | `submit_and_wait(...)` | `submitAndWait(...)` |
| `Close()` | `close()` | `close()` |

### Error Handling Patterns

**Go:**
```go
result, err := client.GetResult(ctx, jobID)
if err != nil {
    return fmt.Errorf("failed: %w", err)
}
```

**Python:**
```python
try:
    result = client.get_result(job_id)
except BananasError as e:
    print(f"Failed: {e}")
```

**TypeScript:**
```typescript
try {
    const result = await client.getResult(jobId);
} catch (error) {
    if (error instanceof BananasError) {
        console.error(`Failed: ${error.message}`);
    }
}
```

### Connection Pooling

- **Go**: go-redis handles connection pooling internally with configured pool size
- **Python**: redis-py manages connection pools automatically
- **TypeScript**: ioredis provides built-in connection pool management

All SDKs should use the Redis client's default pooling behavior without additional configuration.

### Async/Await Patterns

- **Go**: Context-based cancellation, synchronous with goroutines
- **Python**: Optional async support (can add async methods later)
- **TypeScript**: Promise-based async/await (all methods are async)

### Type Safety

- **Go**: Strong static typing with structs
- **Python**: Type hints with mypy validation
- **TypeScript**: Full type safety with TypeScript compiler

### Testing Approach

**Unit Tests:**
- Mock Redis with fakeredis (Python) or ioredis-mock (TypeScript)
- Test each class/module independently
- Aim for 90%+ coverage

**Integration Tests:**
- Use real Redis instance (Docker container)
- Test full workflows
- Test interaction between components
- Verify compatibility with Go workers

---

## Gotchas and Challenges

### 1. Timestamp Handling

**Challenge**: Go uses RFC3339 format, need to ensure consistency across languages.

**Solution**:
- Python: Use `datetime.isoformat() + "Z"` for UTC timestamps
- TypeScript: Use `date.toISOString()` which outputs RFC3339

### 2. JSON Serialization

**Challenge**: Payload serialization differences between languages.

**Solution**:
- All SDKs store payload as JSON in Redis
- Ensure proper JSON encoding/decoding
- Handle null values consistently

### 3. Scheduled Jobs Implementation

**Challenge**: The Go client uses a workaround (enqueue + dequeue + fail) for scheduled jobs.

**Solution**:
- Both Python and TypeScript SDKs should use direct ZADD to scheduled set
- This is simpler and more efficient
- Document the difference from Go implementation
- Consider updating Go implementation later

### 4. Pub/Sub Timeout Handling

**Challenge**: Redis pub/sub doesn't have built-in timeout support.

**Solution**:
- Python: Use threading with timeout or select with timeout
- TypeScript: Use Promise.race() with timeout promise
- Always do final check after timeout expires

### 5. Connection Management

**Challenge**: Ensuring connections are properly closed.

**Solution**:
- Python: Implement context manager (`__enter__`, `__exit__`)
- TypeScript: Provide explicit `close()` method
- Document importance of calling close()
- Consider connection pooling for high-throughput scenarios

### 6. Error Message Consistency

**Challenge**: Maintaining consistent error messages across SDKs.

**Solution**:
- Define error types/classes matching Go's error patterns
- Use descriptive error messages
- Include context (job ID, operation type, etc.)

### 7. Testing with Real Redis

**Challenge**: Integration tests need Redis instance.

**Solution**:
- Use Docker containers in CI/CD
- Provide docker-compose.yml for local testing
- Document how to run tests
- Make integration tests optional (skip if Redis not available)

### 8. Versioning and Compatibility

**Challenge**: Keeping SDKs in sync with server changes.

**Solution**:
- Version SDKs independently
- Document compatibility matrix
- Use semantic versioning
- Test SDKs against multiple server versions

### 9. Result Backend Pub/Sub Cleanup

**Challenge**: Pub/sub subscribers need proper cleanup.

**Solution**:
- Python: Use try/finally to ensure unsubscribe
- TypeScript: Always cleanup subscriber, even on error
- Prevent connection leaks

### 10. Protobuf Support

**Challenge**: Go implementation supports both JSON and Protobuf payloads.

**Solution**:
- **Phase 4.1 & 4.2**: Focus on JSON support only
- **Future Phase**: Add Protobuf support if needed
- Document JSON-only limitation in v0.1.0

---

## Success Criteria

### Python SDK

- [ ] All public APIs implemented and match Go client functionality
- [ ] 90%+ test coverage (unit + integration)
- [ ] Type hints on all public APIs
- [ ] Passes mypy, pylint, black, isort checks
- [ ] Comprehensive documentation (Sphinx + README)
- [ ] At least 3 working examples
- [ ] Package installable via pip
- [ ] Compatible with Python 3.8+

### TypeScript SDK

- [ ] All public APIs implemented and match Go client functionality
- [ ] 90%+ test coverage (unit + integration)
- [ ] Full TypeScript type definitions
- [ ] Passes ESLint and Prettier checks
- [ ] Comprehensive documentation (TypeDoc + README)
- [ ] At least 3 working examples
- [ ] Package installable via npm/yarn
- [ ] Compatible with Node.js 16+

### Both SDKs

- [ ] Can submit jobs that Go workers can process
- [ ] Can retrieve results stored by Go workers
- [ ] Pass integration tests with real Redis
- [ ] Documentation explains differences from Go client
- [ ] CHANGELOG documenting all features
- [ ] LICENSE file included
- [ ] Ready for publishing (PyPI / NPM)

---

## Future Enhancements (Post v0.1.0)

1. **Async Support (Python)**
   - Add async/await methods using aioredis
   - Provide both sync and async APIs

2. **Protobuf Support**
   - Support protobuf-encoded payloads
   - Match Go implementation's dual format support

3. **Batch Operations**
   - Submit multiple jobs at once
   - Bulk result retrieval

4. **Monitoring & Metrics**
   - Queue depth monitoring
   - Job statistics
   - Performance metrics

5. **Advanced Features**
   - Job cancellation
   - Job chaining/workflows
   - Job dependencies
   - Priority queue introspection

6. **Developer Experience**
   - CLI tools for job management
   - Debugging utilities
   - Better error messages with suggestions

---

## Appendix: Quick Reference

### Python Example

```python
from bananas import Client, JobPriority
from datetime import timedelta

# Create client
with Client("redis://localhost:6379") as client:
    # Submit job
    job_id = client.submit_job(
        "send_email",
        {"to": "user@example.com", "subject": "Hello"},
        JobPriority.NORMAL,
        description="Welcome email"
    )
    
    # Get result (blocking)
    result = client.submit_and_wait(
        "process_data",
        {"dataset": "users"},
        timeout=timedelta(seconds=60)
    )
    
    if result.is_success:
        print(f"Result: {result.result}")
```

### TypeScript Example

```typescript
import { Client, JobPriority } from "@bananas/client";

async function main() {
  // Create client
  const client = new Client({ redisUrl: "redis://localhost:6379" });
  
  try {
    // Submit job
    const jobId = await client.submitJob(
      "send_email",
      { to: "user@example.com", subject: "Hello" },
      JobPriority.Normal,
      "Welcome email"
    );
    
    // Get result (blocking)
    const result = await client.submitAndWait(
      "process_data",
      { dataset: "users" },
      JobPriority.Normal,
      60000 // 60 seconds
    );
    
    if (result.isSuccess) {
      console.log("Result:", result.result);
    }
  } finally {
    client.close();
  }
}

main();
```

---

## Summary

This implementation plan provides a comprehensive roadmap for creating Python and TypeScript SDKs that are:

1. **API-compatible** with the Go client
2. **Language-idiomatic** (following Python and TypeScript best practices)
3. **Well-tested** (90%+ coverage with unit and integration tests)
4. **Well-documented** (comprehensive docs, examples, and README)
5. **Production-ready** (error handling, connection management, type safety)

The 4-week timeline is realistic for a single developer, with each SDK taking approximately 2 weeks to complete. The modular structure allows for parallel development if multiple developers are available.

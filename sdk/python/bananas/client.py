"""
Bananas client implementation.

This module provides the main Client class for submitting and managing jobs.
"""

from datetime import datetime, timedelta, timezone
from typing import Any, Dict, Optional

from .exceptions import ConnectionError as BananasConnectionError
from .models import Job, JobPriority, JobResult, JobStatus
from .queue import RedisQueue
from .result_backend import RedisResultBackend


class Client:
    """Bananas job queue client.

    This is the main entry point for submitting and managing jobs.
    It provides a simple API for job submission, result retrieval,
    and synchronous job execution.

    Example:
        >>> from bananas import Client, JobPriority
        >>> client = Client("redis://localhost:6379/0")
        >>> job_id = client.submit_job(
        ...     "send_email",
        ...     {"to": "user@example.com"},
        ...     JobPriority.HIGH
        ... )
        >>> result = client.get_result(job_id)
        >>> client.close()

    Or using context manager:
        >>> with Client("redis://localhost:6379/0") as client:
        ...     job_id = client.submit_job("send_email", {...}, JobPriority.HIGH)
        ...     result = client.submit_and_wait("process_data", {...}, JobPriority.NORMAL)
    """

    def __init__(
        self,
        redis_url: str,
        success_ttl: timedelta = timedelta(hours=1),
        failure_ttl: timedelta = timedelta(hours=24),
    ):
        """Initialize the client.

        Args:
            redis_url: Redis connection URL (e.g., "redis://localhost:6379/0")
            success_ttl: TTL for successful results (default: 1 hour)
            failure_ttl: TTL for failed results (default: 24 hours)

        Raises:
            BananasConnectionError: If connection to Redis fails
        """
        self.queue = RedisQueue(redis_url)
        self.result_backend = RedisResultBackend(redis_url, success_ttl, failure_ttl)

    def submit_job(
        self,
        name: str,
        payload: Dict[str, Any],
        priority: JobPriority,
        description: str = "",
        routing_key: str = "",
    ) -> str:
        """Submit a new job to the queue.

        Args:
            name: Job name (identifies the handler to execute)
            payload: Job data as a dictionary
            priority: Job priority level
            description: Optional human-readable description
            routing_key: Optional routing key for directed task routing

        Returns:
            Job ID (UUID string)

        Raises:
            BananasConnectionError: If job submission fails

        Example:
            >>> job_id = client.submit_job(
            ...     "send_email",
            ...     {"to": "user@example.com", "subject": "Hello"},
            ...     JobPriority.HIGH,
            ...     description="Welcome email to new user"
            ... )
        """
        job = Job(
            name=name,
            payload=payload,
            priority=priority,
            description=description,
            routing_key=routing_key,
        )
        self.queue.enqueue(job)
        return job.id

    def submit_job_with_route(
        self,
        name: str,
        payload: Dict[str, Any],
        priority: JobPriority,
        routing_key: str,
        description: str = "",
    ) -> str:
        """Submit a job with a routing key (alias for submit_job with routing_key).

        This method is provided for API compatibility with the Go client.

        Args:
            name: Job name
            payload: Job data
            priority: Job priority
            routing_key: Routing key for directed task routing
            description: Optional description

        Returns:
            Job ID (UUID string)

        Raises:
            BananasConnectionError: If job submission fails

        Example:
            >>> # Route to GPU workers
            >>> job_id = client.submit_job_with_route(
            ...     "process_video",
            ...     {"video_url": "https://..."},
            ...     JobPriority.HIGH,
            ...     "gpu"
            ... )
        """
        return self.submit_job(name, payload, priority, description, routing_key)

    def submit_job_scheduled(
        self,
        name: str,
        payload: Dict[str, Any],
        priority: JobPriority,
        scheduled_for: datetime,
        description: str = "",
        routing_key: str = "",
    ) -> str:
        """Submit a job scheduled for future execution.

        Args:
            name: Job name
            payload: Job data
            priority: Job priority
            scheduled_for: When the job should be executed
            description: Optional description
            routing_key: Optional routing key

        Returns:
            Job ID (UUID string)

        Raises:
            BananasConnectionError: If job submission fails

        Example:
            >>> from datetime import datetime, timedelta, timezone
            >>> # Schedule job for 1 hour from now
            >>> scheduled_time = datetime.now(timezone.utc) + timedelta(hours=1)
            >>> job_id = client.submit_job_scheduled(
            ...     "send_reminder",
            ...     {"user_id": 123},
            ...     JobPriority.NORMAL,
            ...     scheduled_time
            ... )
        """
        job = Job(
            name=name,
            payload=payload,
            priority=priority,
            description=description,
            status=JobStatus.SCHEDULED,
            scheduled_for=scheduled_for,
            routing_key=routing_key,
        )
        self.queue.enqueue_scheduled(job)
        return job.id

    def get_job(self, job_id: str) -> Optional[Job]:
        """Retrieve a job by its ID.

        Args:
            job_id: Job identifier

        Returns:
            Job instance if found, None otherwise

        Raises:
            BananasConnectionError: If retrieval fails

        Example:
            >>> job = client.get_job(job_id)
            >>> if job:
            ...     print(f"Job status: {job.status}")
        """
        return self.queue.get_job(job_id)

    def get_result(self, job_id: str) -> Optional[JobResult]:
        """Retrieve the result of a completed job.

        Args:
            job_id: Job identifier

        Returns:
            JobResult if found, None if not available or expired

        Raises:
            BananasConnectionError: If retrieval fails

        Example:
            >>> result = client.get_result(job_id)
            >>> if result and result.is_success():
            ...     print(f"Result: {result.result}")
            >>> elif result and result.is_failed():
            ...     print(f"Error: {result.error}")
        """
        return self.result_backend.get_result(job_id)

    def submit_and_wait(
        self,
        name: str,
        payload: Dict[str, Any],
        priority: JobPriority,
        timeout: timedelta = timedelta(minutes=5),
        description: str = "",
        routing_key: str = "",
    ) -> Optional[JobResult]:
        """Submit a job and wait for its result (RPC-style execution).

        This is a convenience method for synchronous job execution.
        It submits the job and blocks until it completes or times out.

        Args:
            name: Job name
            payload: Job data
            priority: Job priority
            timeout: Maximum time to wait (default: 5 minutes)
            description: Optional description
            routing_key: Optional routing key

        Returns:
            JobResult if job completes within timeout, None otherwise

        Raises:
            BananasConnectionError: If submission or waiting fails

        Example:
            >>> from datetime import timedelta
            >>> result = client.submit_and_wait(
            ...     "generate_report",
            ...     {"report_type": "sales"},
            ...     JobPriority.HIGH,
            ...     timeout=timedelta(minutes=10)
            ... )
            >>> if result and result.is_success():
            ...     print(f"Report: {result.result}")
        """
        job = Job(
            name=name,
            payload=payload,
            priority=priority,
            description=description,
            routing_key=routing_key,
        )
        self.queue.enqueue(job)
        return self.result_backend.wait_for_result(job.id, timeout)

    def close(self) -> None:
        """Close all Redis connections.

        This should be called when you're done using the client.
        Alternatively, use the client as a context manager.
        """
        if self.queue:
            self.queue.close()
        if self.result_backend:
            self.result_backend.close()

    def __enter__(self) -> "Client":
        """Context manager entry.

        Example:
            >>> with Client("redis://localhost:6379/0") as client:
            ...     job_id = client.submit_job(...)
        """
        return self

    def __exit__(self, exc_type, exc_val, exc_tb) -> None:
        """Context manager exit."""
        self.close()

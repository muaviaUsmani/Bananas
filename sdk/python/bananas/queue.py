"""
Bananas Redis queue implementation.

This module provides the RedisQueue class for job queue operations.
"""

import time
from typing import Optional

import redis

from .exceptions import ConnectionError as BananasConnectionError
from .exceptions import JobNotFoundError
from .models import Job, JobPriority


class RedisQueue:
    """Redis-backed job queue.

    This class handles enqueueing jobs and retrieving job data from Redis.
    It uses the same key patterns and data structures as the Go implementation.

    Attributes:
        client: Redis client instance
        key_prefix: Prefix for all Redis keys (default: "bananas")
    """

    def __init__(self, redis_url: str, key_prefix: str = "bananas"):
        """Initialize Redis queue connection.

        Args:
            redis_url: Redis connection URL (e.g., "redis://localhost:6379/0")
            key_prefix: Prefix for all Redis keys (default: "bananas")

        Raises:
            BananasConnectionError: If connection to Redis fails
        """
        self.key_prefix = key_prefix
        try:
            self.client = redis.from_url(
                redis_url,
                decode_responses=False,  # We handle encoding/decoding ourselves
                socket_connect_timeout=5,
                socket_keepalive=True,
            )
            # Test connection
            self.client.ping()
        except redis.RedisError as e:
            raise BananasConnectionError(f"Failed to connect to Redis: {e}") from e

    def enqueue(self, job: Job) -> None:
        """Enqueue a job to the appropriate priority queue.

        Args:
            job: Job to enqueue

        Raises:
            BananasConnectionError: If Redis operation fails
        """
        try:
            # Store job data
            job_key = f"{self.key_prefix}:job:{job.id}"
            job_json = job.to_json()
            self.client.set(job_key, job_json.encode("utf-8"))

            # Add to appropriate priority queue
            queue_key = f"{self.key_prefix}:queue:{job.priority.value}"
            self.client.lpush(queue_key, job.id.encode("utf-8"))
        except redis.RedisError as e:
            raise BananasConnectionError(f"Failed to enqueue job: {e}") from e

    def enqueue_scheduled(self, job: Job) -> None:
        """Enqueue a job to the scheduled queue.

        Args:
            job: Job to enqueue (must have scheduled_for set)

        Raises:
            BananasConnectionError: If Redis operation fails
            ValueError: If job.scheduled_for is not set
        """
        if job.scheduled_for is None:
            raise ValueError("Job must have scheduled_for set to enqueue to scheduled queue")

        try:
            # Store job data
            job_key = f"{self.key_prefix}:job:{job.id}"
            job_json = job.to_json()
            self.client.set(job_key, job_json.encode("utf-8"))

            # Add to scheduled set with timestamp as score
            scheduled_key = f"{self.key_prefix}:queue:scheduled"
            timestamp = job.scheduled_for.timestamp()
            self.client.zadd(scheduled_key, {job.id.encode("utf-8"): timestamp})
        except redis.RedisError as e:
            raise BananasConnectionError(f"Failed to enqueue scheduled job: {e}") from e

    def get_job(self, job_id: str) -> Optional[Job]:
        """Retrieve a job by its ID.

        Args:
            job_id: Job identifier

        Returns:
            Job instance if found, None otherwise

        Raises:
            BananasConnectionError: If Redis operation fails
        """
        try:
            job_key = f"{self.key_prefix}:job:{job_id}"
            job_json = self.client.get(job_key)

            if job_json is None:
                return None

            return Job.from_json(job_json.decode("utf-8"))
        except redis.RedisError as e:
            raise BananasConnectionError(f"Failed to get job: {e}") from e
        except Exception as e:
            raise JobNotFoundError(f"Failed to deserialize job: {e}") from e

    def close(self) -> None:
        """Close the Redis connection."""
        if self.client:
            self.client.close()

    def __enter__(self) -> "RedisQueue":
        """Context manager entry."""
        return self

    def __exit__(self, exc_type, exc_val, exc_tb) -> None:
        """Context manager exit."""
        self.close()

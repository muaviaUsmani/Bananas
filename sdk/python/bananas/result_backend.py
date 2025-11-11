"""
Bananas result backend implementation.

This module provides the RedisResultBackend class for storing and retrieving job results.
"""

import json
from datetime import timedelta
from typing import Optional

import redis

from .exceptions import ConnectionError as BananasConnectionError
from .exceptions import ResultNotFoundError, TimeoutError
from .models import JobResult, JobStatus


class RedisResultBackend:
    """Redis-backed result storage.

    This class handles storing and retrieving job results from Redis,
    with support for TTLs and pub/sub notifications.

    Attributes:
        client: Redis client instance
        success_ttl: TTL for successful job results
        failure_ttl: TTL for failed job results
        key_prefix: Prefix for all Redis keys (default: "bananas")
    """

    def __init__(
        self,
        redis_url: str,
        success_ttl: timedelta = timedelta(hours=1),
        failure_ttl: timedelta = timedelta(hours=24),
        key_prefix: str = "bananas",
    ):
        """Initialize Redis result backend connection.

        Args:
            redis_url: Redis connection URL
            success_ttl: TTL for successful results (default: 1 hour)
            failure_ttl: TTL for failed results (default: 24 hours)
            key_prefix: Prefix for all Redis keys (default: "bananas")

        Raises:
            BananasConnectionError: If connection to Redis fails
        """
        self.success_ttl = success_ttl
        self.failure_ttl = failure_ttl
        self.key_prefix = key_prefix

        try:
            self.client = redis.from_url(
                redis_url,
                decode_responses=False,
                socket_connect_timeout=5,
                socket_keepalive=True,
            )
            # Test connection
            self.client.ping()
        except redis.RedisError as e:
            raise BananasConnectionError(f"Failed to connect to Redis: {e}") from e

    def set_result(self, result: JobResult) -> None:
        """Store a job result in Redis.

        Args:
            result: JobResult to store

        Raises:
            BananasConnectionError: If Redis operation fails
        """
        try:
            result_key = f"{self.key_prefix}:result:{result.job_id}"

            # Build result hash
            result_data = {
                b"status": result.status.value.encode("utf-8"),
                b"completed_at": result.completed_at.isoformat().encode("utf-8"),
                b"duration_ms": str(result.duration_ms).encode("utf-8"),
            }

            if result.result:
                result_data[b"result"] = json.dumps(result.result).encode("utf-8")

            if result.error:
                result_data[b"error"] = result.error.encode("utf-8")

            # Store with appropriate TTL
            ttl = self.success_ttl if result.is_success() else self.failure_ttl
            pipe = self.client.pipeline()
            pipe.hset(result_key, mapping=result_data)
            pipe.expire(result_key, int(ttl.total_seconds()))
            pipe.execute()

            # Publish notification
            notify_channel = f"{self.key_prefix}:result:notify:{result.job_id}"
            self.client.publish(notify_channel, result.status.value.encode("utf-8"))

        except redis.RedisError as e:
            raise BananasConnectionError(f"Failed to set result: {e}") from e

    def get_result(self, job_id: str) -> Optional[JobResult]:
        """Retrieve a job result by job ID.

        Args:
            job_id: Job identifier

        Returns:
            JobResult if found, None otherwise

        Raises:
            BananasConnectionError: If Redis operation fails
        """
        try:
            result_key = f"{self.key_prefix}:result:{job_id}"
            data = self.client.hgetall(result_key)

            if not data:
                return None

            # Decode bytes to strings
            decoded_data = {}
            for key, value in data.items():
                key_str = key.decode("utf-8") if isinstance(key, bytes) else key
                value_str = value.decode("utf-8") if isinstance(value, bytes) else value
                decoded_data[key_str] = value_str

            result_dict = {
                "job_id": job_id,
                "status": decoded_data["status"],
                "completed_at": decoded_data.get("completed_at"),
                "duration_ms": decoded_data.get("duration_ms", "0"),
            }

            if "result" in decoded_data:
                result_dict["result"] = decoded_data["result"]

            if "error" in decoded_data:
                result_dict["error"] = decoded_data["error"]

            return JobResult.from_dict(result_dict)

        except redis.RedisError as e:
            raise BananasConnectionError(f"Failed to get result: {e}") from e
        except Exception as e:
            raise ResultNotFoundError(f"Failed to deserialize result: {e}") from e

    def wait_for_result(self, job_id: str, timeout: timedelta) -> Optional[JobResult]:
        """Wait for a job result with timeout.

        This method uses Redis pub/sub to wait for result notifications.

        Args:
            job_id: Job identifier
            timeout: Maximum time to wait

        Returns:
            JobResult if job completes within timeout, None otherwise

        Raises:
            BananasConnectionError: If Redis operation fails
        """
        import time

        # First check if result already exists
        result = self.get_result(job_id)
        if result:
            return result

        # Subscribe to result notification channel
        notify_channel = f"{self.key_prefix}:result:notify:{job_id}"
        pubsub = self.client.pubsub()

        try:
            pubsub.subscribe(notify_channel)

            # Wait for message with timeout
            timeout_seconds = timeout.total_seconds()
            start_time = time.time()
            poll_interval = 0.1  # 100ms

            while time.time() - start_time < timeout_seconds:
                # Check for message with short timeout
                message = pubsub.get_message(timeout=poll_interval)

                if message and message["type"] == "message":
                    # Got notification, fetch result
                    return self.get_result(job_id)

                # Also periodically check if result appeared without notification
                # (in case notification was missed)
                if time.time() - start_time > 0.5:  # After 500ms
                    result = self.get_result(job_id)
                    if result:
                        return result

            # Timeout reached
            return None

        except redis.RedisError as e:
            raise BananasConnectionError(f"Failed to wait for result: {e}") from e
        finally:
            pubsub.close()

    def close(self) -> None:
        """Close the Redis connection."""
        if self.client:
            self.client.close()

    def __enter__(self) -> "RedisResultBackend":
        """Context manager entry."""
        return self

    def __exit__(self, exc_type, exc_val, exc_tb) -> None:
        """Context manager exit."""
        self.close()

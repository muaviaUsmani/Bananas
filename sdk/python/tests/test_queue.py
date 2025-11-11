"""Tests for Bananas Redis queue."""

import pytest

from bananas import JobPriority
from bananas.exceptions import ConnectionError as BananasConnectionError
from bananas.models import Job, JobStatus
from bananas.queue import RedisQueue


class TestRedisQueue:
    """Tests for RedisQueue class."""

    def test_queue_initialization(self, mock_redis_connection, redis_url):
        """Test queue initialization."""
        queue = RedisQueue(redis_url)

        assert queue.client is not None
        assert queue.key_prefix == "bananas"

        queue.close()

    def test_queue_initialization_custom_prefix(self, mock_redis_connection, redis_url):
        """Test queue with custom prefix."""
        queue = RedisQueue(redis_url, key_prefix="custom")

        assert queue.key_prefix == "custom"

        queue.close()

    def test_enqueue(self, mock_redis_connection, redis_url):
        """Test enqueueing a job."""
        queue = RedisQueue(redis_url)
        job = Job("test_job", {"key": "value"}, JobPriority.HIGH)

        queue.enqueue(job)

        # Verify job was stored
        retrieved = queue.get_job(job.id)
        assert retrieved is not None
        assert retrieved.id == job.id
        assert retrieved.name == "test_job"

        queue.close()

    def test_enqueue_different_priorities(self, mock_redis_connection, redis_url):
        """Test enqueueing jobs with different priorities."""
        queue = RedisQueue(redis_url)

        high_job = Job("high", {}, JobPriority.HIGH)
        normal_job = Job("normal", {}, JobPriority.NORMAL)
        low_job = Job("low", {}, JobPriority.LOW)

        queue.enqueue(high_job)
        queue.enqueue(normal_job)
        queue.enqueue(low_job)

        # All should be retrievable
        assert queue.get_job(high_job.id) is not None
        assert queue.get_job(normal_job.id) is not None
        assert queue.get_job(low_job.id) is not None

        queue.close()

    def test_enqueue_scheduled(self, mock_redis_connection, redis_url):
        """Test enqueueing a scheduled job."""
        from datetime import datetime, timedelta, timezone

        queue = RedisQueue(redis_url)
        scheduled_time = datetime.now(timezone.utc) + timedelta(hours=1)

        job = Job(
            "scheduled_job",
            {},
            JobPriority.NORMAL,
            status=JobStatus.SCHEDULED,
            scheduled_for=scheduled_time,
        )

        queue.enqueue_scheduled(job)

        # Verify job was stored
        retrieved = queue.get_job(job.id)
        assert retrieved is not None
        assert retrieved.id == job.id
        assert retrieved.status == JobStatus.SCHEDULED

        queue.close()

    def test_enqueue_scheduled_without_scheduled_for(self, mock_redis_connection, redis_url):
        """Test enqueueing scheduled job without scheduled_for raises error."""
        queue = RedisQueue(redis_url)
        job = Job("test", {}, JobPriority.NORMAL)

        with pytest.raises(ValueError, match="scheduled_for"):
            queue.enqueue_scheduled(job)

        queue.close()

    def test_get_job_not_found(self, mock_redis_connection, redis_url):
        """Test getting non-existent job returns None."""
        queue = RedisQueue(redis_url)

        result = queue.get_job("non-existent-id")
        assert result is None

        queue.close()

    def test_context_manager(self, mock_redis_connection, redis_url):
        """Test using queue as context manager."""
        with RedisQueue(redis_url) as queue:
            job = Job("test", {}, JobPriority.NORMAL)
            queue.enqueue(job)

            retrieved = queue.get_job(job.id)
            assert retrieved is not None

    def test_multiple_jobs(self, mock_redis_connection, redis_url):
        """Test enqueueing and retrieving multiple jobs."""
        queue = RedisQueue(redis_url)

        jobs = []
        for i in range(10):
            job = Job(f"job_{i}", {"index": i}, JobPriority.NORMAL)
            queue.enqueue(job)
            jobs.append(job)

        # Verify all jobs
        for job in jobs:
            retrieved = queue.get_job(job.id)
            assert retrieved is not None
            assert retrieved.id == job.id

        queue.close()

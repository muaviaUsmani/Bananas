"""Tests for Bananas result backend."""

from datetime import datetime, timedelta, timezone

import pytest

from bananas.exceptions import ConnectionError as BananasConnectionError
from bananas.models import JobResult, JobStatus
from bananas.result_backend import RedisResultBackend


class TestRedisResultBackend:
    """Tests for RedisResultBackend class."""

    def test_backend_initialization(self, mock_redis_connection, redis_url):
        """Test backend initialization."""
        backend = RedisResultBackend(redis_url)

        assert backend.client is not None
        assert backend.success_ttl == timedelta(hours=1)
        assert backend.failure_ttl == timedelta(hours=24)
        assert backend.key_prefix == "bananas"

        backend.close()

    def test_backend_custom_ttls(self, mock_redis_connection, redis_url):
        """Test backend with custom TTLs."""
        success_ttl = timedelta(hours=2)
        failure_ttl = timedelta(hours=48)

        backend = RedisResultBackend(redis_url, success_ttl, failure_ttl)

        assert backend.success_ttl == success_ttl
        assert backend.failure_ttl == failure_ttl

        backend.close()

    def test_backend_custom_prefix(self, mock_redis_connection, redis_url):
        """Test backend with custom prefix."""
        backend = RedisResultBackend(redis_url, key_prefix="custom")

        assert backend.key_prefix == "custom"

        backend.close()

    def test_set_and_get_result_success(self, mock_redis_connection, redis_url):
        """Test storing and retrieving a successful result."""
        backend = RedisResultBackend(redis_url)

        result = JobResult(
            job_id="test-job-1",
            status=JobStatus.COMPLETED,
            result={"output": "success", "count": 42},
            duration_ms=1500,
        )

        backend.set_result(result)

        # Retrieve the result
        retrieved = backend.get_result("test-job-1")
        assert retrieved is not None
        assert retrieved.job_id == "test-job-1"
        assert retrieved.status == JobStatus.COMPLETED
        assert retrieved.result == {"output": "success", "count": 42}
        assert retrieved.duration_ms == 1500
        assert retrieved.is_success()

        backend.close()

    def test_set_and_get_result_failure(self, mock_redis_connection, redis_url):
        """Test storing and retrieving a failed result."""
        backend = RedisResultBackend(redis_url)

        result = JobResult(
            job_id="test-job-2",
            status=JobStatus.FAILED,
            error="Something went wrong",
            duration_ms=500,
        )

        backend.set_result(result)

        # Retrieve the result
        retrieved = backend.get_result("test-job-2")
        assert retrieved is not None
        assert retrieved.job_id == "test-job-2"
        assert retrieved.status == JobStatus.FAILED
        assert retrieved.error == "Something went wrong"
        assert retrieved.is_failed()

        backend.close()

    def test_get_result_not_found(self, mock_redis_connection, redis_url):
        """Test getting non-existent result returns None."""
        backend = RedisResultBackend(redis_url)

        result = backend.get_result("non-existent-job")
        assert result is None

        backend.close()

    def test_wait_for_result_immediate(self, mock_redis_connection, redis_url):
        """Test wait_for_result when result already exists."""
        backend = RedisResultBackend(redis_url)

        result = JobResult(
            job_id="test-job-3",
            status=JobStatus.COMPLETED,
            result={"data": "value"},
            duration_ms=100,
        )
        backend.set_result(result)

        # Should return immediately
        retrieved = backend.wait_for_result("test-job-3", timedelta(seconds=5))
        assert retrieved is not None
        assert retrieved.job_id == "test-job-3"

        backend.close()

    def test_wait_for_result_timeout(self, mock_redis_connection, redis_url):
        """Test wait_for_result times out when result doesn't appear."""
        backend = RedisResultBackend(redis_url)

        # Should timeout
        result = backend.wait_for_result("non-existent-job", timedelta(milliseconds=100))
        assert result is None

        backend.close()

    def test_context_manager(self, mock_redis_connection, redis_url):
        """Test using backend as context manager."""
        with RedisResultBackend(redis_url) as backend:
            result = JobResult(
                job_id="test-job-4",
                status=JobStatus.COMPLETED,
                result={"test": True},
                duration_ms=200,
            )
            backend.set_result(result)

            retrieved = backend.get_result("test-job-4")
            assert retrieved is not None

    def test_multiple_results(self, mock_redis_connection, redis_url):
        """Test storing and retrieving multiple results."""
        backend = RedisResultBackend(redis_url)

        # Create multiple results
        for i in range(5):
            result = JobResult(
                job_id=f"job-{i}",
                status=JobStatus.COMPLETED,
                result={"index": i},
                duration_ms=100 * i,
            )
            backend.set_result(result)

        # Retrieve all results
        for i in range(5):
            retrieved = backend.get_result(f"job-{i}")
            assert retrieved is not None
            assert retrieved.job_id == f"job-{i}"
            assert retrieved.result == {"index": i}

        backend.close()

    def test_result_with_empty_result_data(self, mock_redis_connection, redis_url):
        """Test result with no result data."""
        backend = RedisResultBackend(redis_url)

        result = JobResult(
            job_id="test-job-5",
            status=JobStatus.COMPLETED,
            result=None,
            duration_ms=50,
        )
        backend.set_result(result)

        retrieved = backend.get_result("test-job-5")
        assert retrieved is not None
        assert retrieved.result is None
        assert retrieved.is_success()

        backend.close()

    def test_result_with_complex_data(self, mock_redis_connection, redis_url):
        """Test result with complex nested data."""
        backend = RedisResultBackend(redis_url)

        complex_result = {
            "user": {
                "id": 123,
                "name": "John Doe",
                "roles": ["admin", "user"],
            },
            "permissions": ["read", "write", "delete"],
            "metadata": {
                "timestamp": "2025-01-15T10:30:00Z",
                "version": "1.0.0",
            },
        }

        result = JobResult(
            job_id="test-job-6",
            status=JobStatus.COMPLETED,
            result=complex_result,
            duration_ms=1000,
        )
        backend.set_result(result)

        retrieved = backend.get_result("test-job-6")
        assert retrieved is not None
        assert retrieved.result == complex_result

        backend.close()

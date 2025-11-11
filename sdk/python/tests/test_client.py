"""Tests for Bananas client."""

from datetime import datetime, timedelta, timezone

import pytest

from bananas import Client, JobPriority, JobStatus
from bananas.exceptions import ConnectionError as BananasConnectionError


class TestClient:
    """Tests for Client class."""

    def test_client_initialization(self, mock_redis_connection, redis_url):
        """Test client initialization."""
        client = Client(redis_url)

        assert client.queue is not None
        assert client.result_backend is not None

        client.close()

    def test_submit_job(self, mock_redis_connection, redis_url):
        """Test submitting a basic job."""
        client = Client(redis_url)

        job_id = client.submit_job(
            "test_job",
            {"key": "value"},
            JobPriority.HIGH,
            description="Test job",
        )

        assert job_id is not None
        assert isinstance(job_id, str)

        # Verify job was enqueued
        job = client.get_job(job_id)
        assert job is not None
        assert job.name == "test_job"
        assert job.payload == {"key": "value"}
        assert job.priority == JobPriority.HIGH
        assert job.description == "Test job"
        assert job.status == JobStatus.PENDING

        client.close()

    def test_submit_job_with_routing_key(self, mock_redis_connection, redis_url):
        """Test submitting a job with routing key."""
        client = Client(redis_url)

        job_id = client.submit_job(
            "gpu_job",
            {"data": [1, 2, 3]},
            JobPriority.NORMAL,
            routing_key="gpu",
        )

        job = client.get_job(job_id)
        assert job.routing_key == "gpu"

        client.close()

    def test_submit_job_with_route(self, mock_redis_connection, redis_url):
        """Test submit_job_with_route alias."""
        client = Client(redis_url)

        job_id = client.submit_job_with_route(
            "gpu_job",
            {"data": [1, 2, 3]},
            JobPriority.NORMAL,
            "gpu",
            description="GPU task",
        )

        job = client.get_job(job_id)
        assert job.routing_key == "gpu"
        assert job.description == "GPU task"

        client.close()

    def test_submit_job_scheduled(self, mock_redis_connection, redis_url):
        """Test submitting a scheduled job."""
        client = Client(redis_url)

        scheduled_time = datetime.now(timezone.utc) + timedelta(hours=1)
        job_id = client.submit_job_scheduled(
            "scheduled_job",
            {"key": "value"},
            JobPriority.NORMAL,
            scheduled_time,
            description="Scheduled task",
        )

        assert job_id is not None

        # Verify job was stored
        job = client.get_job(job_id)
        assert job is not None
        assert job.name == "scheduled_job"
        assert job.status == JobStatus.SCHEDULED
        assert job.scheduled_for is not None

        client.close()

    def test_get_job_not_found(self, mock_redis_connection, redis_url):
        """Test getting a non-existent job."""
        client = Client(redis_url)

        job = client.get_job("non-existent-id")
        assert job is None

        client.close()

    def test_get_result_not_found(self, mock_redis_connection, redis_url):
        """Test getting a non-existent result."""
        client = Client(redis_url)

        result = client.get_result("non-existent-id")
        assert result is None

        client.close()

    def test_get_result_success(self, mock_redis_connection, redis_url):
        """Test getting a successful result."""
        from bananas.models import JobResult

        client = Client(redis_url)

        # Submit a job
        job_id = client.submit_job("test_job", {"key": "value"}, JobPriority.NORMAL)

        # Manually store a result
        result = JobResult(
            job_id=job_id,
            status=JobStatus.COMPLETED,
            result={"output": "success"},
            duration_ms=1500,
        )
        client.result_backend.set_result(result)

        # Retrieve the result
        retrieved = client.get_result(job_id)
        assert retrieved is not None
        assert retrieved.job_id == job_id
        assert retrieved.is_success()
        assert retrieved.result == {"output": "success"}

        client.close()

    def test_context_manager(self, mock_redis_connection, redis_url):
        """Test using client as context manager."""
        with Client(redis_url) as client:
            job_id = client.submit_job("test", {}, JobPriority.NORMAL)
            assert job_id is not None

        # Client should be closed after exiting context

    def test_close(self, mock_redis_connection, redis_url):
        """Test closing client connections."""
        client = Client(redis_url)
        client.close()

        # Should not raise errors on double close
        client.close()

    def test_custom_ttls(self, mock_redis_connection, redis_url):
        """Test creating client with custom TTLs."""
        success_ttl = timedelta(hours=2)
        failure_ttl = timedelta(hours=48)

        client = Client(redis_url, success_ttl=success_ttl, failure_ttl=failure_ttl)

        assert client.result_backend.success_ttl == success_ttl
        assert client.result_backend.failure_ttl == failure_ttl

        client.close()

    def test_submit_and_wait_timeout(self, mock_redis_connection, redis_url):
        """Test submit_and_wait with timeout."""
        client = Client(redis_url)

        # Submit job and wait (will timeout since no worker is processing)
        result = client.submit_and_wait(
            "test_job",
            {"key": "value"},
            JobPriority.NORMAL,
            timeout=timedelta(milliseconds=100),
        )

        # Should timeout and return None
        assert result is None

        client.close()

    def test_submit_and_wait_success(self, mock_redis_connection, redis_url):
        """Test submit_and_wait with immediate result."""
        import threading
        import time

        from bananas.models import JobResult

        client = Client(redis_url)

        # Submit job
        def complete_job():
            time.sleep(0.1)
            # Simulate worker completing the job
            # Get the job that was just submitted
            jobs = []
            for key in mock_redis_connection.keys(b"bananas:job:*"):
                jobs.append(key)

            if jobs:
                job_key = jobs[-1].decode("utf-8")
                job_id = job_key.split(":")[-1]

                result = JobResult(
                    job_id=job_id,
                    status=JobStatus.COMPLETED,
                    result={"output": "success"},
                    duration_ms=100,
                )
                client.result_backend.set_result(result)

        # Start background thread to complete job
        thread = threading.Thread(target=complete_job)
        thread.start()

        # Submit and wait
        result = client.submit_and_wait(
            "test_job",
            {"key": "value"},
            JobPriority.NORMAL,
            timeout=timedelta(seconds=2),
        )

        thread.join()

        # Should get result
        assert result is not None
        assert result.is_success()

        client.close()

    def test_multiple_jobs(self, mock_redis_connection, redis_url):
        """Test submitting multiple jobs."""
        client = Client(redis_url)

        job_ids = []
        for i in range(5):
            job_id = client.submit_job(
                f"job_{i}",
                {"index": i},
                JobPriority.NORMAL,
            )
            job_ids.append(job_id)

        # Verify all jobs were created
        for i, job_id in enumerate(job_ids):
            job = client.get_job(job_id)
            assert job is not None
            assert job.name == f"job_{i}"
            assert job.payload == {"index": i}

        client.close()

    def test_different_priorities(self, mock_redis_connection, redis_url):
        """Test submitting jobs with different priorities."""
        client = Client(redis_url)

        high_id = client.submit_job("high", {}, JobPriority.HIGH)
        normal_id = client.submit_job("normal", {}, JobPriority.NORMAL)
        low_id = client.submit_job("low", {}, JobPriority.LOW)

        high_job = client.get_job(high_id)
        normal_job = client.get_job(normal_id)
        low_job = client.get_job(low_id)

        assert high_job.priority == JobPriority.HIGH
        assert normal_job.priority == JobPriority.NORMAL
        assert low_job.priority == JobPriority.LOW

        client.close()

"""Tests for Bananas models."""

import json
from datetime import datetime, timedelta, timezone

import pytest

from bananas.models import Job, JobPriority, JobResult, JobStatus


class TestJobStatus:
    """Tests for JobStatus enum."""

    def test_job_status_values(self):
        """Test JobStatus enum values."""
        assert JobStatus.PENDING.value == "pending"
        assert JobStatus.PROCESSING.value == "processing"
        assert JobStatus.COMPLETED.value == "completed"
        assert JobStatus.FAILED.value == "failed"
        assert JobStatus.SCHEDULED.value == "scheduled"


class TestJobPriority:
    """Tests for JobPriority enum."""

    def test_job_priority_values(self):
        """Test JobPriority enum values."""
        assert JobPriority.HIGH.value == "high"
        assert JobPriority.NORMAL.value == "normal"
        assert JobPriority.LOW.value == "low"


class TestJob:
    """Tests for Job model."""

    def test_job_creation_minimal(self):
        """Test creating a job with minimal parameters."""
        job = Job(
            name="test_job",
            payload={"key": "value"},
            priority=JobPriority.NORMAL,
        )

        assert job.name == "test_job"
        assert job.payload == {"key": "value"}
        assert job.priority == JobPriority.NORMAL
        assert job.status == JobStatus.PENDING
        assert job.attempts == 0
        assert job.max_retries == 3
        assert job.error == ""
        assert job.description == ""
        assert job.routing_key == ""
        assert job.scheduled_for is None
        assert job.id is not None
        assert isinstance(job.created_at, datetime)
        assert isinstance(job.updated_at, datetime)

    def test_job_creation_full(self):
        """Test creating a job with all parameters."""
        now = datetime.now(timezone.utc)
        scheduled = now + timedelta(hours=1)

        job = Job(
            name="test_job",
            payload={"key": "value"},
            priority=JobPriority.HIGH,
            description="Test description",
            job_id="custom-id",
            status=JobStatus.SCHEDULED,
            created_at=now,
            updated_at=now,
            scheduled_for=scheduled,
            attempts=2,
            max_retries=5,
            error="test error",
            routing_key="gpu",
        )

        assert job.id == "custom-id"
        assert job.name == "test_job"
        assert job.description == "Test description"
        assert job.payload == {"key": "value"}
        assert job.status == JobStatus.SCHEDULED
        assert job.priority == JobPriority.HIGH
        assert job.created_at == now
        assert job.updated_at == now
        assert job.scheduled_for == scheduled
        assert job.attempts == 2
        assert job.max_retries == 5
        assert job.error == "test error"
        assert job.routing_key == "gpu"

    def test_update_status(self):
        """Test updating job status."""
        job = Job("test", {}, JobPriority.NORMAL)
        original_updated = job.updated_at

        # Small delay to ensure timestamp changes
        import time

        time.sleep(0.01)

        job.update_status(JobStatus.PROCESSING)

        assert job.status == JobStatus.PROCESSING
        assert job.updated_at > original_updated

    def test_to_dict(self):
        """Test converting job to dictionary."""
        job = Job(
            name="test_job",
            payload={"key": "value"},
            priority=JobPriority.HIGH,
            description="Test",
            routing_key="gpu",
        )

        data = job.to_dict()

        assert data["id"] == job.id
        assert data["name"] == "test_job"
        assert data["description"] == "Test"
        assert data["payload"] == {"key": "value"}
        assert data["status"] == "pending"
        assert data["priority"] == "high"
        assert data["attempts"] == 0
        assert data["max_retries"] == 3
        assert data["error"] == ""
        assert data["routing_key"] == "gpu"
        assert "created_at" in data
        assert "updated_at" in data

    def test_to_dict_with_scheduled(self):
        """Test converting scheduled job to dictionary."""
        scheduled = datetime.now(timezone.utc) + timedelta(hours=1)
        job = Job(
            name="test",
            payload={},
            priority=JobPriority.NORMAL,
            scheduled_for=scheduled,
        )

        data = job.to_dict()
        assert "scheduled_for" in data

    def test_to_json(self):
        """Test converting job to JSON."""
        job = Job("test", {"key": "value"}, JobPriority.NORMAL)
        json_str = job.to_json()

        assert isinstance(json_str, str)
        data = json.loads(json_str)
        assert data["name"] == "test"
        assert data["payload"] == {"key": "value"}

    def test_from_dict(self):
        """Test creating job from dictionary."""
        data = {
            "id": "test-id",
            "name": "test_job",
            "description": "Test",
            "payload": {"key": "value"},
            "status": "pending",
            "priority": "high",
            "created_at": "2025-01-15T10:30:00+00:00",
            "updated_at": "2025-01-15T10:30:05+00:00",
            "attempts": 1,
            "max_retries": 3,
            "error": "",
            "routing_key": "gpu",
        }

        job = Job.from_dict(data)

        assert job.id == "test-id"
        assert job.name == "test_job"
        assert job.description == "Test"
        assert job.payload == {"key": "value"}
        assert job.status == JobStatus.PENDING
        assert job.priority == JobPriority.HIGH
        assert job.attempts == 1
        assert job.max_retries == 3
        assert job.routing_key == "gpu"

    def test_from_dict_with_scheduled(self):
        """Test creating scheduled job from dictionary."""
        data = {
            "id": "test-id",
            "name": "test",
            "payload": {},
            "status": "scheduled",
            "priority": "normal",
            "created_at": "2025-01-15T10:30:00+00:00",
            "updated_at": "2025-01-15T10:30:00+00:00",
            "scheduled_for": "2025-01-15T12:00:00+00:00",
            "attempts": 0,
            "max_retries": 3,
        }

        job = Job.from_dict(data)
        assert job.scheduled_for is not None
        assert isinstance(job.scheduled_for, datetime)

    def test_from_json(self):
        """Test creating job from JSON string."""
        data = {
            "id": "test-id",
            "name": "test",
            "payload": {"key": "value"},
            "status": "pending",
            "priority": "normal",
            "created_at": "2025-01-15T10:30:00+00:00",
            "updated_at": "2025-01-15T10:30:00+00:00",
            "attempts": 0,
            "max_retries": 3,
        }
        json_str = json.dumps(data)

        job = Job.from_json(json_str)
        assert job.id == "test-id"
        assert job.name == "test"

    def test_repr(self):
        """Test job string representation."""
        job = Job("test_job", {}, JobPriority.HIGH)
        repr_str = repr(job)

        assert "Job(" in repr_str
        assert "test_job" in repr_str
        assert "high" in repr_str
        assert "pending" in repr_str


class TestJobResult:
    """Tests for JobResult model."""

    def test_job_result_success(self):
        """Test creating a successful job result."""
        result = JobResult(
            job_id="test-id",
            status=JobStatus.COMPLETED,
            result={"output": "success"},
            duration_ms=1500,
        )

        assert result.job_id == "test-id"
        assert result.status == JobStatus.COMPLETED
        assert result.result == {"output": "success"}
        assert result.error == ""
        assert result.duration_ms == 1500
        assert result.is_success()
        assert not result.is_failed()

    def test_job_result_failure(self):
        """Test creating a failed job result."""
        result = JobResult(
            job_id="test-id",
            status=JobStatus.FAILED,
            error="Something went wrong",
            duration_ms=500,
        )

        assert result.job_id == "test-id"
        assert result.status == JobStatus.FAILED
        assert result.result is None
        assert result.error == "Something went wrong"
        assert result.duration_ms == 500
        assert not result.is_success()
        assert result.is_failed()

    def test_to_dict_success(self):
        """Test converting successful result to dictionary."""
        result = JobResult(
            job_id="test-id",
            status=JobStatus.COMPLETED,
            result={"output": "value"},
            duration_ms=1000,
        )

        data = result.to_dict()
        assert data["job_id"] == "test-id"
        assert data["status"] == "completed"
        assert data["result"] == {"output": "value"}
        assert data["duration_ms"] == 1000
        assert "completed_at" in data
        assert "error" not in data

    def test_to_dict_failure(self):
        """Test converting failed result to dictionary."""
        result = JobResult(
            job_id="test-id",
            status=JobStatus.FAILED,
            error="Error message",
            duration_ms=500,
        )

        data = result.to_dict()
        assert data["error"] == "Error message"
        assert "result" not in data

    def test_from_dict(self):
        """Test creating result from dictionary."""
        data = {
            "job_id": "test-id",
            "status": "completed",
            "result": {"output": "value"},
            "completed_at": "2025-01-15T10:30:05+00:00",
            "duration_ms": 1500,
        }

        result = JobResult.from_dict(data)
        assert result.job_id == "test-id"
        assert result.status == JobStatus.COMPLETED
        assert result.result == {"output": "value"}
        assert result.duration_ms == 1500

    def test_from_dict_with_json_string_result(self):
        """Test parsing result when it's a JSON string."""
        data = {
            "job_id": "test-id",
            "status": "completed",
            "result": '{"output": "value"}',
            "completed_at": "2025-01-15T10:30:05+00:00",
            "duration_ms": 1500,
        }

        result = JobResult.from_dict(data)
        assert result.result == {"output": "value"}

    def test_repr(self):
        """Test result string representation."""
        result = JobResult(
            job_id="test-id",
            status=JobStatus.COMPLETED,
            duration_ms=1500,
        )
        repr_str = repr(result)

        assert "JobResult(" in repr_str
        assert "test-id" in repr_str
        assert "completed" in repr_str
        assert "1500" in repr_str

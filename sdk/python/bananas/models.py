"""
Bananas data models and enums.

This module provides the core data structures for the Bananas task queue system,
including Job, JobResult, and related enums.
"""

import json
from datetime import datetime, timezone
from enum import Enum
from typing import Any, Dict, Optional
from uuid import uuid4


class JobStatus(str, Enum):
    """Job status enumeration.

    Attributes:
        PENDING: Job is waiting to be processed
        PROCESSING: Job is currently being processed
        COMPLETED: Job was successfully completed
        FAILED: Job failed and will not be retried
        SCHEDULED: Job is scheduled for future execution
    """
    PENDING = "pending"
    PROCESSING = "processing"
    COMPLETED = "completed"
    FAILED = "failed"
    SCHEDULED = "scheduled"


class JobPriority(str, Enum):
    """Job priority enumeration.

    Attributes:
        HIGH: High priority jobs that should be processed first
        NORMAL: Normal priority jobs
        LOW: Low priority jobs that can be processed later
    """
    HIGH = "high"
    NORMAL = "normal"
    LOW = "low"


class Job:
    """Represents a unit of work to be processed by the task queue.

    Attributes:
        id: Unique identifier for the job
        name: Identifies the kind of job to be executed
        description: Optional human-readable description
        payload: Job-specific data as a dictionary
        status: Current status of the job
        priority: Processing order priority
        created_at: When the job was created
        updated_at: When the job was last updated
        scheduled_for: When a scheduled job should execute (optional)
        attempts: Number of times the job has been attempted
        max_retries: Maximum number of retry attempts allowed
        error: Error message if the job failed
        routing_key: Optional routing key for directed task routing
    """

    def __init__(
        self,
        name: str,
        payload: Dict[str, Any],
        priority: JobPriority,
        description: str = "",
        job_id: Optional[str] = None,
        status: JobStatus = JobStatus.PENDING,
        created_at: Optional[datetime] = None,
        updated_at: Optional[datetime] = None,
        scheduled_for: Optional[datetime] = None,
        attempts: int = 0,
        max_retries: int = 3,
        error: str = "",
        routing_key: str = "",
    ):
        """Initialize a new Job.

        Args:
            name: Job name (identifies the handler)
            payload: Job data as a dictionary
            priority: Job priority level
            description: Optional description
            job_id: Unique ID (auto-generated if not provided)
            status: Initial status (default: PENDING)
            created_at: Creation timestamp (auto-set if not provided)
            updated_at: Update timestamp (auto-set if not provided)
            scheduled_for: Scheduled execution time (optional)
            attempts: Number of attempts (default: 0)
            max_retries: Maximum retry attempts (default: 3)
            error: Error message (default: empty)
            routing_key: Routing key for task routing (default: empty)
        """
        now = datetime.now(timezone.utc)

        self.id = job_id or str(uuid4())
        self.name = name
        self.description = description
        self.payload = payload
        self.status = status
        self.priority = priority
        self.created_at = created_at or now
        self.updated_at = updated_at or now
        self.scheduled_for = scheduled_for
        self.attempts = attempts
        self.max_retries = max_retries
        self.error = error
        self.routing_key = routing_key

    def update_status(self, status: JobStatus) -> None:
        """Update the job's status and update timestamp.

        Args:
            status: New job status
        """
        self.status = status
        self.updated_at = datetime.now(timezone.utc)

    def to_dict(self) -> Dict[str, Any]:
        """Convert job to dictionary for JSON serialization.

        Returns:
            Dictionary representation of the job
        """
        data = {
            "id": self.id,
            "name": self.name,
            "description": self.description,
            "payload": self.payload,
            "status": self.status.value,
            "priority": self.priority.value,
            "created_at": self.created_at.isoformat(),
            "updated_at": self.updated_at.isoformat(),
            "attempts": self.attempts,
            "max_retries": self.max_retries,
            "error": self.error,
            "routing_key": self.routing_key,
        }

        if self.scheduled_for:
            data["scheduled_for"] = self.scheduled_for.isoformat()

        return data

    def to_json(self) -> str:
        """Convert job to JSON string.

        Returns:
            JSON string representation of the job
        """
        return json.dumps(self.to_dict())

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Job":
        """Create a Job from a dictionary.

        Args:
            data: Dictionary containing job data

        Returns:
            Job instance
        """
        scheduled_for = None
        if data.get("scheduled_for"):
            scheduled_for = datetime.fromisoformat(data["scheduled_for"].replace("Z", "+00:00"))

        return cls(
            name=data["name"],
            payload=data["payload"],
            priority=JobPriority(data["priority"]),
            description=data.get("description", ""),
            job_id=data["id"],
            status=JobStatus(data["status"]),
            created_at=datetime.fromisoformat(data["created_at"].replace("Z", "+00:00")),
            updated_at=datetime.fromisoformat(data["updated_at"].replace("Z", "+00:00")),
            scheduled_for=scheduled_for,
            attempts=data.get("attempts", 0),
            max_retries=data.get("max_retries", 3),
            error=data.get("error", ""),
            routing_key=data.get("routing_key", ""),
        )

    @classmethod
    def from_json(cls, json_str: str) -> "Job":
        """Create a Job from a JSON string.

        Args:
            json_str: JSON string containing job data

        Returns:
            Job instance
        """
        data = json.loads(json_str)
        return cls.from_dict(data)

    def __repr__(self) -> str:
        """Return string representation of the job."""
        return (
            f"Job(id={self.id!r}, name={self.name!r}, status={self.status.value!r}, "
            f"priority={self.priority.value!r})"
        )


class JobResult:
    """Represents the result of a completed job.

    Attributes:
        job_id: Unique identifier of the job
        status: Final status (completed or failed)
        result: Job's return value (for successful jobs)
        error: Error message (for failed jobs)
        completed_at: When the job finished executing
        duration_ms: How long the job took to execute (milliseconds)
    """

    def __init__(
        self,
        job_id: str,
        status: JobStatus,
        result: Optional[Dict[str, Any]] = None,
        error: str = "",
        completed_at: Optional[datetime] = None,
        duration_ms: int = 0,
    ):
        """Initialize a new JobResult.

        Args:
            job_id: Unique job identifier
            status: Final job status
            result: Result data for successful jobs
            error: Error message for failed jobs
            completed_at: Completion timestamp
            duration_ms: Execution duration in milliseconds
        """
        self.job_id = job_id
        self.status = status
        self.result = result
        self.error = error
        self.completed_at = completed_at or datetime.now(timezone.utc)
        self.duration_ms = duration_ms

    def is_success(self) -> bool:
        """Check if the job completed successfully.

        Returns:
            True if the job completed successfully
        """
        return self.status == JobStatus.COMPLETED

    def is_failed(self) -> bool:
        """Check if the job failed.

        Returns:
            True if the job failed
        """
        return self.status == JobStatus.FAILED

    def to_dict(self) -> Dict[str, Any]:
        """Convert result to dictionary.

        Returns:
            Dictionary representation of the result
        """
        data = {
            "job_id": self.job_id,
            "status": self.status.value,
            "completed_at": self.completed_at.isoformat(),
            "duration_ms": self.duration_ms,
        }

        if self.result:
            data["result"] = self.result

        if self.error:
            data["error"] = self.error

        return data

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "JobResult":
        """Create a JobResult from a dictionary.

        Args:
            data: Dictionary containing result data

        Returns:
            JobResult instance
        """
        completed_at = None
        if data.get("completed_at"):
            completed_at = datetime.fromisoformat(data["completed_at"].replace("Z", "+00:00"))

        result = data.get("result")
        if result and isinstance(result, str):
            try:
                result = json.loads(result)
            except json.JSONDecodeError:
                pass

        return cls(
            job_id=data["job_id"],
            status=JobStatus(data["status"]),
            result=result,
            error=data.get("error", ""),
            completed_at=completed_at,
            duration_ms=int(data.get("duration_ms", 0)),
        )

    def __repr__(self) -> str:
        """Return string representation of the result."""
        return (
            f"JobResult(job_id={self.job_id!r}, status={self.status.value!r}, "
            f"duration_ms={self.duration_ms})"
        )

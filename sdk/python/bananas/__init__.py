"""
Bananas - Distributed Task Queue Client Library.

A Python client for the Bananas distributed task queue system.
Provides a simple API for submitting jobs, retrieving results,
and synchronous job execution.

Example:
    >>> from bananas import Client, JobPriority
    >>>
    >>> # Create client
    >>> client = Client("redis://localhost:6379/0")
    >>>
    >>> # Submit a job
    >>> job_id = client.submit_job(
    ...     "send_email",
    ...     {"to": "user@example.com", "subject": "Hello"},
    ...     JobPriority.HIGH
    ... )
    >>>
    >>> # Get result
    >>> result = client.get_result(job_id)
    >>> if result and result.is_success():
    ...     print(result.result)
    >>>
    >>> # Or use submit_and_wait for RPC-style execution
    >>> result = client.submit_and_wait(
    ...     "generate_report",
    ...     {"report_type": "sales"},
    ...     JobPriority.NORMAL
    ... )
    >>>
    >>> client.close()

Context Manager Usage:
    >>> with Client("redis://localhost:6379/0") as client:
    ...     result = client.submit_and_wait("process_data", {...}, JobPriority.HIGH)
"""

from .client import Client
from .exceptions import (
    BananasError,
    ConnectionError,
    InvalidJobError,
    JobNotFoundError,
    ResultNotFoundError,
    SerializationError,
    TimeoutError,
)
from .models import Job, JobPriority, JobResult, JobStatus

__version__ = "0.1.0"

__all__ = [
    # Main client
    "Client",
    # Models
    "Job",
    "JobResult",
    "JobStatus",
    "JobPriority",
    # Exceptions
    "BananasError",
    "ConnectionError",
    "JobNotFoundError",
    "ResultNotFoundError",
    "TimeoutError",
    "SerializationError",
    "InvalidJobError",
]

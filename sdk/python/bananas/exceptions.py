"""
Bananas exceptions.

This module defines custom exceptions for the Bananas client library.
"""


class BananasError(Exception):
    """Base exception for all Bananas errors."""
    pass


class ConnectionError(BananasError):
    """Raised when there's a problem connecting to Redis."""
    pass


class JobNotFoundError(BananasError):
    """Raised when a job cannot be found."""
    pass


class ResultNotFoundError(BananasError):
    """Raised when a job result cannot be found or has expired."""
    pass


class TimeoutError(BananasError):
    """Raised when an operation times out."""
    pass


class SerializationError(BananasError):
    """Raised when there's a problem serializing or deserializing data."""
    pass


class InvalidJobError(BananasError):
    """Raised when job data is invalid."""
    pass

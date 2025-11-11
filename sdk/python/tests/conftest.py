"""
Pytest configuration and fixtures.
"""

import pytest
from fakeredis import FakeRedis, FakeStrictRedis


@pytest.fixture
def redis_url():
    """Redis URL for testing."""
    return "redis://localhost:6379/0"


@pytest.fixture
def fake_redis():
    """Fake Redis client for testing."""
    return FakeStrictRedis(decode_responses=False)


@pytest.fixture
def mock_redis_connection(monkeypatch, fake_redis):
    """Mock Redis connection to use fake_redis."""
    import redis

    def mock_from_url(*args, **kwargs):
        return fake_redis

    monkeypatch.setattr(redis, "from_url", mock_from_url)
    return fake_redis

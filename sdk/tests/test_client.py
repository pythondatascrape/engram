"""Tests for the Engram client."""

import pytest
from engram.client import Engram
from engram.codebook import Codebook, Enum, Range
from engram.errors import EngramError


class TestCB(Codebook):
    __codebook_name__ = "test"
    __codebook_version__ = 1
    cuisine: str = Enum(values=["italian", "thai"], required=True)


def test_client_rejects_invalid_mode():
    with pytest.raises(EngramError):
        Engram(mode="invalid")


def test_client_embedded_mode_creates():
    app = Engram(mode="embedded", auto_discover=False)
    assert app.mode == "embedded"


def test_client_hosted_mode_creates():
    app = Engram(mode="hosted", url="https://example.com", api_key="key", auto_discover=False)
    assert app.mode == "hosted"


def test_client_accepts_plugins():
    from engram.plugins.middleware import Middleware

    class MW(Middleware):
        order = 1
        name = "test_mw"

    app = Engram(mode="embedded", plugins=[MW()], auto_discover=False)
    assert len(app._plugins["middleware"]) == 1


@pytest.mark.asyncio
async def test_embedded_session_raises_not_implemented():
    """Embedded engine is a stub — session.query() should raise NotImplementedError."""
    app = Engram(mode="embedded", auto_discover=False)
    identity = TestCB(cuisine="thai")
    async with app.session(identity=identity, provider="test", model="m") as session:
        with pytest.raises(NotImplementedError):
            await session.query("hello")


@pytest.mark.asyncio
async def test_hosted_session_raises_not_implemented():
    app = Engram(mode="hosted", url="https://example.com", api_key="key", auto_discover=False)
    identity = TestCB(cuisine="thai")
    async with app.session(identity=identity, provider="test", model="m") as session:
        with pytest.raises(NotImplementedError):
            await session.query("hello")

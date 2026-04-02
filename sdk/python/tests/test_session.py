"""Tests for Session class."""

import asyncio
import pytest
from unittest.mock import AsyncMock, MagicMock

from engram.session import Session
from engram.codebook import Codebook, Enum, Range
from engram.models import RequestContext, Response, SessionInfo
from engram.errors import SessionError


class TestCB(Codebook):
    __codebook_name__ = "test"
    __codebook_version__ = 1
    cuisine: str = Enum(values=["italian", "thai"], required=True)
    party_size: int = Range(min=1, max=20, required=True)


@pytest.mark.asyncio
async def test_session_caches_identity():
    """First query serializes identity; subsequent queries reuse cached value."""
    identity = TestCB(cuisine="thai", party_size=4)
    engine = AsyncMock()
    engine.execute = AsyncMock(
        return_value=Response(text="ok", provider="test", model="m")
    )

    session = Session(
        identity=identity,
        provider="test",
        model="m",
        engine=engine,
        middlewares=[],
        hooks=[],
    )

    await session.query("first")
    await session.query("second")

    # Engine called twice
    assert engine.execute.call_count == 2
    # Both calls should have the same serialized identity
    ctx1 = engine.execute.call_args_list[0][0][0]
    ctx2 = engine.execute.call_args_list[1][0][0]
    assert ctx1.serialized_identity == ctx2.serialized_identity
    assert ctx1.serialized_identity == "cuisine=thai party_size=4"


@pytest.mark.asyncio
async def test_session_query_closed_raises():
    identity = TestCB(cuisine="thai", party_size=4)
    engine = AsyncMock()

    session = Session(
        identity=identity, provider="test", model="m",
        engine=engine, middlewares=[], hooks=[],
    )
    await session.close()

    with pytest.raises(SessionError):
        await session.query("should fail")


@pytest.mark.asyncio
async def test_session_context_manager():
    identity = TestCB(cuisine="thai", party_size=4)
    engine = AsyncMock()
    engine.execute = AsyncMock(
        return_value=Response(text="ok", provider="test", model="m")
    )

    session = Session(
        identity=identity, provider="test", model="m",
        engine=engine, middlewares=[], hooks=[],
    )

    async with session:
        result = await session.query("hello")
        assert result.text == "ok"

    # Session should be closed after context manager
    with pytest.raises(SessionError):
        await session.query("should fail")


@pytest.mark.asyncio
async def test_session_concurrent_queries_serialized():
    """Concurrent queries should be serialized via lock (no interleaving)."""
    identity = TestCB(cuisine="thai", party_size=4)
    order = []

    async def slow_execute(ctx):
        order.append(f"start-{ctx.query}")
        await asyncio.sleep(0.01)
        order.append(f"end-{ctx.query}")
        return Response(text=ctx.query, provider="test", model="m")

    engine = AsyncMock()
    engine.execute = slow_execute

    session = Session(
        identity=identity, provider="test", model="m",
        engine=engine, middlewares=[], hooks=[],
    )

    await asyncio.gather(session.query("a"), session.query("b"))

    # With lock, we should see start-X, end-X, start-Y, end-Y (no interleaving)
    assert order[0].startswith("start-")
    assert order[1].startswith("end-")
    assert order[2].startswith("start-")
    assert order[3].startswith("end-")

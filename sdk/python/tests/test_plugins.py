"""Tests for plugin base classes."""

import pytest
import asyncio
from engram.plugins.middleware import Middleware
from engram.plugins.hook import Hook, on
from engram.plugins.provider import ProviderAdapter
from engram.models import RequestContext, Response, Chunk, Capabilities, SessionInfo


# --- Middleware ---

class TestMiddleware(Middleware):
    order = 5

    async def before_request(self, ctx: RequestContext) -> RequestContext | Response:
        ctx.metadata["seen_by"] = "TestMiddleware"
        return ctx

    async def after_response(self, ctx: RequestContext, response: Response) -> Response:
        return response


def test_middleware_has_order():
    mw = TestMiddleware()
    assert mw.order == 5


@pytest.mark.asyncio
async def test_middleware_before_request():
    mw = TestMiddleware()
    ctx = RequestContext(
        query="test", serialized_identity="a=1",
        session_id="s1", provider="p", model="m",
    )
    result = await mw.before_request(ctx)
    assert isinstance(result, RequestContext)
    assert result.metadata["seen_by"] == "TestMiddleware"


# --- Hook ---

class TestHook(Hook):
    events_fired: list = []

    @on("session.create")
    async def on_create(self, session: SessionInfo):
        self.events_fired.append("session.create")

    @on("query.after")
    async def on_query(self, ctx: RequestContext, response: Response):
        self.events_fired.append("query.after")


def test_hook_event_registration():
    h = TestHook()
    handlers = h._get_handlers()
    assert "session.create" in handlers
    assert "query.after" in handlers


@pytest.mark.asyncio
async def test_hook_dispatch():
    h = TestHook()
    h.events_fired = []
    info = SessionInfo(session_id="s1", provider="p", model="m")
    await h.dispatch("session.create", info)
    assert "session.create" in h.events_fired


@pytest.mark.asyncio
async def test_hook_errors_are_swallowed():
    """Hook errors must not propagate."""

    class FailHook(Hook):
        @on("error")
        async def on_error(self, ctx, error):
            raise RuntimeError("hook crashed")

    h = FailHook()
    ctx = RequestContext(
        query="test", serialized_identity="a=1",
        session_id="s1", provider="p", model="m",
    )
    # Should not raise
    await h.dispatch("error", ctx, Exception("original"))


# --- ProviderAdapter ---

def test_provider_adapter_has_name():
    class TestProvider(ProviderAdapter):
        name = "test-llm"

        async def send(self, request):
            yield Chunk(text="hi", index=0, finished=True)

        async def healthcheck(self) -> bool:
            return True

        def capabilities(self) -> Capabilities:
            return Capabilities(
                models=["test"], max_context_window=4096, supports_streaming=True
            )

    p = TestProvider()
    assert p.name == "test-llm"


# --- Sorting ---

def test_middleware_sorting():
    class M1(Middleware):
        order = 10

    class M2(Middleware):
        order = 1

    middlewares = sorted([M1(), M2()], key=lambda m: m.order)
    assert middlewares[0].order == 1

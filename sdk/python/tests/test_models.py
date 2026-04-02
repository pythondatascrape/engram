"""Tests for pipeline models."""

from engram.models import RequestContext, Response, Chunk, Capabilities, SessionInfo


def test_request_context_creation():
    ctx = RequestContext(
        query="What do you recommend?",
        serialized_identity="cuisine=thai party_size=4",
        session_id="sess-123",
        provider="openai",
        model="gpt-4o",
    )
    assert ctx.query == "What do you recommend?"
    assert ctx.metadata == {}


def test_request_context_metadata():
    ctx = RequestContext(
        query="test",
        serialized_identity="a=1",
        session_id="s1",
        provider="openai",
        model="gpt-4o",
        metadata={"cache_key": "abc"},
    )
    assert ctx.metadata["cache_key"] == "abc"


def test_response_creation():
    r = Response(text="Try the pad thai", provider="openai", model="gpt-4o")
    assert r.text == "Try the pad thai"


def test_response_with_metadata():
    r = Response(
        text="answer",
        provider="openai",
        model="gpt-4o",
        usage={"input_tokens": 100, "output_tokens": 50},
    )
    assert r.usage["input_tokens"] == 100


def test_chunk_creation():
    c = Chunk(text="partial", index=0, finished=False)
    assert c.finished is False


def test_capabilities():
    cap = Capabilities(
        models=["gpt-4o", "gpt-4o-mini"],
        max_context_window=128000,
        supports_streaming=True,
    )
    assert "gpt-4o" in cap.models


def test_session_info():
    info = SessionInfo(
        session_id="sess-abc",
        provider="openai",
        model="gpt-4o",
    )
    assert info.session_id == "sess-abc"

"""Tests for plugin discovery and merge."""

import pytest
from unittest.mock import patch, MagicMock

from engram.discovery import discover_plugins, merge_plugins
from engram.plugins.middleware import Middleware
from engram.plugins.hook import Hook, on
from engram.plugins.provider import ProviderAdapter
from engram.models import Chunk, Capabilities
from engram.errors import PluginError


class MW1(Middleware):
    order = 1
    name = "mw1"


class MW2(Middleware):
    order = 2
    name = "mw2"


class H1(Hook):
    name = "h1"

    @on("session.create")
    async def on_create(self, session):
        pass


class P1(ProviderAdapter):
    name = "provider1"

    async def send(self, request):
        yield Chunk(text="x", index=0, finished=True)

    async def healthcheck(self):
        return True

    def capabilities(self):
        return Capabilities(models=["m"], max_context_window=4096, supports_streaming=True)


def test_merge_sorts_middleware_by_order():
    result = merge_plugins(explicit=[MW2(), MW1()])
    middlewares = result["middleware"]
    assert middlewares[0].order <= middlewares[1].order


def test_merge_separates_types():
    result = merge_plugins(explicit=[MW1(), H1(), P1()])
    assert len(result["middleware"]) == 1
    assert len(result["hook"]) == 1
    assert len(result["provider"]) == 1


def test_merge_duplicate_names_raises():
    with pytest.raises(PluginError, match="duplicate"):
        merge_plugins(explicit=[MW1(), MW1()])


def test_discover_with_auto_discover_false():
    """auto_discover=False skips entry points."""
    result = discover_plugins(auto_discover=False)
    assert result == []


def test_merge_empty():
    result = merge_plugins(explicit=[])
    assert result["middleware"] == []
    assert result["hook"] == []
    assert result["provider"] == []

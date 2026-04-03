"""Tests for the daemon-backed Engram client."""

import json
import asyncio
import os
import tempfile
import pytest

from engram import Engram, EngramError


class FakeDaemon:
    def __init__(self, socket_path: str):
        self.socket_path = socket_path
        self._server = None
        self._responses: dict[str, dict] = {}

    def set_response(self, method: str, result: dict):
        self._responses[method] = result

    async def _handle(self, reader: asyncio.StreamReader, writer: asyncio.StreamWriter):
        while True:
            line = await reader.readline()
            if not line:
                break
            request = json.loads(line)
            method = request["method"]
            result = self._responses.get(method, {})
            response = {"jsonrpc": "2.0", "id": request["id"], "result": result}
            writer.write(json.dumps(response).encode() + b"\n")
            await writer.drain()
        writer.close()

    async def start(self):
        self._server = await asyncio.start_unix_server(self._handle, path=self.socket_path)

    async def stop(self):
        if self._server:
            self._server.close()
            await self._server.wait_closed()


@pytest.fixture
async def daemon():
    with tempfile.TemporaryDirectory() as tmpdir:
        sock = os.path.join(tmpdir, "engram.sock")
        d = FakeDaemon(sock)
        d.set_response("engram.compress", {
            "compressed": "c:expert|t:formal",
            "original_tokens": 500,
            "compressed_tokens": 12,
        })
        d.set_response("engram.deriveCodebook", {
            "dimensions": [
                {"key": "expertise", "type": "enum", "values": ["novice", "expert"]},
            ],
        })
        d.set_response("engram.getStats", {
            "sessions": 1,
            "total_tokens_saved": 488,
            "compression_ratio": 0.976,
        })
        d.set_response("engram.checkRedundancy", {
            "redundant": False,
            "patterns": [],
        })
        d.set_response("engram.generateReport", {
            "report": "Session saved 488 tokens (97.6%)",
        })
        await d.start()
        yield sock, d
        await d.stop()


@pytest.mark.asyncio
async def test_compress(daemon):
    sock, _ = daemon
    client = await Engram.connect(sock)
    result = await client.compress({"identity": "test", "history": [], "query": "hello"})
    assert result["compressed"] == "c:expert|t:formal"
    assert result["original_tokens"] == 500
    await client.close()


@pytest.mark.asyncio
async def test_derive_codebook(daemon):
    sock, _ = daemon
    client = await Engram.connect(sock)
    result = await client.derive_codebook("some content about expertise")
    assert len(result["dimensions"]) == 1
    assert result["dimensions"][0]["key"] == "expertise"
    await client.close()


@pytest.mark.asyncio
async def test_get_stats(daemon):
    sock, _ = daemon
    client = await Engram.connect(sock)
    result = await client.get_stats()
    assert result["total_tokens_saved"] == 488
    await client.close()


@pytest.mark.asyncio
async def test_check_redundancy(daemon):
    sock, _ = daemon
    client = await Engram.connect(sock)
    result = await client.check_redundancy("some content")
    assert result["redundant"] is False
    await client.close()


@pytest.mark.asyncio
async def test_generate_report(daemon):
    sock, _ = daemon
    client = await Engram.connect(sock)
    result = await client.generate_report()
    assert "488" in result["report"]
    await client.close()


@pytest.mark.asyncio
async def test_connect_missing_socket():
    with pytest.raises(EngramError):
        await Engram.connect("/tmp/nonexistent-engram-test.sock")


@pytest.mark.asyncio
async def test_context_manager(daemon):
    sock, _ = daemon
    async with await Engram.connect(sock) as client:
        result = await client.get_stats()
        assert result["sessions"] == 1

"""Engram client — thin wrapper over the daemon's Unix socket JSON-RPC API."""

import asyncio
import json
import os
from typing import Any, Optional

from engram.errors import EngramError, EngramConnectionError


_DEFAULT_SOCKET = os.path.expanduser("~/.engram/engram.sock")
_REQUEST_ID = 0


def _next_id() -> int:
    global _REQUEST_ID
    _REQUEST_ID += 1
    return _REQUEST_ID


class Engram:
    """Async client for the Engram compression daemon.

    Holds a persistent Unix socket connection across calls.

    Usage:
        async with await Engram.connect() as client:
            result = await client.compress(...)
    """

    def __init__(self, reader: asyncio.StreamReader, writer: asyncio.StreamWriter):
        self._reader = reader
        self._writer = writer

    @classmethod
    async def connect(cls, socket_path: Optional[str] = None) -> "Engram":
        path = socket_path or _DEFAULT_SOCKET
        try:
            reader, writer = await asyncio.open_unix_connection(path)
        except (ConnectionRefusedError, FileNotFoundError, OSError) as e:
            raise EngramError(f"daemon not reachable: {e}") from e

        return cls(reader, writer)

    async def _call(self, method: str, params: Optional[dict[str, Any]] = None) -> dict[str, Any]:
        if self._writer is None:
            raise EngramError("client is closed")

        request = {
            "jsonrpc": "2.0",
            "id": _next_id(),
            "method": method,
            "params": params or {},
        }

        try:
            self._writer.write(json.dumps(request).encode() + b"\n")
            await self._writer.drain()

            line = await self._reader.readline()
            if not line:
                raise EngramConnectionError("daemon closed connection without response")

            response = json.loads(line)

            if "error" in response:
                err = response["error"]
                raise EngramError(f"daemon error ({err.get('code', '?')}): {err.get('message', 'unknown')}")

            return response.get("result", {})
        except (ConnectionResetError, BrokenPipeError, OSError) as e:
            raise EngramConnectionError(f"connection lost: {e}") from e

    async def compress(self, context: dict[str, Any]) -> dict[str, Any]:
        return await self._call("engram.compress", context)

    async def derive_codebook(self, content: str) -> dict[str, Any]:
        return await self._call("engram.deriveCodebook", {"content": content})

    async def get_stats(self) -> dict[str, Any]:
        return await self._call("engram.getStats")

    async def check_redundancy(self, content: str) -> dict[str, Any]:
        return await self._call("engram.checkRedundancy", {"content": content})

    async def generate_report(self) -> dict[str, Any]:
        return await self._call("engram.generateReport")

    async def close(self) -> None:
        if self._writer is not None:
            self._writer.close()
            await self._writer.wait_closed()
            self._writer = None

    async def __aenter__(self) -> "Engram":
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb) -> None:
        await self.close()

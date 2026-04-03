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

    Usage:
        client = await Engram.connect()
        result = await client.compress({"identity": ..., "history": ..., "query": ...})
        await client.close()

    Or as a context manager:
        async with await Engram.connect() as client:
            result = await client.compress(...)
    """

    def __init__(self, socket_path: str):
        self._socket_path = socket_path
        self._connected = False

    @classmethod
    async def connect(cls, socket_path: Optional[str] = None) -> "Engram":
        path = socket_path or _DEFAULT_SOCKET
        # Verify connectivity by opening and immediately closing a connection.
        try:
            reader, writer = await asyncio.open_unix_connection(path)
            writer.close()
            await writer.wait_closed()
        except (ConnectionRefusedError, FileNotFoundError, OSError) as e:
            raise EngramError(f"daemon not reachable: {e}") from e

        client = cls(path)
        client._connected = True
        return client

    async def _call(self, method: str, params: Optional[dict[str, Any]] = None) -> dict[str, Any]:
        if not self._connected:
            raise EngramError("client is not connected")

        request = {
            "jsonrpc": "2.0",
            "id": _next_id(),
            "method": method,
            "params": params or {},
        }

        try:
            reader, writer = await asyncio.open_unix_connection(self._socket_path)
        except (ConnectionRefusedError, FileNotFoundError, OSError) as e:
            raise EngramConnectionError(f"failed to connect to daemon: {e}") from e

        try:
            writer.write(json.dumps(request).encode())
            await writer.drain()

            data = await reader.read(1 << 20)
            if not data:
                raise EngramConnectionError("daemon closed connection without response")

            response = json.loads(data.decode())

            if "error" in response:
                err = response["error"]
                raise EngramError(f"daemon error ({err.get('code', '?')}): {err.get('message', 'unknown')}")

            return response.get("result", {})
        finally:
            writer.close()
            await writer.wait_closed()

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
        self._connected = False

    async def __aenter__(self) -> "Engram":
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb) -> None:
        await self.close()

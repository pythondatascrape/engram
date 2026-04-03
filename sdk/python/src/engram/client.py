"""Engram client — thin wrapper over the daemon's Unix socket JSON-RPC API."""

import asyncio
import json
import os
from typing import Any, Optional

from engram.errors import EngramError, EngramConnectionError


_DEFAULT_SOCKET = os.path.expanduser("~/.engram/engram.sock")
_DEFAULT_POOL_SIZE = 4
_REQUEST_ID = 0


def _next_id() -> int:
    global _REQUEST_ID
    _REQUEST_ID += 1
    return _REQUEST_ID


class _Conn:
    """A single persistent daemon connection."""
    __slots__ = ("reader", "writer")

    def __init__(self, reader: asyncio.StreamReader, writer: asyncio.StreamWriter):
        self.reader = reader
        self.writer = writer

    async def close(self):
        self.writer.close()
        await self.writer.wait_closed()


class Engram:
    """Async client for the Engram compression daemon.

    Maintains a pool of persistent connections for concurrent calls.

    Usage:
        async with await Engram.connect() as client:
            result = await client.compress(...)
    """

    def __init__(self, socket_path: str, pool: asyncio.Queue):
        self._socket_path = socket_path
        self._pool = pool
        self._closed = False

    @classmethod
    async def connect(cls, socket_path: Optional[str] = None, pool_size: int = _DEFAULT_POOL_SIZE) -> "Engram":
        path = socket_path or _DEFAULT_SOCKET
        # Seed pool with one connection to verify reachability.
        try:
            first = await cls._dial(path)
        except (ConnectionRefusedError, FileNotFoundError, OSError) as e:
            raise EngramError(f"daemon not reachable: {e}") from e

        pool: asyncio.Queue[Optional[_Conn]] = asyncio.Queue(maxsize=pool_size)
        await pool.put(first)
        return cls(path, pool)

    @staticmethod
    async def _dial(path: str) -> _Conn:
        reader, writer = await asyncio.open_unix_connection(path)
        return _Conn(reader, writer)

    async def _get(self) -> _Conn:
        # Try to grab an idle connection without blocking.
        try:
            return self._pool.get_nowait()
        except asyncio.QueueEmpty:
            pass
        # Pool empty — dial a new one.
        try:
            return await self._dial(self._socket_path)
        except (ConnectionRefusedError, FileNotFoundError, OSError) as e:
            raise EngramConnectionError(f"failed to connect to daemon: {e}") from e

    def _put(self, conn: _Conn):
        try:
            self._pool.put_nowait(conn)
        except asyncio.QueueFull:
            # Pool at capacity — close excess connection.
            asyncio.ensure_future(conn.close())

    async def _call(self, method: str, params: Optional[dict[str, Any]] = None) -> dict[str, Any]:
        if self._closed:
            raise EngramError("client is closed")

        cn = await self._get()

        request = {
            "jsonrpc": "2.0",
            "id": _next_id(),
            "method": method,
            "params": params or {},
        }

        try:
            cn.writer.write(json.dumps(request).encode() + b"\n")
            await cn.writer.drain()

            line = await cn.reader.readline()
            if not line:
                raise EngramConnectionError("daemon closed connection without response")

            response = json.loads(line)
        except (ConnectionResetError, BrokenPipeError, OSError) as e:
            # Connection is dead — don't return it to the pool.
            raise EngramConnectionError(f"connection lost: {e}") from e

        self._put(cn)

        if "error" in response:
            err = response["error"]
            raise EngramError(f"daemon error ({err.get('code', '?')}): {err.get('message', 'unknown')}")

        return response.get("result", {})

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
        self._closed = True
        while not self._pool.empty():
            try:
                cn = self._pool.get_nowait()
                await cn.close()
            except asyncio.QueueEmpty:
                break

    async def __aenter__(self) -> "Engram":
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb) -> None:
        await self.close()

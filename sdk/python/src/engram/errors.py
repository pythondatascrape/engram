"""Engram exception hierarchy."""


class EngramError(Exception):
    """Base exception for all Engram errors."""


class EngramConnectionError(EngramError):
    """Raised when the client cannot connect to the daemon."""

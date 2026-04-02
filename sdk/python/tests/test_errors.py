"""Tests for the engram error hierarchy."""

from engram.errors import (
    EngramError,
    CodebookValidationError,
    SerializationError,
    ProviderError,
    SessionError,
    PluginError,
    EngramConnectionError,
)


def test_base_error_is_exception():
    assert issubclass(EngramError, Exception)


def test_all_errors_inherit_from_base():
    for cls in [
        CodebookValidationError,
        SerializationError,
        ProviderError,
        SessionError,
        PluginError,
        EngramConnectionError,
    ]:
        assert issubclass(cls, EngramError), f"{cls.__name__} must subclass EngramError"


def test_error_message_preserved():
    err = CodebookValidationError("bad field")
    assert str(err) == "bad field"


def test_error_can_be_caught_as_base():
    try:
        raise ProviderError("timeout")
    except EngramError as e:
        assert str(e) == "timeout"

"""Tests for the Codebook base class and dimension types."""

import pytest
from engram.codebook import Codebook, Enum, Range, Scale, Boolean
from engram.errors import CodebookValidationError


# --- Dimension type construction ---

def test_enum_dimension():
    """Enum stores allowed values and required flag."""

    class CB(Codebook):
        __codebook_name__ = "test"
        __codebook_version__ = 1
        cuisine: str = Enum(values=["italian", "thai"], required=True)

    identity = CB(cuisine="thai")
    assert identity.cuisine == "thai"


def test_enum_rejects_invalid_value():
    class CB(Codebook):
        __codebook_name__ = "test"
        __codebook_version__ = 1
        cuisine: str = Enum(values=["italian", "thai"], required=True)

    with pytest.raises(CodebookValidationError):
        CB(cuisine="french")


def test_range_dimension():
    class CB(Codebook):
        __codebook_name__ = "test"
        __codebook_version__ = 1
        party_size: int = Range(min=1, max=20, required=True)

    identity = CB(party_size=4)
    assert identity.party_size == 4


def test_range_rejects_out_of_bounds():
    class CB(Codebook):
        __codebook_name__ = "test"
        __codebook_version__ = 1
        party_size: int = Range(min=1, max=20, required=True)

    with pytest.raises(CodebookValidationError):
        CB(party_size=25)


def test_scale_dimension():
    class CB(Codebook):
        __codebook_name__ = "test"
        __codebook_version__ = 1
        ambiance: int = Scale(min=1, max=5)

    identity = CB(ambiance=3)
    assert identity.ambiance == 3


def test_boolean_dimension():
    class CB(Codebook):
        __codebook_name__ = "test"
        __codebook_version__ = 1
        outdoor: bool = Boolean()

    identity = CB(outdoor=True)
    assert identity.outdoor is True


def test_optional_fields_default_to_none():
    class CB(Codebook):
        __codebook_name__ = "test"
        __codebook_version__ = 1
        cuisine: str = Enum(values=["italian", "thai"], required=True)
        outdoor: bool = Boolean()

    identity = CB(cuisine="thai")
    assert identity.outdoor is None


def test_required_field_missing_raises():
    class CB(Codebook):
        __codebook_name__ = "test"
        __codebook_version__ = 1
        cuisine: str = Enum(values=["italian", "thai"], required=True)

    with pytest.raises(CodebookValidationError):
        CB()


def test_to_dict_omits_none():
    class CB(Codebook):
        __codebook_name__ = "test"
        __codebook_version__ = 1
        cuisine: str = Enum(values=["italian", "thai"], required=True)
        outdoor: bool = Boolean()

    identity = CB(cuisine="thai")
    d = identity.to_dict()
    assert d == {"cuisine": "thai"}
    assert "outdoor" not in d


def test_multi_value_enum():
    """Multi-value enum accepts a list and stores it."""

    class CB(Codebook):
        __codebook_name__ = "test"
        __codebook_version__ = 1
        certs: list[str] = Enum(values=["hazmat", "rescue", "emt"], multi=True)

    identity = CB(certs=["hazmat", "emt"])
    assert identity.certs == ["hazmat", "emt"]


def test_multi_value_enum_rejects_invalid():
    class CB(Codebook):
        __codebook_name__ = "test"
        __codebook_version__ = 1
        certs: list[str] = Enum(values=["hazmat", "rescue", "emt"], multi=True)

    with pytest.raises(CodebookValidationError):
        CB(certs=["hazmat", "invalid"])


def test_codebook_dimensions_metadata():
    """Codebook exposes dimension metadata for serialization and YAML export."""

    class CB(Codebook):
        __codebook_name__ = "restaurant"
        __codebook_version__ = 1
        cuisine: str = Enum(values=["italian", "thai"], required=True)
        party_size: int = Range(min=1, max=20, required=True)

    dims = CB.get_dimensions()
    assert len(dims) == 2
    names = [d.name for d in dims]
    assert "cuisine" in names
    assert "party_size" in names

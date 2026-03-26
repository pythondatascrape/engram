"""Tests for the identity serializer."""

import pytest
from engram.codebook import Codebook, Enum, Range, Scale, Boolean
from engram.serializer import serialize
from engram.errors import SerializationError


class RestaurantCodebook(Codebook):
    __codebook_name__ = "restaurant"
    __codebook_version__ = 1
    ambiance: int = Scale(min=1, max=5)
    cuisine: str = Enum(values=["italian", "mexican", "thai"], required=True)
    dietary: str = Enum(values=["none", "vegetarian", "vegan", "gluten_free"])
    outdoor_seating: bool = Boolean()
    party_size: int = Range(min=1, max=20, required=True)
    price_range: str = Enum(values=["budget", "moderate", "upscale"], required=True)


def test_serialize_basic():
    identity = RestaurantCodebook(
        cuisine="thai", price_range="moderate", party_size=4
    )
    result = serialize(identity)
    assert result == "cuisine=thai party_size=4 price_range=moderate"


def test_serialize_all_fields():
    identity = RestaurantCodebook(
        ambiance=2,
        cuisine="thai",
        dietary="vegetarian",
        outdoor_seating=True,
        party_size=4,
        price_range="moderate",
    )
    result = serialize(identity)
    assert result == "ambiance=2 cuisine=thai dietary=vegetarian outdoor_seating=true party_size=4 price_range=moderate"


def test_serialize_sorted_keys():
    """Keys must be sorted alphabetically regardless of definition order."""
    identity = RestaurantCodebook(
        price_range="budget", cuisine="italian", party_size=2
    )
    result = serialize(identity)
    assert result == "cuisine=italian party_size=2 price_range=budget"


def test_serialize_omits_optional_none():
    identity = RestaurantCodebook(
        cuisine="mexican", party_size=10, price_range="upscale"
    )
    result = serialize(identity)
    assert "ambiance" not in result
    assert "dietary" not in result
    assert "outdoor_seating" not in result


def test_serialize_boolean_lowercase():
    identity = RestaurantCodebook(
        cuisine="thai", party_size=1, price_range="budget", outdoor_seating=False
    )
    result = serialize(identity)
    assert "outdoor_seating=false" in result


def test_serialize_multi_value_enum():
    class CB(Codebook):
        __codebook_name__ = "test"
        __codebook_version__ = 1
        certs: list[str] = Enum(values=["hazmat", "rescue", "emt"], multi=True, required=True)

    identity = CB(certs=["hazmat", "emt"])
    result = serialize(identity)
    assert result == "certs=emt,hazmat"  # values sorted within multi-value


def test_serialize_deterministic():
    """Same input always produces same output."""
    for _ in range(10):
        identity = RestaurantCodebook(
            cuisine="thai", party_size=4, price_range="moderate", ambiance=3
        )
        assert serialize(identity) == "ambiance=3 cuisine=thai party_size=4 price_range=moderate"


def test_codebook_serialize_method():
    """Codebook.serialize() is a convenience wrapper."""
    identity = RestaurantCodebook(
        cuisine="thai", party_size=4, price_range="moderate"
    )
    assert identity.serialize() == serialize(identity)

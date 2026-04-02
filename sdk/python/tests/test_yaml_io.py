"""Tests for YAML codebook import/export."""

import tempfile
from pathlib import Path

import pytest
import yaml

from engram.codebook import Codebook, Enum, Range, Scale, Boolean
from engram.yaml_io import to_yaml, from_yaml


class RestaurantCodebook(Codebook):
    __codebook_name__ = "restaurant"
    __codebook_version__ = 1
    cuisine: str = Enum(values=["italian", "mexican", "thai"], required=True)
    party_size: int = Range(min=1, max=20, required=True)
    ambiance: int = Scale(min=1, max=5)
    outdoor_seating: bool = Boolean()


def test_to_yaml_structure():
    """Exported YAML matches Go codebook format."""
    result = to_yaml(RestaurantCodebook)
    data = yaml.safe_load(result)
    assert data["name"] == "restaurant"
    assert data["version"] == 1
    assert len(data["dimensions"]) == 4
    # Dimensions sorted by name (matches Go convention)
    names = [d["name"] for d in data["dimensions"]]
    assert names == sorted(names)


def test_to_yaml_enum_dimension():
    result = to_yaml(RestaurantCodebook)
    data = yaml.safe_load(result)
    cuisine = next(d for d in data["dimensions"] if d["name"] == "cuisine")
    assert cuisine["type"] == "enum"
    assert cuisine["values"] == ["italian", "mexican", "thai"]
    assert cuisine["required"] is True


def test_to_yaml_range_dimension():
    result = to_yaml(RestaurantCodebook)
    data = yaml.safe_load(result)
    ps = next(d for d in data["dimensions"] if d["name"] == "party_size")
    assert ps["type"] == "range"
    assert ps["min"] == 1
    assert ps["max"] == 20


def test_to_yaml_file(tmp_path):
    """to_yaml can write to a file path."""
    path = tmp_path / "codebook.yaml"
    to_yaml(RestaurantCodebook, path=path)
    assert path.exists()
    data = yaml.safe_load(path.read_text())
    assert data["name"] == "restaurant"


def test_from_yaml_creates_codebook():
    """from_yaml returns a dynamic Codebook subclass."""
    yaml_str = to_yaml(RestaurantCodebook)
    DynCB = from_yaml(yaml_str)
    identity = DynCB(cuisine="thai", party_size=4)
    assert identity.cuisine == "thai"
    assert identity.party_size == 4


def test_from_yaml_file(tmp_path):
    path = tmp_path / "codebook.yaml"
    to_yaml(RestaurantCodebook, path=path)
    DynCB = from_yaml(path=path)
    identity = DynCB(cuisine="italian", party_size=2)
    assert identity.cuisine == "italian"


def test_round_trip_serialize():
    """Export -> import -> create identity -> serialize produces same result."""
    yaml_str = to_yaml(RestaurantCodebook)
    DynCB = from_yaml(yaml_str)

    original = RestaurantCodebook(cuisine="thai", party_size=4, ambiance=3)
    restored = DynCB(cuisine="thai", party_size=4, ambiance=3)

    assert original.serialize() == restored.serialize()


def test_from_yaml_validates():
    """Dynamic codebook still validates values."""
    yaml_str = to_yaml(RestaurantCodebook)
    DynCB = from_yaml(yaml_str)

    with pytest.raises(Exception):  # CodebookValidationError
        DynCB(cuisine="french", party_size=4)


def test_boolean_round_trip():
    yaml_str = to_yaml(RestaurantCodebook)
    data = yaml.safe_load(yaml_str)
    outdoor = next(d for d in data["dimensions"] if d["name"] == "outdoor_seating")
    assert outdoor["type"] == "boolean"
    assert outdoor.get("required") is False or "required" not in outdoor or outdoor["required"] is False

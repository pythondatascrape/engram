"""Integration test: full flow from codebook definition to serialization."""

import pytest
from engram import Codebook, Enum, Range, Scale, Boolean, serialize, Engram
from engram.yaml_io import to_yaml, from_yaml


class TravelCodebook(Codebook):
    __codebook_name__ = "travel"
    __codebook_version__ = 1
    destination: str = Enum(
        values=["beach", "mountain", "city", "countryside"], required=True
    )
    budget: str = Enum(values=["budget", "moderate", "luxury"], required=True)
    group_size: int = Range(min=1, max=30, required=True)
    adventure_level: int = Scale(min=1, max=10)
    family_friendly: bool = Boolean()
    activities: list[str] = Enum(
        values=["hiking", "swimming", "sightseeing", "dining"], multi=True
    )


def test_full_flow_define_serialize():
    """Define codebook -> create identity -> serialize -> verify format."""
    identity = TravelCodebook(
        destination="beach",
        budget="moderate",
        group_size=4,
        adventure_level=7,
        family_friendly=True,
        activities=["swimming", "dining"],
    )

    result = serialize(identity)

    # Verify sorted keys
    keys = [pair.split("=")[0] for pair in result.split(" ")]
    assert keys == sorted(keys)

    # Verify specific values
    assert "destination=beach" in result
    assert "budget=moderate" in result
    assert "group_size=4" in result
    assert "adventure_level=7" in result
    assert "family_friendly=true" in result
    assert "activities=dining,swimming" in result  # sorted multi-value


def test_full_flow_optional_omitted():
    """Optional fields not set should not appear in serialized output."""
    identity = TravelCodebook(
        destination="city", budget="luxury", group_size=2
    )
    result = serialize(identity)
    assert "adventure_level" not in result
    assert "family_friendly" not in result
    assert "activities" not in result
    assert result == "budget=luxury destination=city group_size=2"


def test_full_flow_yaml_round_trip():
    """Export -> import -> create identity -> serialize -> compare."""
    # Export
    yaml_str = to_yaml(TravelCodebook)

    # Import
    DynCB = from_yaml(yaml_str)

    # Create identical identities
    original = TravelCodebook(
        destination="mountain", budget="budget", group_size=6, adventure_level=9
    )
    restored = DynCB(
        destination="mountain", budget="budget", group_size=6, adventure_level=9
    )

    # Serialized output must match exactly
    assert serialize(original) == serialize(restored)


def test_full_flow_codebook_method_serialize():
    """identity.serialize() should produce same result as serialize(identity)."""
    identity = TravelCodebook(
        destination="countryside", budget="moderate", group_size=2
    )
    assert identity.serialize() == serialize(identity)


def test_full_flow_validation_rejects_bad_values():
    """Codebook validation catches bad values at construction time."""
    with pytest.raises(Exception):
        TravelCodebook(
            destination="space",  # not in values
            budget="moderate",
            group_size=4,
        )


def test_full_flow_to_dict():
    identity = TravelCodebook(
        destination="beach", budget="luxury", group_size=1, family_friendly=False
    )
    d = identity.to_dict()
    assert d["destination"] == "beach"
    assert d["family_friendly"] is False
    assert "adventure_level" not in d  # optional, not set
    assert "activities" not in d


@pytest.mark.asyncio
async def test_full_flow_client_session():
    """Engram client -> session -> query (hits NotImplementedError stub)."""
    app = Engram(mode="embedded", auto_discover=False)
    identity = TravelCodebook(
        destination="beach", budget="moderate", group_size=4
    )
    async with app.session(identity=identity, provider="test", model="m") as session:
        # Verify session has cached the serialized identity after context entry
        assert session.session_id is not None
        with pytest.raises(NotImplementedError):
            await session.query("What should I pack?")

"""Portable caller models and explicit synthetic entropy inputs."""

import math
import uuid
from contextlib import contextmanager
from datetime import datetime
from unittest.mock import patch


def decode_input(value):
    if isinstance(value, list):
        return [decode_input(item) for item in value]
    if not isinstance(value, dict):
        return value
    if "$datetime" in value:
        assert set(value) == {"$datetime"}
        return datetime.fromisoformat(value["$datetime"])
    if "$model" in value:
        from pyicloud.services.reminders import models

        assert set(value) == {"$model", "value"}
        assert value["$model"] in {
            "Reminder",
            "Hashtag",
            "URLAttachment",
            "ImageAttachment",
            "RecurrenceRule",
            "Alarm",
            "LocationTrigger",
        }
        return getattr(models, value["$model"]).model_validate(value["value"])
    return {key: decode_input(item) for key, item in value.items()}


def project(value):
    if hasattr(value, "model_dump"):
        return value.model_dump(mode="json")
    if isinstance(value, (list, tuple)):
        return [project(item) for item in value]
    if isinstance(value, dict):
        return {key: project(item) for key, item in value.items()}
    return value


@contextmanager
def synthetic_entropy(scenario):
    """Inject declared UUID/epoch samples without replacing provider behavior."""
    entropy = scenario.get("entropy")
    if entropy is None:
        yield
        return
    samples = entropy["uuid4"]
    seconds = entropy["unix_seconds"]
    assert type(seconds) in {int, float} and math.isfinite(seconds)
    used = []

    def next_uuid():
        assert len(used) < len(samples), "unexpected UUID generation"
        value = samples[len(used)]
        parsed = uuid.UUID(value)
        assert str(parsed) == value and parsed.version == 4
        assert parsed.variant == uuid.RFC_4122
        used.append(value)
        return parsed

    with (
        patch("uuid.uuid4", next_uuid),
        patch("time.time", return_value=seconds),
    ):
        try:
            yield
        finally:
            assert len(used) == len(samples), "unconsumed UUID samples"

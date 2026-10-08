"""Portable caller models and explicit synthetic entropy inputs."""

import base64
import math
import uuid
from contextlib import ExitStack, contextmanager
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
def replay_wait_clock(scenario, entropy):
    entropy = entropy or {}
    assert not ("photos_wait_trace" in entropy and "auth_wait_trace" in entropy)
    trace = entropy.get("photos_wait_trace", entropy.get("auth_wait_trace"))
    guarded = (
        scenario.get("service") == "auth"
        or scenario.get("operation") == "service_upload"
    )
    if trace is None and not guarded:
        yield
        return
    if trace is not None:
        assert isinstance(trace, list) and trace
    events = trace or []
    used = []
    ticks = []
    failed = []

    def event(kind, duration=None):
        try:
            assert not failed and len(used) < len(events), "undeclared replay wait"
            item = events[len(used)]
            assert item["kind"] == kind, "replay wait event order mismatch"
            value = item["value"]
            assert type(value) in {int, float} and math.isfinite(value) and value >= 0
            if kind == "monotonic":
                assert not ticks or value >= ticks[-1]
                ticks.append(value)
            else:
                assert value == duration, "replay sleep duration mismatch"
            used.append(item)
            return value
        except (AssertionError, KeyError, TypeError, IndexError) as error:
            failed.append(True)
            raise AssertionError("Rejected replay wait event") from error

    with (
        patch("time.monotonic", lambda: event("monotonic")),
        patch("time.sleep", lambda seconds: event("sleep", seconds)),
    ):
        try:
            yield
        finally:
            assert not failed, "Rejected replay wait was caught by reference code"
            assert len(used) == len(events), "unconsumed replay wait trace"


@contextmanager
def synthetic_entropy(scenario):
    """Inject declared UUID/epoch samples without replacing provider behavior."""
    entropy = scenario.get("entropy")
    if entropy is None:
        with replay_wait_clock(scenario, entropy):
            yield
        return
    samples = entropy["uuid4"]
    seconds = entropy["unix_seconds"]
    assert type(seconds) in {int, float} and math.isfinite(seconds)
    used = []
    random_used = []

    def next_uuid():
        assert len(used) < len(samples), "unexpected UUID generation"
        value = samples[len(used)]
        parsed = uuid.UUID(value)
        assert str(parsed) == value and parsed.version == 4
        assert parsed.variant == uuid.RFC_4122
        used.append(value)
        return parsed

    def random_bytes(size):
        samples = entropy["random_bytes"]
        assert len(random_used) < len(samples), "unexpected random-byte generation"
        value = base64.b64decode(samples[len(random_used)], validate=True)
        assert len(value) == size, "random-byte size mismatch"
        random_used.append(value)
        return value

    with (
        patch("uuid.uuid4", next_uuid),
        patch("time.time", return_value=seconds),
        replay_wait_clock(scenario, entropy),
        ExitStack() as stack,
    ):
        if scenario.get("operation") in {"upload_pipeline", "service_upload"}:
            stack.enter_context(
                patch("pyicloud.services.photos_cloudkit.upload.uuid4", next_uuid)
            )
        if "random_bytes" in entropy:
            stack.enter_context(patch("os.urandom", random_bytes))
        position = None
        local_zone = None
        if "photos_local_timezone" in entropy:
            zone, offset = entropy["photos_local_timezone"]
            assert isinstance(zone, str) and type(offset) is int
            local_zone = stack.enter_context(
                patch(
                    "pyicloud.services.photos_cloudkit.upload._local_time_zone",
                    return_value=(zone, offset),
                )
            )
        if "photos_position_ms" in entropy:
            assert entropy["photos_position_ms"] == int(seconds * 1000)
            position = stack.enter_context(
                patch(
                    "pyicloud.services.photos_cloudkit.service._new_album_position",
                    return_value=entropy["photos_position_ms"],
                )
            )
        try:
            yield
        finally:
            assert len(used) == len(samples), "unconsumed UUID samples"
            assert len(random_used) == len(entropy.get("random_bytes", [])), (
                "unconsumed random-byte samples"
            )
            assert position is None or position.call_count == 1, (
                "album position clock consumption mismatch"
            )
            assert local_zone is None or local_zone.call_count == 1, (
                "upload timezone consumption mismatch"
            )

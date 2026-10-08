"""Portable service parity and fail-closed replay controls."""

import base64
import io
import json
import threading
import time
import unittest
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest.mock import patch

from scenario_inputs import synthetic_entropy
from synthetic import FIXTURES, replay_synthetic
from synthetic import execute as execute_scenario


class SyntheticTests(unittest.TestCase):
    def test_findmy_acknowledgement_errors_are_bound(self):
        for change in ["reason", "code", "media", "body", "duplicate", "result"]:
            name = (
                "findmy-sound-ack-array-202.json"
                if change == "result"
                else "findmy-sound-ack-refused-200-json.json"
            )
            scenario = json.loads((FIXTURES / name).read_text())
            with self.subTest(change=change), TemporaryDirectory() as directory:
                response = scenario["exchanges"][-1]["response"]
                if change in {"reason", "code"}:
                    scenario["error_context"][change] = "wrong"
                elif change == "media":
                    response["headers"] = [["Content-Type", "application/octet-stream"]]
                elif change == "body":
                    response["body"]["value"] = base64.b64encode(b"{}").decode()
                elif change == "duplicate":
                    scenario["exchanges"].append(scenario["exchanges"][-1])
                else:
                    scenario["result"] = ["invented completion"]
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario))
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_all_portable_service_scenarios(self):
        fixtures = sorted(FIXTURES.glob("*.json"))
        self.assertGreaterEqual(len(fixtures), 413)
        initial_threads = set(threading.enumerate())
        for fixture in fixtures:
            with self.subTest(scenario=fixture.stem):
                replay_synthetic(fixture)
        self.assertEqual(set(threading.enumerate()) - initial_threads, set())

    def test_semantic_and_wire_changes_fail(self):
        original = json.loads((FIXTURES / "drive-rename-success.json").read_text())
        for change in ["result", "path", "body", "duplicate", "source", "format"]:
            with self.subTest(change=change), TemporaryDirectory() as directory:
                scenario = json.loads(json.dumps(original))
                if change == "result":
                    scenario["result"] = {"unexpected": True}
                elif change == "path":
                    scenario["exchanges"][0]["request"]["path"] = "/unexpected"
                elif change == "body":
                    scenario["inputs"][2] = "changed-name"
                elif change == "duplicate":
                    scenario["exchanges"] *= 2
                elif change == "source":
                    scenario["source"]["commit"] = "wrong"
                else:
                    scenario["format"] = "unknown-format"
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario))
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_drive_transfer_and_failure_state_are_bound(self):
        for change in ["position", "token", "context", "remaining", "node"]:
            fixture = (
                "drive-missing-child"
                if change == "node"
                else "drive-upload-binary-refused-1"
            )
            scenario = json.loads((FIXTURES / (fixture + ".json")).read_text())
            with self.subTest(change=change), TemporaryDirectory() as directory:
                if change == "position":
                    scenario["error_drive_state"]["file_position"] = 0
                elif change == "token":
                    scenario["error_drive_state"]["params"]["token"] = "wrong"
                elif change == "context":
                    scenario["error_context"]["response"]["status"] = 200
                elif change == "remaining":
                    scenario["exchanges"].append(scenario["exchanges"][-1])
                else:
                    scenario["error_node_state"]["root"]["name"] = "wrong"
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario))
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_node_upload_files_close_after_success_and_refusal(self):
        for fixture in sorted(FIXTURES.glob("drive-node-upload-*.json")):
            files = []

            def make_file(*args, tracked_files=files, **kwargs):
                file = io.BytesIO(*args, **kwargs)
                tracked_files.append(file)
                return file

            with (
                self.subTest(scenario=fixture.stem),
                patch("drive_scenarios.BytesIO", side_effect=make_file),
            ):
                replay_synthetic(fixture)
                self.assertEqual(len(files), 1)
                self.assertTrue(files[0].closed)

    def test_node_upload_cursor_zone_and_state_are_bound(self):
        for change in ["position", "zone", "node", "file_state", "extra"]:
            filename = (
                "drive-node-upload-binary-refused-1"
                if change in {"node", "file_state"}
                else "drive-node-upload-position"
            )
            scenario = json.loads((FIXTURES / (filename + ".json")).read_text())
            with self.subTest(change=change), TemporaryDirectory() as directory:
                if change == "position":
                    scenario["inputs"][0]["inputs"][0]["position"] = 0
                elif change == "zone":
                    body = scenario["exchanges"][0]["response"]["body"]
                    payload = json.loads(base64.b64decode(body["value"]))
                    payload[0]["zone"] = "wrong-zone"
                    body["value"] = base64.b64encode(
                        json.dumps(payload).encode()
                    ).decode()
                elif change == "node":
                    scenario["error_node_state"]["node"]["data"]["docwsid"] = "wrong"
                elif change == "file_state":
                    scenario["error_drive_state"]["file_position"] = 0
                else:
                    scenario["exchanges"].append(scenario["exchanges"][-1])
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario))
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_findmy_refresh_state_and_polling_are_bound(self):
        for change in ["state", "context", "sleep", "extra", "response"]:
            fixture = (
                "findmy-refresh-refused"
                if change in {"state", "context"}
                else "findmy-family-progress"
            )
            scenario = json.loads((FIXTURES / (fixture + ".json")).read_text())
            with self.subTest(change=change), TemporaryDirectory() as directory:
                if change == "state":
                    scenario["error_findmy_state"]["devices"][0]["name"] = "wrong"
                elif change == "context":
                    scenario["error_context"]["response"]["status"] = 200
                elif change == "sleep":
                    scenario["entropy"]["findmy_wait_trace"][0]["value"] = 1
                elif change == "extra":
                    scenario["entropy"]["findmy_wait_trace"].append(
                        {"kind": "sleep", "value": 0.5}
                    )
                else:
                    scenario["exchanges"].pop()
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario))
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_findmy_description_results_are_bound(self):
        for change in ["name", "location", "capability", "status", "integer", "null"]:
            scenario = json.loads(
                (FIXTURES / "findmy-description-populated.json").read_text()
            )
            with self.subTest(change=change), TemporaryDirectory() as directory:
                result = scenario["result"]
                if change == "name":
                    result["modelName"] = "wrong"
                elif change == "location":
                    result["location"] = None
                elif change == "capability":
                    result["capabilities"]["location"] = False
                elif change == "status":
                    result["status"]["batteryLevel"] = 0
                elif change == "integer":
                    result["status"]["future"]["nested"][-1] -= 1
                else:
                    result["status"].pop("missing")
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario))
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_findmy_monitor_clock_and_state_are_bound(self):
        for change in [
            "duration",
            "clock",
            "extra",
            "stop",
            "state",
            "failure",
            "infinite",
            "no-stop",
            "cookie",
        ]:
            scenario = json.loads(
                (
                    FIXTURES
                    / (
                        "findmy-monitor-repeated-refusal.json"
                        if change == "cookie"
                        else "findmy-monitor-recovery.json"
                    )
                ).read_text()
            )
            with self.subTest(change=change), TemporaryDirectory() as directory:
                events = scenario["entropy"]["findmy_monitor_trace"]
                if change == "duration":
                    events[0]["delaySeconds"] = 61
                elif change == "clock":
                    events[0]["completedAtUnix"] = 1059
                elif change == "extra":
                    events.append(events[-1])
                elif change == "stop":
                    events[0]["stopped"] = True
                elif change == "state":
                    scenario["result"][-1]["state"]["devices"][0]["name"] = "wrong"
                elif change == "infinite":
                    events[-1]["completedAtUnix"] = float("inf")
                elif change == "no-stop":
                    events[-1]["stopped"] = False
                    events[-1]["completedAtUnix"] = 1183
                elif change == "cookie":
                    scenario["result"][-1]["cookies"][0]["value"] = "wrong"
                else:
                    scenario["result"][1]["failed"] = False
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario))
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_cloudkit_scope_zone_and_payload_are_bound(self):
        for change in ["scope", "library", "zone", "error_payload", "favorite_route"]:
            names = {
                "scope": "photos-container-private-changes-1",
                "library": "photos-shared-library-assets-1",
                "zone": "photos-container-shared-lookup-1",
                "error_payload": "reminders-zones-schema-error",
                "favorite_route": "photos-shared-library-favorite-true",
            }
            scenario = json.loads((FIXTURES / (names[change] + ".json")).read_text())
            with self.subTest(change=change), TemporaryDirectory() as directory:
                if change == "scope":
                    scenario["container_scope"] = "shared"
                elif change == "library":
                    scenario["library"] = "root"
                elif change == "zone":
                    scenario["keyword_inputs"]["zone_id"]["ownerRecordName"] = "wrong"
                elif change == "error_payload":
                    scenario["error_payload"] = {"zones": []}
                else:
                    pair = next(
                        pair
                        for pair in scenario["exchanges"]
                        if pair["request"]["path"].endswith("/records/modify")
                    )
                    pair["request"]["path"] = pair["request"]["path"].replace(
                        "/private/", "/shared/"
                    )
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario))
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_photos_error_resources_are_required_and_bound(self):
        fixture = FIXTURES / "photos-shared-library-favorite-record-error.json"
        for change in ["missing", "photo", "album"]:
            scenario = json.loads(fixture.read_text())
            with self.subTest(change=change), TemporaryDirectory() as directory:
                if change == "missing":
                    del scenario["error_resources"]
                elif change == "photo":
                    scenario["error_resources"]["photo"] = None
                else:
                    scenario["error_resources"]["album"] = {"id": "unexpected"}
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario))
                with self.assertRaisesRegex(AssertionError, "error resource mismatch"):
                    replay_synthetic(path)

        def remove_error_photo(*args, **kwargs):
            from pyicloud.services.photos_cloudkit.models import PhotosServiceException

            try:
                return execute_scenario(*args, **kwargs)
            except PhotosServiceException as error:
                error.photo = None
                raise

        with (
            patch("synthetic.execute", remove_error_photo),
            self.assertRaisesRegex(AssertionError, "error resource mismatch"),
        ):
            replay_synthetic(fixture)

    def test_drive_refresh_and_transfer_results_are_bound(self):
        for fixture in ["drive-root-force-refresh", "drive-upload-position"]:
            scenario = json.loads((FIXTURES / (fixture + ".json")).read_text())
            with self.subTest(fixture=fixture), TemporaryDirectory() as directory:
                if fixture == "drive-upload-position":
                    scenario["inputs"][1]["position"] += 1
                else:
                    scenario["exchanges"].pop()
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario))
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_error_type_and_meaning_must_match(self):
        original = json.loads(
            (FIXTURES / "reminders-lists-record-error.json").read_text()
        )
        for field in ["type", "message"]:
            with self.subTest(field=field), TemporaryDirectory() as directory:
                scenario = json.loads(json.dumps(original))
                scenario["error"][field] = "incorrect"
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario))
                with self.assertRaisesRegex(AssertionError, "semantic error mismatch"):
                    replay_synthetic(path)

    def test_mutation_state_and_entropy_are_bound(self):
        for change in ["tag", "error_state", "uuid_extra", "uuid_missing", "epoch"]:
            fixture = (
                "reminders-update-record-error"
                if change == "error_state"
                else "reminders-update-basic"
            )
            original = json.loads((FIXTURES / (fixture + ".json")).read_text())
            with self.subTest(change=change), TemporaryDirectory() as directory:
                if change == "tag":
                    original["result"]["arguments"][0]["record_change_tag"] = "wrong"
                elif change == "error_state":
                    original["error_arguments"][0]["title"] = "wrong"
                elif change == "uuid_extra":
                    original["entropy"]["uuid4"].append(
                        "000000ff-0000-4000-8000-000000000000"
                    )
                elif change == "uuid_missing":
                    original["entropy"]["uuid4"].pop()
                else:
                    original["entropy"]["unix_seconds"] += 1
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(original))
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_photos_caught_unexpected_refresh_fails(self):
        original = json.loads(
            (FIXTURES / "photos-favorite-record-error.json").read_text()
        )
        with TemporaryDirectory() as directory:
            original["exchanges"] = original["exchanges"][:4]
            path = Path(directory) / "scenario.json"
            path.write_text(json.dumps(original))
            with self.assertRaisesRegex(AssertionError, "caught by reference"):
                replay_synthetic(path)

    def test_photo_entropy_and_result_bindings(self):
        for change in ["random_bytes", "position", "result"]:
            original = json.loads((FIXTURES / "photos-create-album.json").read_text())
            with self.subTest(change=change), TemporaryDirectory() as directory:
                if change == "random_bytes":
                    original["entropy"]["random_bytes"][0] = "AA=="
                elif change == "position":
                    original["entropy"]["photos_position_ms"] += 1
                else:
                    original["result"]["name"] = "Wrong name"
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(original))
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_upload_file_and_clock_inputs_are_bound(self):
        for change in ["bytes", "mtime", "timezone", "uuid", "status"]:
            original = json.loads(
                (FIXTURES / "photos-upload-pipeline-success.json").read_text()
            )
            with self.subTest(change=change), TemporaryDirectory() as directory:
                if change == "bytes":
                    original["file"]["body"] = "YmFk"
                elif change == "mtime":
                    original["file"]["modified_seconds"] += 1
                elif change == "timezone":
                    original["entropy"]["photos_local_timezone"] = ["UTC", 60]
                elif change == "uuid":
                    original["entropy"]["uuid4"][0] = (
                        "00000002-0000-4000-8000-000000000000"
                    )
                else:
                    original["result"]["value"]["response"]["status"] = 500
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(original))
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_upload_error_payload_is_required_and_bound(self):
        original = json.loads(
            (FIXTURES / "photos-upload-pipeline-rejected.json").read_text()
        )
        for change in ["removed", "changed"]:
            with self.subTest(change=change), TemporaryDirectory() as directory:
                scenario = json.loads(json.dumps(original))
                if change == "removed":
                    del scenario["error_payload"]
                else:
                    scenario["error_payload"] = {"unrelated": True}
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario))
                with self.assertRaisesRegex(AssertionError, "error payload mismatch"):
                    replay_synthetic(path)

    def test_service_upload_wait_trace_is_ordered_and_consumed(self):
        for change in ["sleep", "surplus", "missing", "backwards", "boolean"]:
            original = json.loads(
                (FIXTURES / "photos-upload-service-backoff.json").read_text()
            )
            with self.subTest(change=change), TemporaryDirectory() as directory:
                trace = original["entropy"]["photos_wait_trace"]
                if change == "sleep":
                    trace[2]["value"] = 2
                elif change == "surplus":
                    trace.append({"kind": "monotonic", "value": 2})
                elif change == "missing":
                    trace.pop()
                elif change == "backwards":
                    trace[0]["value"] = 1
                else:
                    trace[0]["value"] = False
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(original))
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_auth_session_and_error_state_are_bound(self):
        for change in [
            "token",
            "cookie",
            "logout",
            "error",
            "argument",
            "clock",
            "error_argument",
            "error_context",
        ]:
            fixture = {
                "token": "auth-token-cookie-rotation",
                "cookie": "auth-token-cookie-rotation",
                "logout": "auth-logout-default",
                "error": "auth-terms-refused",
                "argument": "auth-validate-code-success",
                "clock": "auth-pcs-cookies-later",
                "error_argument": "auth-send-code-error",
                "error_context": "auth-token-login-needs-2fa",
            }[change]
            original = json.loads((FIXTURES / (fixture + ".json")).read_text())
            with self.subTest(change=change), TemporaryDirectory() as directory:
                if change == "token":
                    original["result"]["auth_state"]["session_data"][
                        "session_token"
                    ] = "wrong"
                elif change == "cookie":
                    original["result"]["auth_state"]["cookies"][0]["value"] = "wrong"
                elif change == "logout":
                    original["result"]["auth_state"]["session_file_exists"] = True
                elif change == "error":
                    original["error_auth_state"]["trusted_session"] = False
                elif change == "argument":
                    del original["result"]["arguments"][0]["trustBrowser"]
                elif change == "error_argument":
                    original["error_arguments"][0]["unexpected"] = True
                elif change == "error_context":
                    original["error_context"]["response"]["status"] = 500
                else:
                    original["entropy"]["auth_wait_trace"][0]["value"] = 4
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(original))
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_srp_wire_proofs_and_entropy_are_bound(self):
        for change in ["secret", "missing", "surplus", "password", "m1", "m2"]:
            scenario = json.loads((FIXTURES / "auth-srp-s2k.json").read_text())
            with self.subTest(change=change), TemporaryDirectory() as directory:
                if change == "secret":
                    scenario["entropy"]["random_bytes"][0] = base64.b64encode(
                        bytes(reversed(range(256)))
                    ).decode()
                elif change == "missing":
                    scenario["entropy"]["random_bytes"].clear()
                elif change == "surplus":
                    scenario["entropy"]["random_bytes"] *= 2
                elif change == "password":
                    scenario["initial_state"]["synthetic_password"] = "other-password"
                else:
                    body = scenario["exchanges"][2]["request"]["body"]
                    payload = json.loads(base64.b64decode(body["value"]))
                    payload[change] = base64.b64encode(bytes(32)).decode()
                    body["value"] = base64.b64encode(
                        json.dumps(payload).encode()
                    ).decode()
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario))
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_auth_and_upload_cannot_use_undeclared_waits(self):
        for fixture, key in [
            ("auth-pcs-cookies-later", "auth_wait_trace"),
            ("photos-upload-service-delayed", "photos_wait_trace"),
        ]:
            with self.subTest(fixture=fixture), TemporaryDirectory() as directory:
                scenario = json.loads((FIXTURES / (fixture + ".json")).read_text())
                del scenario["entropy"][key]
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario))
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)
        before = time.sleep, time.monotonic
        with self.assertRaisesRegex(AssertionError, "caught by reference"):
            with synthetic_entropy({"service": "auth"}):
                try:
                    time.sleep(1)
                except AssertionError:
                    pass
        self.assertEqual(before, (time.sleep, time.monotonic))


if __name__ == "__main__":
    unittest.main()

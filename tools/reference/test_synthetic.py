"""Portable service parity and fail-closed replay controls."""

import json
import threading
import unittest
from pathlib import Path
from tempfile import TemporaryDirectory

from synthetic import FIXTURES, replay_synthetic


class SyntheticTests(unittest.TestCase):
    def test_all_portable_service_scenarios(self):
        fixtures = sorted(FIXTURES.glob("*.json"))
        self.assertGreaterEqual(len(fixtures), 230)
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


if __name__ == "__main__":
    unittest.main()

"""Portable service parity and fail-closed replay controls."""

import base64
import copy
import io
import json
import threading
import time
import unittest
from datetime import datetime
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest.mock import patch

from scenario_inputs import synthetic_entropy
from synthetic import FIXTURES, replay_synthetic
from synthetic import execute as execute_scenario


class SyntheticTests(unittest.TestCase):
    def test_photo_lookup_matrix(self):
        paths = sorted(FIXTURES.glob("photos-get-*.json"))
        self.assertEqual(len(paths), 22)
        self.assertEqual(sum(replay_synthetic(path) for path in paths), 77)

    def test_photo_lookup_result_fallback_and_consumption_are_bound(self):
        baseline = json.loads((FIXTURES / "photos-get-fallback-found.json").read_text())
        for mutation in ["result", "request", "unused"]:
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                scenario = copy.deepcopy(baseline)
                if mutation == "result":
                    scenario["result"]["filename"] = "wrong.jpg"
                elif mutation == "request":
                    scenario["exchanges"][2]["request"]["path"] = "/wrong"
                else:
                    scenario["exchanges"].append(
                        copy.deepcopy(scenario["exchanges"][-1])
                    )
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_legacy_reminders_matrix(self):
        paths = sorted(FIXTURES.glob("reminders-legacy-*.json"))
        self.assertEqual(len(paths), 9)
        self.assertEqual(sum(replay_synthetic(path) for path in paths), 9)

    def test_legacy_reminders_results_and_consumption_are_bound(self):
        baseline = json.loads((FIXTURES / "reminders-legacy-1.json").read_text())
        for mutation in ["result", "request", "unused"]:
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                scenario = copy.deepcopy(baseline)
                if mutation == "result":
                    scenario["result"]["reminders"][0]["extra"] = {"wrong": True}
                elif mutation == "request":
                    scenario["exchanges"][0]["request"]["path"] = "/wrong"
                else:
                    scenario["exchanges"].append(
                        copy.deepcopy(scenario["exchanges"][0])
                    )
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_photo_assets_matrix(self):
        paths = sorted(FIXTURES.glob("photos-assets-*.json"))
        self.assertEqual(len(paths), 109)
        self.assertEqual(sum(replay_synthetic(path) for path in paths), 427)

    def test_photo_assets_full_projection_and_consumption_are_bound(self):
        baseline = json.loads((FIXTURES / "photos-assets-1.json").read_text())
        for mutation in ["filename", "metadata", "resource", "request", "unused"]:
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                scenario = copy.deepcopy(baseline)
                if mutation == "filename":
                    scenario["result"][0]["filename"] = "different.jpg"
                elif mutation == "metadata":
                    scenario["result"][0]["asset"]["recordChangeTag"] = "different"
                elif mutation == "resource":
                    scenario["result"][0]["versions"]["original"]["url"] = (
                        "https://different.example.invalid"
                    )
                elif mutation == "request":
                    scenario["exchanges"][-1]["request"]["query"][2][1] = "true"
                else:
                    scenario["exchanges"].append(
                        copy.deepcopy(scenario["exchanges"][-1])
                    )
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_photo_assets_failure_and_cookie_are_bound(self):
        for fixture, mutation in [
            ("photos-assets-invalid-last-record", "payload"),
            ("photos-assets-pagination-cookie", "cookie"),
        ]:
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                scenario = json.loads((FIXTURES / (fixture + ".json")).read_text())
                if mutation == "payload":
                    scenario["error_payload"] = {"wrong": True}
                else:
                    scenario["exchanges"][-1]["request"]["headers"] = [
                        header
                        for header in scenario["exchanges"][-1]["request"]["headers"]
                        if header[0].lower() != "cookie"
                    ]
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_photo_count_matrix(self):
        paths = sorted(FIXTURES.glob("photos-count-*.json"))
        self.assertEqual(len(paths), 56)
        self.assertEqual(sum(replay_synthetic(path) for path in paths), 161)

    def test_photo_count_results_requests_and_consumption_are_bound(self):
        baseline = json.loads((FIXTURES / "photos-count-3.json").read_text())
        for mutation in ["result", "selector", "request", "unused"]:
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                scenario = copy.deepcopy(baseline)
                if mutation == "result":
                    scenario["result"] = 9
                elif mutation == "selector":
                    scenario["album"] = "Favorites"
                elif mutation == "request":
                    scenario["exchanges"][-1]["request"]["query"][2][1] = "true"
                else:
                    scenario["exchanges"].append(
                        copy.deepcopy(scenario["exchanges"][-1])
                    )
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_photo_count_failure_payload_and_cookie_are_bound(self):
        for fixture, mutation in [
            ("photos-count-fields-missing", "payload"),
            ("photos-count-pagination-cookie", "cookie"),
        ]:
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                scenario = json.loads((FIXTURES / (fixture + ".json")).read_text())
                if mutation == "payload":
                    scenario["error_payload"] = {"wrong": True}
                else:
                    headers = scenario["exchanges"][-1]["request"]["headers"]
                    scenario["exchanges"][-1]["request"]["headers"] = [
                        header for header in headers if header[0].lower() != "cookie"
                    ]
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_photo_album_matrix(self):
        paths = sorted(FIXTURES.glob("photos-albums-*.json"))
        self.assertEqual(len(paths), 41)
        self.assertEqual(sum(replay_synthetic(path) for path in paths), 98)

    def test_photo_album_results_requests_and_consumption_are_bound(self):
        baseline = json.loads((FIXTURES / "photos-albums-1.json").read_text())
        for mutation in ["name", "fullname", "order", "request", "unused"]:
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                scenario = copy.deepcopy(baseline)
                if mutation in {"name", "fullname"}:
                    scenario["result"][-1][mutation] = "different-album"
                elif mutation == "order":
                    scenario["result"].reverse()
                elif mutation == "request":
                    scenario["exchanges"][1]["request"]["query"][2][1] = "true"
                else:
                    scenario["exchanges"].append(
                        copy.deepcopy(scenario["exchanges"][-1])
                    )
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_photo_album_error_and_cookie_are_bound(self):
        for fixture, mutation in [
            ("photos-albums-sort-invalid", "error"),
            ("photos-albums-pagination-cookie", "cookie"),
        ]:
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                scenario = json.loads((FIXTURES / (fixture + ".json")).read_text())
                if mutation == "error":
                    scenario["error"]["message"] = "different-error"
                else:
                    headers = scenario["exchanges"][-1]["request"]["headers"]
                    scenario["exchanges"][-1]["request"]["headers"] = [
                        header for header in headers if header[0].lower() != "cookie"
                    ]
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_reminder_related_lookup_matrix(self):
        paths = sorted(FIXTURES.glob("reminders-related-*.json"))
        self.assertEqual(len(paths), 46)
        self.assertEqual(sum(replay_synthetic(path) for path in paths), 51)
        for kind in ["tags", "attachments", "recurrence-rules", "alarms"]:
            scenario = json.loads(
                (FIXTURES / f"reminders-related-{kind}-empty-ids.json").read_text()
            )
            self.assertEqual(scenario["result"], [])
            self.assertEqual(scenario["exchanges"], [])
        alarm = json.loads(
            (
                FIXTURES / "reminders-related-alarms-many-ordered-duplicates.json"
            ).read_text()
        )["result"]
        self.assertEqual(len(alarm), 3)
        self.assertEqual(alarm[0]["trigger"]["title"], "Synthetic replacement")
        self.assertIsNone(alarm[1]["trigger"])
        self.assertEqual(alarm[0], alarm[2])

    def test_reminder_related_lookup_results_and_requests_are_bound(self):
        for change in ["result", "order", "input", "unused", "error", "trigger"]:
            name = (
                "alarms-many-ordered-duplicates"
                if change in {"trigger", "order"}
                else "tags-many-ordered-duplicates"
            )
            if change == "error":
                name = "tags-provider-error"
            scenario = json.loads(
                (FIXTURES / f"reminders-related-{name}.json").read_text()
            )
            if change == "result":
                scenario["result"].clear()
            elif change == "order":
                scenario["result"][0], scenario["result"][1] = (
                    scenario["result"][1],
                    scenario["result"][0],
                )
            elif change == "input":
                scenario["inputs"][0]["value"]["hashtag_ids"] = ["other"]
            elif change == "unused":
                scenario["exchanges"].append(scenario["exchanges"][-1])
            elif change == "error":
                scenario["error_payload"][0]["serverErrorCode"] = "OTHER"
            else:
                scenario["result"][0]["trigger"]["title"] = "Changed"
            with self.subTest(change=change), TemporaryDirectory() as directory:
                path = Path(directory) / "changed.json"
                path.write_text(json.dumps(scenario), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_reminder_list_union_selection_and_discovery_cookies(self):
        paths = sorted(FIXTURES.glob("reminders-list-union-*.json"))
        paths += sorted(FIXTURES.glob("reminders-snapshot-union-*.json"))
        paths += [FIXTURES / "reminders-snapshot-discovery-cookie.json"]
        self.assertEqual(len(paths), 27)
        self.assertEqual(sum(replay_synthetic(path) for path in paths), 31)
        for prefix in ["list", "snapshot"]:
            for name in [
                "normal-extra-error",
                "normal-tie-error",
                "normal-tombstone-preferred",
            ]:
                scenario = json.loads(
                    (FIXTURES / f"reminders-{prefix}-union-{name}.json").read_text()
                )
                self.assertEqual(len(scenario["result"]), 1)
            for name in ["tombstone-only", "tombstone-tie-error"]:
                scenario = json.loads(
                    (FIXTURES / f"reminders-{prefix}-union-{name}.json").read_text()
                )
                self.assertEqual(scenario["result"], [])
            for name in [
                "normal-invalid-error",
                "error-preferred",
                "error-tombstone-preferred",
                "error-before-projection",
                "empty-error-code",
            ]:
                scenario = json.loads(
                    (FIXTURES / f"reminders-{prefix}-union-{name}.json").read_text()
                )
                self.assertEqual(scenario["error"]["type"], "RemindersApiError")
            for name in [
                "error-before-wire-validation",
                "error-before-later-zone-validation",
                "projection-before-later-zone-validation",
            ]:
                scenario = json.loads(
                    (FIXTURES / f"reminders-{prefix}-union-{name}.json").read_text()
                )
                self.assertEqual(
                    scenario["error"]["message"], "Changes response validation failed"
                )
                self.assertIn("zones", scenario["error_payload"])

    def test_reminder_list_union_selection_and_cookie_state_are_bound(self):
        for change in ["selected-result", "error", "cookie"]:
            name = (
                "reminders-list-union-normal-extra-error"
                if change == "selected-result"
                else "reminders-snapshot-discovery-cookie"
            )
            scenario = json.loads((FIXTURES / f"{name}.json").read_text())
            if change == "selected-result":
                scenario["result"].clear()
            elif change == "error":
                scenario = json.loads(
                    (
                        FIXTURES / "reminders-list-union-empty-error-code.json"
                    ).read_text()
                )
                scenario["error_payload"][0]["serverErrorCode"] = "REFUSAL"
            else:
                headers = scenario["exchanges"][1]["request"]["headers"]
                scenario["exchanges"][1]["request"]["headers"] = [
                    item for item in headers if item[0] != "cookie"
                ]
            with self.subTest(change=change), TemporaryDirectory() as directory:
                path = Path(directory) / "changed.json"
                path.write_text(json.dumps(scenario), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_reminder_snapshot_discovery_and_replacement(self):
        paths = sorted(FIXTURES.glob("reminders-snapshot-*.json"))
        self.assertEqual(len(paths), 28)
        self.assertEqual(sum(replay_synthetic(path) for path in paths), 45)
        for name, count in [
            ("explicit-0-active", 0),
            ("explicit-1-active", 1),
            ("explicit-3-active", 3),
            ("discovered-many", 3),
            ("discovered-empty", 0),
            ("discovered-no-zones", 0),
        ]:
            scenario = json.loads(
                (FIXTURES / f"reminders-snapshot-{name}.json").read_text()
            )
            self.assertEqual(len(scenario["result"]), count)
        replaced = json.loads(
            (
                FIXTURES / "reminders-snapshot-discovered-duplicate-replacement.json"
            ).read_text()
        )
        self.assertEqual(len(replaced["result"]), 1)
        self.assertEqual(replaced["result"][0]["list_id"], "List/synthetic-2")

    def test_reminder_snapshot_binds_results_filters_and_consumption(self):
        for change in ["result", "filter", "input", "unused", "replacement"]:
            name = (
                "discovered-duplicate-replacement"
                if change == "replacement"
                else "explicit-1-active"
            )
            scenario = json.loads(
                (FIXTURES / f"reminders-snapshot-{name}.json").read_text()
            )
            if change == "result":
                scenario["result"].clear()
            elif change == "replacement":
                scenario["result"][0]["list_id"] = "List/synthetic-0"
            elif change == "filter":
                body = scenario["exchanges"][0]["request"]["body"]
                value = json.loads(base64.b64decode(body["value"]))
                value["query"]["filterBy"][1]["fieldValue"]["value"] = 0
                body["value"] = base64.b64encode(json.dumps(value).encode()).decode()
            elif change == "input":
                scenario["inputs"][0] = "List/synthetic-other"
            else:
                scenario["exchanges"].append(scenario["exchanges"][-1])
            with self.subTest(change=change), TemporaryDirectory() as directory:
                path = Path(directory) / "changed.json"
                path.write_text(json.dumps(scenario), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_reminder_compound_query_matrix(self):
        paths = sorted(FIXTURES.glob("reminders-query-compound-*.json"))
        self.assertEqual(len(paths), 31)
        pairs = 0
        for path in paths:
            with self.subTest(case=path.name):
                pairs += replay_synthetic(path)
        self.assertEqual(pairs, 32)
        complete = json.loads(
            (FIXTURES / "reminders-query-compound-all-types.json").read_text()
        )["result"]
        self.assertEqual(len(complete["reminders"]), 1)
        for name, count in {
            "alarms": 1,
            "triggers": 1,
            "attachments": 2,
            "hashtags": 1,
            "recurrence_rules": 1,
        }.items():
            self.assertEqual(len(complete[name]), count)

    def test_reminder_related_wrapper_sensitive_selection(self):
        for kind, whitespace in [("lf", "\n"), ("cr", "\r")]:
            scenario = json.loads(
                (
                    FIXTURES / f"reminders-query-compound-encoded-url-{kind}.json"
                ).read_text()
            )
            self.assertEqual(
                scenario["result"]["attachments"]["Attachment/synthetic-url"]["url"],
                base64.b64encode(b"https://example.invalid/synthetic").decode()
                + whitespace * 4,
            )
        for kind in ["bytes", "encrypted-bytes"]:
            scenario = json.loads(
                (FIXTURES / f"reminders-query-compound-type-{kind}.json").read_text()
            )
            self.assertEqual(scenario["result"]["triggers"], {})
            self.assertEqual(scenario["result"]["attachments"], {})
            for form in ["plain", "encoded"]:
                scenario = json.loads(
                    (
                        FIXTURES / f"reminders-query-compound-url-{form}-{kind}.json"
                    ).read_text()
                )
                value = b"https://example.invalid/synthetic"
                if form == "encoded":
                    value = base64.b64encode(value)
                self.assertEqual(
                    scenario["result"]["attachments"]["Attachment/synthetic-url"][
                        "url"
                    ],
                    value.decode(),
                )

    def test_reminder_related_field_decoding_and_frequency_selection(self):
        for name in ["numeric-text", "fractional-frequency"]:
            scenario = json.loads(
                (FIXTURES / f"reminders-query-compound-{name}.json").read_text()
            )
            self.assertEqual(
                scenario["result"]["recurrence_rules"]["RecurrenceRule/synthetic-rule"][
                    "frequency"
                ],
                1,
            )
        for name in ["bytes", "encrypted-bytes"]:
            scenario = json.loads(
                (FIXTURES / f"reminders-query-compound-text-{name}.json").read_text()
            )
            self.assertEqual(
                scenario["result"]["hashtags"]["Hashtag/synthetic-tag"]["name"],
                "synthetic tag 🌍",
            )
            self.assertEqual(
                scenario["result"]["alarms"]["Alarm/synthetic-alarm"]["alarm_uid"],
                "synthetic alarm 🌍",
            )
        invalid = json.loads(
            (
                FIXTURES / "reminders-query-compound-invalid-orphan-image.json"
            ).read_text()
        )
        self.assertEqual(invalid["error"]["type"], "ValidationError")

    def test_reminder_compound_query_binds_relations_and_public_results(self):
        for change in ["relation", "result", "replacement", "request", "unused"]:
            name = "duplicates" if change == "replacement" else "all-types"
            scenario = json.loads(
                (FIXTURES / f"reminders-query-compound-{name}.json").read_text()
            )
            if change == "relation":
                body = scenario["exchanges"][0]["response"]["body"]
                payload = json.loads(base64.b64decode(body["value"]))
                payload["records"][1]["fields"]["Reminder"]["value"]["recordName"] = (
                    "Reminder/synthetic-orphan"
                )
                body["value"] = base64.b64encode(json.dumps(payload).encode()).decode()
            elif change == "result":
                scenario["result"]["attachments"].clear()
            elif change == "replacement":
                scenario["result"]["alarms"]["Alarm/synthetic-alarm"][
                    "record_change_tag"
                ] = "synthetic-tag"
            elif change == "request":
                scenario["inputs"][0] = "List/synthetic-other"
            else:
                scenario["exchanges"].append(scenario["exchanges"][-1])
            with self.subTest(change=change), TemporaryDirectory() as directory:
                path = Path(directory) / "changed.json"
                path.write_text(json.dumps(scenario), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_reminder_change_iteration_matrix(self):
        paths = sorted(FIXTURES.glob("reminders-changes-*.json"))
        self.assertEqual(len(paths), 72)
        pairs = 0
        for path in paths:
            with self.subTest(case=path.name):
                pairs += replay_synthetic(path)
        self.assertEqual(pairs, 75)
        full = json.loads(
            (FIXTURES / "reminders-changes-full-deleted.json").read_text()
        )
        self.assertEqual(full["result"][0]["type"], "deleted")
        self.assertEqual(len(full["result"][0]["reminder"]), 21)

    def test_reminder_change_iteration_binds_events_requests_and_consumption(self):
        for change in ["event", "order", "request", "unused", "payload"]:
            name = "record-error" if change == "payload" else "paged"
            scenario = json.loads(
                (FIXTURES / f"reminders-changes-{name}.json").read_text()
            )
            if change == "event":
                scenario["result"][0]["type"] = "deleted"
            elif change == "order":
                scenario["result"].reverse()
            elif change == "request":
                scenario["keyword_inputs"]["since"] = "synthetic-changed"
            elif change == "unused":
                scenario["exchanges"].append(scenario["exchanges"][-1])
            else:
                scenario["error_payload"]["recordName"] = "synthetic-changed"
            with self.subTest(change=change), TemporaryDirectory() as directory:
                path = Path(directory) / "changed.json"
                path.write_text(json.dumps(scenario), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_reminder_sync_cursor_fallback_and_paging_matrix(self):
        paths = sorted(FIXTURES.glob("reminders-sync-*.json"))
        self.assertEqual(len(paths), 53)
        pairs = 0
        for path in paths:
            scenario = json.loads(path.read_text(encoding="utf-8"))
            with self.subTest(case=path.name):
                pairs += replay_synthetic(path)
                if path.stem in {
                    "reminders-sync-unavailable",
                    "reminders-sync-rate-limit",
                    "reminders-sync-unauthorized",
                    "reminders-sync-forbidden",
                }:
                    self.assertEqual(len(scenario["exchanges"]), 1)
                    self.assertEqual(
                        scenario["error"]["type"], "PyiCloudAPIResponseException"
                    )
        self.assertEqual(pairs, 85)

    def test_reminder_sync_cursor_binds_final_token_and_consumption(self):
        for change in ["token", "result", "unused"]:
            scenario = json.loads((FIXTURES / "reminders-sync-paged.json").read_text())
            if change == "token":
                body = scenario["exchanges"][-1]["response"]["body"]
                payload = json.loads(base64.b64decode(body["value"]))
                payload["zones"][0]["syncToken"] = "synthetic-changed"
                body["value"] = base64.b64encode(json.dumps(payload).encode()).decode()
            elif change == "result":
                scenario["result"] = "synthetic-changed"
            else:
                scenario["exchanges"].append(scenario["exchanges"][-1])
            with self.subTest(change=change), TemporaryDirectory() as directory:
                path = Path(directory) / "changed.json"
                path.write_text(json.dumps(scenario), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_reminder_lookup_complete_projection_and_text_fallbacks(self):
        for name in ["full", "audit-dates", "unreadable-documents"]:
            path = FIXTURES / f"reminders-get-{name}.json"
            scenario = json.loads(path.read_text(encoding="utf-8"))
            with self.subTest(case=name):
                self.assertEqual(len(scenario["result"]), 21)
                self.assertEqual(replay_synthetic(path), 1)
        full = json.loads((FIXTURES / "reminders-get-full.json").read_text())
        self.assertEqual(full["result"]["title"], "Synthetic title 🌍")
        self.assertEqual(full["result"]["desc"], "Synthetic notes\nsecond line")
        self.assertEqual(
            full["result"]["alarm_ids"],
            ["synthetic-first", "synthetic-second", "synthetic-first"],
        )
        unreadable = json.loads(
            (FIXTURES / "reminders-get-unreadable-documents.json").read_text()
        )
        self.assertEqual(unreadable["result"]["title"], "Error Decoding Title")
        self.assertEqual(unreadable["result"]["desc"], "")

    def test_reminder_lookup_rejects_missing_public_fields(self):
        for field in ["created", "modified", "title", "due_date", "alarm_ids"]:
            scenario = json.loads((FIXTURES / "reminders-get-full.json").read_text())
            scenario["result"].pop(field)
            with self.subTest(field=field), TemporaryDirectory() as directory:
                path = Path(directory) / "changed.json"
                path.write_text(json.dumps(scenario), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_incomplete_photo_records_skip_orphans_and_preserve_valid_pairs(self):
        for name, count in [("master-only", 0), ("asset-only", 0), ("mixed", 1)]:
            path = FIXTURES / f"photos-incomplete-records-{name}.json"
            scenario = json.loads(path.read_text(encoding="utf-8"))
            with self.subTest(case=name):
                self.assertEqual(len(scenario["result"]), count)
                self.assertEqual(replay_synthetic(path), 4)

    def test_incomplete_photo_records_bind_pairing_and_consumption(self):
        for change in ["master-reference", "result", "unused"]:
            scenario = json.loads(
                (FIXTURES / "photos-incomplete-records-mixed.json").read_text(
                    encoding="utf-8"
                )
            )
            if change == "master-reference":
                body = scenario["exchanges"][-1]["response"]["body"]
                payload = json.loads(base64.b64decode(body["value"]))
                payload["records"][-1]["fields"]["masterRef"]["value"]["recordName"] = (
                    "synthetic-missing-master"
                )
                body["value"] = base64.b64encode(json.dumps(payload).encode()).decode()
            elif change == "result":
                scenario["result"] = []
            else:
                scenario["exchanges"].append(scenario["exchanges"][-1])
            with self.subTest(change=change), TemporaryDirectory() as directory:
                path = Path(directory) / "changed.json"
                path.write_text(json.dumps(scenario), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_photo_download_matrix_binds_exact_bytes_null_and_lazy_getters(self):
        paths = sorted(FIXTURES.glob("photos-download-*.json"))
        self.assertEqual(len(paths), 25)
        self.assertEqual(sum(replay_synthetic(path) for path in paths), 105)

    def test_photo_download_success_rejects_changed_bytes_urls_and_unused_calls(self):
        for change in ["result", "origin", "query", "unused"]:
            scenario = json.loads(
                (FIXTURES / "photos-download-escaped-signed-url.json").read_text(
                    encoding="utf-8"
                )
            )
            if change == "result":
                scenario["result"] = None
            elif change == "origin":
                scenario["exchanges"][-1]["request"]["origin"] = (
                    "https://wrong-assets.example.invalid"
                )
            elif change == "query":
                scenario["exchanges"][-1]["request"]["query"][-1][1] = "wrong"
            else:
                scenario["exchanges"].append(scenario["exchanges"][-1])
            with self.subTest(change=change), TemporaryDirectory() as directory:
                path = Path(directory) / "changed.json"
                path.write_text(json.dumps(scenario), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_photo_download_failures_reject_changed_outcomes_and_requests(self):
        for name in [
            "photos-download-refused-429.json",
            "photos-download-transport-timeout.json",
        ]:
            for change in ["error", "request", "unused"]:
                scenario = json.loads((FIXTURES / name).read_text(encoding="utf-8"))
                if change == "error":
                    scenario["error"]["message"] = "changed failure"
                elif change == "request":
                    scenario["exchanges"][-1]["request"]["origin"] = (
                        "https://wrong-assets.example.invalid"
                    )
                else:
                    scenario["exchanges"].append(scenario["exchanges"][-1])
                with (
                    self.subTest(name=name, change=change),
                    TemporaryDirectory() as directory,
                ):
                    path = Path(directory) / "changed.json"
                    path.write_text(json.dumps(scenario), encoding="utf-8")
                    with self.assertRaises(AssertionError):
                        replay_synthetic(path)

    def test_recently_added_matrix_preserves_newest_first_without_count_requests(self):
        paths = sorted(FIXTURES.glob("photos-recently-added-*.json"))
        self.assertEqual(len(paths), 16)
        self.assertEqual(sum(replay_synthetic(path) for path in paths), 61)
        for path in paths:
            scenario = json.loads(path.read_text(encoding="utf-8"))
            if "error" in scenario:
                continue
            with self.subTest(case=path.name):
                dates = [
                    datetime.fromisoformat(photo["added"])
                    for photo in scenario["result"]
                ]
                self.assertEqual(dates, sorted(dates, reverse=True))
                ids = [photo["id"] for photo in scenario["result"]]
                self.assertEqual(len(ids), len(set(ids)))
                self.assertFalse(
                    any(
                        "internal/records/query/batch" in pair["request"]["path"]
                        for pair in scenario["exchanges"]
                    )
                )
                self.assertEqual(replay_synthetic(path), len(scenario["exchanges"]))

    def test_recently_added_order_rank_and_consumption_are_bound(self):
        name = "photos-recently-added-overlap-partial-next.json"
        for change in ["order", "duplicate", "first-rank", "next-rank", "unused"]:
            scenario = json.loads((FIXTURES / name).read_text(encoding="utf-8"))
            if change == "order":
                scenario["result"].reverse()
            elif change == "duplicate":
                scenario["result"].append(scenario["result"][0])
            elif change == "unused":
                scenario["exchanges"].append(scenario["exchanges"][-1])
            else:
                index = -2 if change == "first-rank" else -1
                body = scenario["exchanges"][index]["request"]["body"]
                payload = json.loads(base64.b64decode(body["value"]))
                rank = next(
                    item
                    for item in payload["query"]["filterBy"]
                    if item["fieldName"] == "startRank"
                )
                rank["fieldValue"]["value"] += 1
                body["value"] = base64.b64encode(json.dumps(payload).encode()).decode()
            with self.subTest(change=change), TemporaryDirectory() as directory:
                path = Path(directory) / "changed.json"
                path.write_text(json.dumps(scenario), encoding="utf-8")
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

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

    def test_photos_initialization_matrix(self):
        paths = sorted(FIXTURES.glob("photos-index-*.json"))
        self.assertEqual(len(paths), 27)
        for path in paths:
            with self.subTest(path=path.name):
                self.assertEqual(replay_synthetic(path), 1)

    def test_photos_initialization_results_requests_and_consumption_are_bound(self):
        baseline = json.loads((FIXTURES / "photos-index-ready.json").read_text())
        for mutation in ["state", "cursor", "request", "unused"]:
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                scenario = copy.deepcopy(baseline)
                if mutation == "state":
                    scenario["result"]["state"] = "PENDING"
                elif mutation == "cursor":
                    scenario["result"]["sync_token"] = "wrong-cursor"
                elif mutation == "request":
                    scenario["exchanges"][0]["request"]["query"][2][1] = "true"
                else:
                    scenario["exchanges"].append(
                        copy.deepcopy(scenario["exchanges"][0])
                    )
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario))
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)


if __name__ == "__main__":
    unittest.main()

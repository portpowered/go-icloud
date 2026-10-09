"""Replay reference login restoration through the shared coverage runner."""

import copy
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from icloud import ROOT
from reference_resume import replay_reference_resume


class ReferenceResumeTests(unittest.TestCase):
    def test_reference_import_authentication_and_read(self):
        self.assertEqual(
            replay_reference_resume(
                "reference-resume.json", "portos.reference-resume.v1"
            ),
            8,
        )

    def test_reference_restore_and_resume_again(self):
        self.assertEqual(
            replay_reference_resume(
                "reference-resume-repeat.json", "portos.reference-resume-repeat.v1"
            ),
            12,
        )

    def test_shared_runner_rejects_changed_state_requests_and_unused_exchanges(self):
        fixtures = ROOT / "tests/replay/fixtures/synthetic/local"
        name = "reference-resume-repeat.json"
        corpus = json.loads((fixtures / name).read_text(encoding="utf-8"))
        for field, replacement in [
            ("authState", {}),
            ("restoredAuthState", {}),
            ("headers", {}),
            ("cookieState", []),
            ("request", "DELETE"),
            ("unused", None),
        ]:
            changed = copy.deepcopy(corpus)
            case = changed["cases"][0]
            if field == "request":
                case["exchanges"][0]["request"]["method"] = replacement
            elif field == "unused":
                case["exchanges"].append(copy.deepcopy(case["exchanges"][-1]))
            else:
                case[field] = replacement
            with self.subTest(field=field), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                destination = root / "tests/replay/fixtures/synthetic/local"
                destination.mkdir(parents=True)
                (destination / name).write_text(json.dumps(changed), encoding="utf-8")
                (destination / "reference-logins.json").write_bytes(
                    (fixtures / "reference-logins.json").read_bytes()
                )
                with (
                    patch("reference_resume.ROOT", root),
                    self.assertRaises(AssertionError),
                ):
                    replay_reference_resume(name, "portos.reference-resume-repeat.v1")

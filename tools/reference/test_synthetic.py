"""Portable service parity and fail-closed replay controls."""

import json
import unittest
from pathlib import Path
from tempfile import TemporaryDirectory

from synthetic import FIXTURES, replay_synthetic


class SyntheticTests(unittest.TestCase):
    def test_all_portable_service_scenarios(self):
        fixtures = sorted(FIXTURES.glob("*.json"))
        self.assertGreaterEqual(len(fixtures), 20)
        for fixture in fixtures:
            with self.subTest(scenario=fixture.stem):
                replay_synthetic(fixture)

    def test_semantic_and_wire_changes_fail(self):
        original = json.loads((FIXTURES / "drive-rename-success.json").read_text())
        for change in ["result", "path", "body", "duplicate", "source"]:
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
                else:
                    scenario["source"]["commit"] = "wrong"
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario))
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)


if __name__ == "__main__":
    unittest.main()

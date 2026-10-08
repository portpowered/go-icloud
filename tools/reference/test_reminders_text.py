"""Bind the portable CRDT corpus to the pinned reference decoder."""

import copy
import json
import unittest
from pathlib import Path

from reminders_text import replay_reminders_text

ROOT = Path(__file__).resolve().parents[2]


def corpus():
    path = ROOT / "tests/replay/fixtures/synthetic/binary/reminders-text.json"
    return json.loads(path.read_text(encoding="utf-8"))


class RemindersTextTests(unittest.TestCase):
    def test_source_text_corpus(self):
        data = corpus()
        self.assertEqual(
            data["source"]["commit"], "e2e44ab875d47dab4475096021da60030f26c35e"
        )
        self.assertEqual(replay_reminders_text(data), 52)

    def test_changed_document_result_is_rejected(self):
        data = copy.deepcopy(corpus())
        for change in ["encoding", "value", "source_type"]:
            altered = copy.deepcopy(data)
            altered["cases"][0]["result"][change] = "changed"
            with self.subTest(change=change), self.assertRaises(ValueError):
                replay_reminders_text(altered)
        altered = copy.deepcopy(data)
        altered["cases"][0]["error"] = {"type": "CRDTDecodeError", "message": "wrong"}
        with self.assertRaises(ValueError):
            replay_reminders_text(altered)
        altered = copy.deepcopy(data)
        failure = next(case for case in altered["cases"] if "error" in case)
        failure["error"]["message"] = "wrong"
        with self.assertRaises(ValueError):
            replay_reminders_text(altered)


if __name__ == "__main__":
    unittest.main()

"""Pinned Source correspondence for synthetic Photos projection and cursor gaps."""

import base64
import json
import unittest
from pathlib import Path
from tempfile import TemporaryDirectory

from synthetic import FIXTURES, replay_synthetic


class PhotosProjectionGapTests(unittest.TestCase):
    def test_projection_and_pagination_pairs_match_source(self):
        cases = {
            "raw-original-format": 4,
            "raw-filename-extension": 4,
            "image-filename-extension": 4,
            "unknown-movie-format": 4,
            "cursor-pagination": 3,
        }
        for name, exchanges in cases.items():
            with self.subTest(name=name):
                path = FIXTURES / ("photos-replay-gap-" + name + ".json")
                self.assertEqual(replay_synthetic(path), exchanges)

    def test_reusing_the_initial_cursor_rejects_the_second_request(self):
        path = FIXTURES / "photos-replay-gap-cursor-pagination.json"
        scenario = json.loads(path.read_text(encoding="utf-8"))
        request = scenario["exchanges"][-1]["request"]
        body = json.loads(base64.b64decode(request["body"]["value"]))
        body["zones"][0]["syncToken"] = "synthetic-old"
        encoded = json.dumps(body).encode()
        request["body"]["value"] = base64.b64encode(encoded).decode()
        for header in request["headers"]:
            if header[0].lower() == "content-length":
                header[1] = str(len(encoded))
        with TemporaryDirectory() as directory:
            changed = Path(directory) / "changed.json"
            changed.write_text(json.dumps(scenario), encoding="utf-8")
            with self.assertRaises(AssertionError):
                replay_synthetic(changed)

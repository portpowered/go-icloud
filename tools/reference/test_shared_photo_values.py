"""Bind legacy Photos defensive projections to the pinned offline Source."""

import base64
import copy
import json
import unittest
from pathlib import Path

from icloud import SOURCE
from pyicloud.services.photos_legacy import (
    PhotoStreamLibrary,
    PhotoStreamAsset,
    SharedPhotoStreamAlbum,
)


class SharedPhotoValuesTests(unittest.TestCase):
    def setUp(self):
        root = Path(__file__).resolve().parents[2]
        scenario = json.loads((root / "tests/replay/fixtures/synthetic/http/photos-upload-shared-get-found.json").read_text(encoding="utf-8"))
        self.assertEqual(scenario["source"], SOURCE["live"])
        response = scenario["exchanges"][2]["response"]["body"]
        self.records = json.loads(base64.b64decode(response["value"]))["records"]

    def asset(self):
        return PhotoStreamAsset(None, self.records[0], self.records[1])

    def test_value_rules(self):
        fields = self.records[0]["fields"]
        fields["filenameEnc"] = {"value": "WVdKag=="}
        self.assertEqual(self.asset().filename, "YWJj")
        fields["originalCreationDate"] = {"value": 1e300}
        self.assertEqual(self.asset().created.timestamp(), 0)
        for value, expected in [(1.75, 1), ("1.0", 0)]:
            fields["resOriginalFileSize"] = {"value": value}
            self.assertEqual(self.asset().size, expected)
        for value, expected in [("false", True), ([], False)]:
            self.records[1]["pluginFields"] = {"likedByCaller": {"value": value}}
            self.assertEqual(self.asset().liked, expected)
        for value in [None, "many"]:
            self.records[1]["pluginFields"] = {"likeCount": {"value": value}}
            self.assertEqual(self.asset().like_count, value)

    def test_missing_dimensions_raise(self):
        for field in ["resOriginalWidth", "resOriginalHeight"]:
            records = copy.deepcopy(self.records)
            del records[0]["fields"][field]
            with self.assertRaises(KeyError):
                _ = PhotoStreamAsset(None, *records).dimensions

    def test_ignored_records(self):
        parser = object.__new__(PhotoStreamLibrary)
        ignored = [None, 42, "ignored", {"recordType": "future"}, {"recordType": "CPLAsset", "fields": 42}]
        assets, masters = parser.parse_asset_response({"records": ignored + self.records})
        self.assertEqual(len(assets), 1)
        self.assertEqual(len(masters), 1)
        self.assertEqual(parser.parse_asset_response({"records": 42}), ({}, []))
        self.records[1]["fields"] = {"masterRef": {"value": 42}}
        assets, _ = parser.parse_asset_response({"records": self.records})
        self.assertEqual(assets, {})

    def test_inert_shared_album_methods_have_no_transport(self):
        album = object.__new__(SharedPhotoStreamAlbum)
        self.assertFalse(album.delete())
        self.assertIsNone(album.rename("synthetic-name"))
        self.assertFalse(hasattr(album, "upload"))
        library = object.__new__(PhotoStreamLibrary)
        self.assertFalse(hasattr(library, "upload_file"))


if __name__ == "__main__":
    unittest.main()

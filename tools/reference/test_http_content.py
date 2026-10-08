"""Bind compressed-wire pairs to the reference's pinned HTTP dependencies."""

import base64
import copy
import json
import unittest
from importlib.metadata import version
from urllib.parse import urlencode

import requests
from icloud import ROOT, SOURCE
from network_guard import forbid_network
from recording import ReplayAdapter


def replay_content(case):
    pair = case["exchange"]
    request = pair["request"]
    prepared = requests.Request(
        request["method"],
        request["origin"] + request["path"] + "?" + urlencode(request["query"]),
        headers=dict(request["headers"]),
        data=base64.b64decode(request["body"]["value"], validate=True),
    ).prepare()
    adapter = ReplayAdapter([pair])
    with forbid_network():
        response = adapter.send(prepared)
        try:
            actual = {"decoded": base64.b64encode(response.content).decode()}
        except requests.exceptions.ContentDecodingError as error:
            actual = {"error": {"type": type(error).__name__, "message": str(error)}}
        finally:
            response.close()
        adapter.assert_consumed()
    expected = {key: case[key] for key in ["decoded", "error"] if key in case}
    if actual != expected:
        raise AssertionError("Compressed entity outcome changed")


class HTTPContentTests(unittest.TestCase):
    def test_source_compressed_wire_pairs(self):
        path = ROOT / "tests/replay/fixtures/synthetic/binary/http-content.json"
        corpus = json.loads(path.read_text(encoding="utf-8"))
        self.assertEqual(corpus["source"], SOURCE["live"])
        self.assertEqual(corpus["format"], "portos.http-content.v1")
        self.assertEqual(corpus["evidence"], "synthetic; implementation-derived")
        self.assertEqual(
            corpus["dependencies"],
            {name: version(name) for name in ["requests", "urllib3"]},
        )
        self.assertEqual(len(corpus["cases"]), 604)
        for case in corpus["cases"]:
            with self.subTest(case=case["name"]):
                self.assertEqual(
                    case["exchange"]["response"]["bodyRepresentation"], "wire"
                )
                replay_content(case)

    def test_changed_outcome_and_body_representation_are_rejected(self):
        path = ROOT / "tests/replay/fixtures/synthetic/binary/http-content.json"
        original = json.loads(path.read_text(encoding="utf-8"))["cases"][0]
        changed = copy.deepcopy(original)
        changed["decoded"] = ""
        with self.assertRaises(AssertionError):
            replay_content(changed)
        changed = copy.deepcopy(original)
        changed["exchange"]["response"]["bodyRepresentation"] = "decoded"
        with self.assertRaises(AssertionError):
            replay_content(changed)

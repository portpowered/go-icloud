"""Actual reference socket framing and strict transcript negative controls."""

import base64
import json
import unittest
from pathlib import Path
from tempfile import TemporaryDirectory

from socket_replay import FIXTURES, Transcript, match_frame, replay_socket


class SocketReplayTests(unittest.TestCase):
    def test_reference_transcripts(self):
        fixtures = sorted(FIXTURES.glob("*.json"))
        self.assertGreaterEqual(len(fixtures), 18)
        for path in fixtures:
            with self.subTest(scenario=path.stem):
                replay_socket(path)

    def test_changed_transcripts_fail_closed(self):
        original = json.loads((FIXTURES / "binary-length-1.json").read_text())
        for change in ["path", "payload", "order", "duplicate", "result", "source"]:
            with self.subTest(change=change), TemporaryDirectory() as directory:
                scenario = json.loads(json.dumps(original))
                if change == "path":
                    scenario["events"][0]["lines"][0] = "GET /wrong HTTP/1.1"
                elif change == "payload":
                    scenario["events"][2]["payload"] = base64.b64encode(b"y").decode()
                elif change == "order":
                    scenario["events"][1:3] = reversed(scenario["events"][1:3])
                elif change == "duplicate":
                    scenario["events"].append({"direction": "close"})
                elif change == "result":
                    scenario["results"] = ["incorrect"]
                else:
                    scenario["source"]["commit"] = "incorrect"
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario))
                with self.assertRaises(AssertionError):
                    replay_socket(path)

    def test_frame_mutations_fail(self):
        expected = {"opcode": 2, "payload": "eA=="}
        frame = b"\x82\x81abcd\x19"
        match_frame(frame, expected)
        for mutation in [
            b"\x02" + frame[1:],
            b"\x83" + frame[1:],
            b"\x82\x01" + frame[2:],
            frame + b"trailing",
            frame[:-1],
            frame[:-1] + b"\x18",
            b"\x82\xfe\x00\x01" + frame[2:],
        ]:
            with self.subTest(frame=mutation), self.assertRaises(AssertionError):
                match_frame(mutation, expected)

    def test_unconsumed_client_buffer_fails(self):
        original = json.loads((FIXTURES / "coalesced-upgrade-frames.json").read_text())
        for suffix in [b"\x82\x01x", b"TRAILING"]:
            with self.subTest(suffix=suffix), TemporaryDirectory() as directory:
                scenario = json.loads(json.dumps(original))
                event = scenario["events"][1]
                event["suffix"] = base64.b64encode(
                    base64.b64decode(event["suffix"]) + suffix
                ).decode("ascii")
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario))
                with self.assertRaisesRegex(AssertionError, "unconsumed reference"):
                    replay_socket(path)

    def test_failed_upgrade_buffer_is_also_consumed(self):
        original = json.loads((FIXTURES / "upgrade-invalid-accept.json").read_text())
        with TemporaryDirectory() as directory:
            original["events"][1]["suffix"] = base64.b64encode(b"TRAILING").decode()
            path = Path(directory) / "scenario.json"
            path.write_text(json.dumps(original))
            with self.assertRaisesRegex(AssertionError, "unconsumed reference"):
                replay_socket(path)

    def test_routing_and_connection_ownership(self):
        scenario = json.loads((FIXTURES / "empty-session.json").read_text())
        transcript = Transcript(scenario)
        with self.assertRaises(AssertionError):
            transcript.connect(("wrong.invalid", 443), 5)
        raw = transcript.connect(("bridge.example.invalid", 443), 5)
        with self.assertRaises(AssertionError):
            transcript.connect(("bridge.example.invalid", 443), 5)
        with self.assertRaises(AssertionError):
            transcript.wrap_socket(object(), "bridge.example.invalid")
        transcript.wrap_socket(raw, "bridge.example.invalid")
        with self.assertRaises(AssertionError):
            transcript.wrap_socket(raw, "bridge.example.invalid")


if __name__ == "__main__":
    unittest.main()

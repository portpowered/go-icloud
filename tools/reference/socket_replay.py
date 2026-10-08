"""Replay ordered plaintext socket transcripts through the pinned bridge client.

TLS factory injection covers ownership and routing, not TLS cryptography.
Entropy is matched by decoded value and size rather than fixed random bytes.
"""

import base64
import hashlib
import json
import struct
from unittest.mock import patch
from urllib.parse import urlsplit

from icloud import ROOT, SOURCE, verify_reference
from network_guard import forbid_network

FIXTURES = ROOT / "tests/replay/fixtures/synthetic/socket"
GUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"


def decode(value):
    return base64.b64decode(value, validate=True)


def match_frame(data, expected):
    """Require canonical client framing; only the four mask bytes vary."""
    assert len(data) >= 6, "truncated client frame"
    assert data[0] == 0x80 | expected["opcode"], "frame opcode/finality mismatch"
    assert data[1] & 0x80, "client frame must be masked"
    size = data[1] & 127
    offset = 2
    if size == 126:
        size = struct.unpack("!H", data[2:4])[0]
        offset = 4
        assert 126 <= size < 65536, "noncanonical frame length"
    elif size == 127:
        size = struct.unpack("!Q", data[2:10])[0]
        offset = 10
        assert size >= 65536, "noncanonical frame length"
    assert len(data) == offset + 4 + size, "frame length/trailing bytes mismatch"
    mask = data[offset : offset + 4]
    payload = bytes(
        value ^ mask[index % 4] for index, value in enumerate(data[offset + 4 :])
    )
    assert payload == decode(expected["payload"]), "frame payload mismatch"


class Transcript:
    """One ordered duplex stream, with no response available before its send."""

    def __init__(self, scenario):
        self.scenario = scenario
        self.events = scenario["events"]
        self.index = 0
        self.pending = b""
        self.key = None
        self.connected = False
        self.wrapped = False
        self.timeout_set = False
        self.closed = False
        self.raw = object()

    def take(self, direction):
        assert self.index < len(self.events), "unexpected socket event"
        event = self.events[self.index]
        assert event["direction"] == direction, "socket event order mismatch"
        return event

    def connect(self, address, timeout):
        url = urlsplit(self.scenario["connection"]["url"])
        assert not self.connected, "duplicate socket connection"
        assert address == (url.hostname, url.port or 443), "socket route mismatch"
        assert timeout == self.scenario["connection"]["timeout"]
        self.connected = True
        return self.raw

    def wrap_socket(self, raw, server_hostname):
        assert self.connected and not self.wrapped, "TLS wrap order mismatch"
        assert raw is self.raw, "TLS socket ownership mismatch"
        assert server_hostname == urlsplit(self.scenario["connection"]["url"]).hostname
        self.wrapped = True
        return self

    def settimeout(self, timeout):
        assert self.wrapped and not self.timeout_set, "timeout order mismatch"
        assert timeout == self.scenario["connection"]["timeout"]
        self.timeout_set = True

    def sendall(self, data):
        assert self.timeout_set and not self.closed, "send socket lifecycle mismatch"
        event = self.take("send")
        if event["kind"] == "upgrade":
            lines = data.decode("ascii").split("\r\n")
            assert lines[-2:] == ["", ""], "upgrade terminator mismatch"
            assert lines[:-3] == event["lines"], "upgrade request mismatch"
            prefix = "Sec-WebSocket-Key: "
            assert lines[-3].startswith(prefix), "upgrade key header mismatch"
            key = lines[-3][len(prefix) :]
            assert len(decode(key)) == 16, "upgrade key size mismatch"
            assert base64.b64encode(decode(key)).decode("ascii") == key
            self.key = key
        else:
            match_frame(data, event)
        self.index += 1
        if event.get("error"):
            raise OSError(event["error"])

    def recv(self, size):
        assert self.timeout_set and not self.closed, "receive lifecycle mismatch"
        if not self.pending:
            event = self.take("receive")
            if event["kind"] == "upgrade":
                assert self.key is not None, "upgrade response before request"
                accept = base64.b64encode(
                    hashlib.sha1((self.key + GUID).encode("ascii")).digest()
                ).decode("ascii")
                self.pending = event["template"].replace("{accept}", accept).encode(
                    "ascii"
                ) + decode(event.get("suffix", ""))
            else:
                self.pending = decode(event["data"])
            self.index += 1
            if event.get("error"):
                raise OSError(event["error"])
        result, self.pending = self.pending[:size], self.pending[size:]
        return result

    def close(self):
        self.take("close")
        assert not self.closed, "duplicate socket close"
        self.closed = True
        self.index += 1

    def consumed(self):
        assert not self.pending, "unconsumed received bytes"
        assert self.index == len(self.events), "unconsumed socket events"
        assert self.closed == self.scenario["reference_socket_closed"]


def replay_socket(path):
    verify_reference()
    from pyicloud.hsa2_bridge import _RawWebSocketClient

    scenario = json.loads(path.read_text())
    assert scenario["format"] == "portos.socket-scenario.v1"
    assert scenario["source"] == SOURCE["live"]
    assert scenario["evidence"] == "synthetic; implementation-derived"
    transcript = Transcript(scenario)
    # Retain the instance even if __init__ raises, so failed upgrades cannot hide
    # received bytes buffered before the exception. Execute the actual initializer.
    client = _RawWebSocketClient.__new__(_RawWebSocketClient)
    initialized = False
    results = []
    error = None
    with (
        forbid_network(),
        patch("pyicloud.hsa2_bridge.socket.create_connection", transcript.connect),
        patch(
            "pyicloud.hsa2_bridge.ssl.create_default_context", return_value=transcript
        ),
    ):
        try:
            client.__init__(**scenario["connection"])
            initialized = True
            for action in scenario["actions"]:
                if action["operation"] == "send_binary":
                    client.send_binary(decode(action["payload"]))
                elif action["operation"] == "read_message":
                    results.append(
                        base64.b64encode(client.read_message()).decode("ascii")
                    )
                else:
                    raise AssertionError("unknown socket action")
        except AssertionError:
            raise
        except Exception as exception:
            error = {"type": type(exception).__name__, "message": str(exception)}
        finally:
            if initialized:
                client.close()
    assert error == scenario.get("error"), "socket semantic error mismatch"
    assert results == scenario["results"], "socket semantic result mismatch"
    assert not client._buffer, "unconsumed reference buffer bytes"
    transcript.consumed()
    return len(transcript.events)

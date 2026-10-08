"""Bind bridge HTTP and actual raw socket framing to one portable timeline."""

import base64
from contextlib import ExitStack, contextmanager
from dataclasses import fields
from functools import wraps
from unittest.mock import patch

from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec
from socket_replay import Transcript


def protobuf(data):
    """Decode canonical fields independently of the implementation under test."""
    offset = 0

    def integer():
        nonlocal offset
        value = 0
        for index in range(10):
            assert offset < len(data), "truncated bootstrap integer"
            byte = data[offset]
            offset += 1
            value |= (byte & 127) << (7 * index)
            if byte < 128:
                assert index == 0 or byte != 0, "noncanonical bootstrap integer"
                assert value < 1 << 64
                return value
        raise AssertionError("oversize bootstrap integer")

    result = []
    while offset < len(data):
        tag = integer()
        assert tag >> 3 > 0
        if tag & 7 == 0:
            value = integer()
        else:
            assert tag & 7 == 2, "unsupported bootstrap wire type"
            size = integer()
            assert offset + size <= len(data), "truncated bootstrap bytes"
            value = data[offset : offset + size]
            offset += size
        result.append((tag >> 3, value))
    return result


def signed_connection(path):
    """Verify the variable signature while retaining every bootstrap field."""
    from cryptography.exceptions import InvalidSignature

    raw = bytes.fromhex(path)
    assert raw.hex() == path
    outer = protobuf(raw)
    assert len(outer) == 1 and outer[0][0] == 1
    body = protobuf(outer[0][1])
    assert [number for number, _ in body] == [1, 2, 3, 5]
    public, nonce, signature, expiration = [value for _, value in body]
    assert len(public) == 65 and public[0] == 4
    assert len(nonce) == 17 and nonce[0] == 0
    assert signature[:2] == b"\x01\x03"
    expiry = protobuf(expiration)
    assert len(expiry) == 1 and expiry[0][0] == 1
    key = ec.EllipticCurvePublicKey.from_encoded_point(ec.SECP256R1(), public)
    try:
        key.verify(signature[2:], nonce, ec.ECDSA(hashes.SHA256()))
    except InvalidSignature as error:
        raise AssertionError("invalid bootstrap signature") from error
    return {
        "public_key": base64.b64encode(public).decode(),
        "nonce": base64.b64encode(nonce).decode(),
        "expiration_seconds": expiry[0][1],
    }


def bridge_state(state):
    if state is None:
        return None
    result = {
        field.name: getattr(state, field.name)
        for field in fields(state)
        if field.name not in {"connection_path", "websocket"}
    }
    result["connection"] = signed_connection(state.connection_path)
    result["websocket_active"] = state.websocket is not None
    return result


class Timeline:
    def __init__(self, events):
        self.events = events
        self.index = 0
        self.failed = False

    def take(self, event):
        try:
            assert not self.failed and self.index < len(self.events)
            assert self.events[self.index] == event, "bridge network order mismatch"
            self.index += 1
        except AssertionError:
            self.failed = True
            raise

    def consumed(self):
        assert not self.failed, "caught bridge network order mismatch"
        assert self.index == len(self.events), "unconsumed bridge network timeline"


@contextmanager
def validation(timeline):
    try:
        assert not timeline.failed, "previous bridge exchange failed"
        yield
    except (AssertionError, ValueError, KeyError, TypeError, IndexError) as error:
        timeline.failed = True
        raise AssertionError("Rejected bridge boundary") from error


def guarded(method):
    @wraps(method)
    def checked(self, *args, **kwargs):
        with validation(self.timeline):
            return method(self, *args, **kwargs)

    return checked


class BridgeTranscript(Transcript):
    def __init__(self, scenario, timeline, index, url):
        super().__init__(scenario)
        self.timeline = timeline
        self.connection_index = index
        self.url = url

    def mark(self, operation):
        event = {
            "surface": "socket",
            "connection": self.connection_index,
            "operation": operation,
        }
        if operation in {"send", "receive", "close"}:
            event["event"] = self.index
        self.timeline.take(event)

    @guarded
    def connect(self, address, timeout):
        self.mark("connect")
        return super().connect(address, timeout)

    @guarded
    def wrap_socket(self, raw, server_hostname):
        self.mark("wrap")
        return super().wrap_socket(raw, server_hostname)

    @guarded
    def settimeout(self, timeout):
        self.mark("timeout")
        return super().settimeout(timeout)

    @guarded
    def sendall(self, data):
        self.mark("send")
        if self.index == 0:
            first, rest = data.split(b"\r\n", 1)
            path = self.url.split("/v2/", 1)[1]
            assert first == f"GET /v2/{path} HTTP/1.1".encode()
            data = b"GET /v2/{signed_bootstrap} HTTP/1.1\r\n" + rest
        super().sendall(data)

    @guarded
    def recv(self, size):
        if not self.pending:
            self.mark("receive")
        return super().recv(size)

    @guarded
    def close(self):
        self.mark("close")
        super().close()


@contextmanager
def bridge_network(api, scenario, adapter):
    """Inject only entropy and TCP/TLS factories, preserving bridge algorithms."""
    network = scenario.get("bridge_network")
    if network is None:
        yield
        return
    from pyicloud.hsa2_bridge import _RawWebSocketClient

    timeline = Timeline(network["timeline"])
    connections = []
    scalars = []
    prover_random = []
    http_used = []
    original_send = adapter.send
    original_factory = api._trusted_device_bridge._websocket_factory

    def send(request, **kwargs):
        timeline.take({"surface": "http", "exchange": len(http_used)})
        http_used.append(True)
        return original_send(request, **kwargs)

    def keypair_unchecked(curve):
        assert type(curve) is ec.SECP256R1
        values = network["private_scalars"]
        assert len(scalars) < len(values), "undeclared bridge private scalar"
        value = int(values[len(scalars)], 16)
        scalars.append(value)
        return ec.derive_private_key(value, curve)

    def keypair(curve):
        with validation(timeline):
            return keypair_unchecked(curve)

    def randbelow(upper):
        with validation(timeline):
            samples = network.get("prover_random", [])
            assert len(prover_random) < len(samples), "undeclared bridge prover entropy"
            item = samples[len(prover_random)]
            assert upper == int(item["upper_hex"], 16), "prover random bound mismatch"
            value = int(item["value_hex"], 16)
            assert 0 <= value < upper
            prover_random.append(value)
            return value

    def websocket_unchecked(url, timeout, origin, user_agent):
        index = len(connections)
        assert index < len(network["connections"]), "undeclared bridge connection"
        item = network["connections"][index]
        expected = item["connection"]
        prefix = expected["url"].removesuffix("{signed_bootstrap}")
        assert expected["url"].endswith("/v2/{signed_bootstrap}")
        assert url.startswith(prefix)
        assert signed_connection(url[len(prefix) :]) == item["bootstrap"]
        assert (timeout, origin, user_agent) == (
            expected["timeout"],
            expected["origin"],
            expected["user_agent"],
        )
        transcript = BridgeTranscript(item, timeline, index, url)
        client = _RawWebSocketClient.__new__(_RawWebSocketClient)
        connections.append((client, transcript))
        with (
            patch("pyicloud.hsa2_bridge.socket.create_connection", transcript.connect),
            patch(
                "pyicloud.hsa2_bridge.ssl.create_default_context",
                return_value=transcript,
            ),
        ):
            client.__init__(url, timeout, origin, user_agent)
        return client

    def websocket(url, timeout, origin, user_agent):
        with validation(timeline):
            return websocket_unchecked(url, timeout, origin, user_agent)

    with ExitStack() as stack:
        stack.enter_context(patch.object(adapter, "send", send))
        stack.enter_context(
            patch("pyicloud.hsa2_bridge.ec.generate_private_key", keypair)
        )
        stack.enter_context(
            patch("pyicloud.hsa2_bridge_prover.secrets.randbelow", randbelow)
        )
        api._trusted_device_bridge._websocket_factory = websocket
        try:
            yield
        finally:
            try:
                api._clear_trusted_device_bridge_state()
                for client, transcript in connections:
                    assert not client._buffer, "unconsumed bridge client buffer"
                    transcript.consumed()
                assert len(connections) == len(network["connections"])
                assert len(scalars) == len(network["private_scalars"])
                assert len(prover_random) == len(network.get("prover_random", []))
                timeline.consumed()
            finally:
                api._trusted_device_bridge._websocket_factory = original_factory

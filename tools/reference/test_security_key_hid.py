"""Actual pinned Fido2Client ceremonies over strict, synthetic HID transcripts."""

import json
import unittest
from pathlib import Path
from unittest.mock import patch

from cryptography.hazmat.primitives.asymmetric import ec
from fido2.client import DefaultClientDataCollector, Fido2Client, PinRequiredError
from fido2.hid import CtapHidDevice
from fido2.hid.base import CtapHidConnection, HidDescriptor
from fido2.webauthn import PublicKeyCredentialRequestOptions


FIXTURES = Path(__file__).resolve().parents[2] / "tests/replay/fixtures/securitykey"


class StrictHIDConnection(CtapHidConnection):
    def __init__(self, fixture):
        self.events = []
        for step in fixture["steps"]:
            self.events.extend(("write", bytes.fromhex(value)[1:]) for value in step["writes"])
            self.events.extend(("read", bytes.fromhex(value)) for value in step["reads"])
        self.closed = False

    def write_packet(self, data):
        if not self.events:
            raise AssertionError("unexpected HID write")
        direction, expected = self.events.pop(0)
        if direction != "write" or data != expected:
            raise AssertionError(f"HID write mismatch: expected {expected.hex()}, got {data.hex()}")

    def read_packet(self):
        if not self.events:
            raise AssertionError("unexpected HID read")
        direction, value = self.events.pop(0)
        if direction != "read":
            raise AssertionError("HID direction mismatch")
        return value

    def close(self):
        self.closed = True


class SecurityKeyHIDTests(unittest.TestCase):
    def test_source_ceremonies(self):
        names = (
            "python-ctap2-synthetic.json",
            "python-u2f-synthetic.json",
            "python-uv1-synthetic.json",
            "python-uv2-synthetic.json",
            "python-selection-synthetic.json",
            "python-zero-limit-synthetic.json",
            "python-uv-retry-synthetic.json",
            "python-uv-blocked-synthetic.json",
            "python-pin-required-synthetic.json",
        )
        for name in names:
            with self.subTest(scenario=name):
                fixture = json.loads((FIXTURES / name).read_text())
                connection = StrictHIDConnection(fixture)
                descriptor = HidDescriptor("synthetic", 0, 0, 64, 64, "Synthetic", None)
                with (
                    patch("fido2.hid.os.urandom", return_value=bytes(range(8))),
                    patch("fido2.ctap2.pin.ec.generate_private_key", return_value=ec.derive_private_key(1, ec.SECP256R1())),
                ):
                    device = CtapHidDevice(descriptor, connection)
                    try:
                        client = Fido2Client(device, DefaultClientDataCollector("https://apple.com"))
                        options = PublicKeyCredentialRequestOptions.from_dict({
                            "challenge": "AQID",
                            "rpId": "apple.com",
                            "userVerification": "discouraged",
                            "allowCredentials": [
                                {"type": "public-key", "id": value}
                                for value in fixture.get("credentialIds", ["qrvM"])
                            ],
                        })
                        if fixture.get("error") == "pinRequired":
                            with self.assertRaises(PinRequiredError):
                                client.get_assertion(options)
                            self.assertEqual(connection.events, [])
                            continue
                        result = client.get_assertion(options).get_response(0)
                        self.assertEqual(bytes(result.response.client_data).hex(), fixture["clientData"])
                        self.assertEqual(bytes(result.response.authenticator_data).hex(), fixture["authenticatorData"])
                        self.assertEqual(result.response.signature.hex(), fixture["signature"])
                        self.assertEqual(result.raw_id.hex(), fixture["credentialId"])
                        self.assertEqual(connection.events, [])
                    finally:
                        device.close()
                self.assertTrue(connection.closed)

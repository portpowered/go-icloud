"""Combined bridge proof, ordering and teardown compatibility controls."""

import copy
import json
import unittest
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest.mock import patch

from synthetic import FIXTURES, replay_synthetic


class BridgeReplayTests(unittest.TestCase):
    def test_modern_proof_entropy_and_challenge_are_bound(self):
        original = json.loads(
            (FIXTURES / "auth-bridge-modern-code-204.json").read_text()
        )
        for change in ["scalar", "bound", "missing", "surplus", "proof", "ciphertext"]:
            with self.subTest(change=change), TemporaryDirectory() as directory:
                scenario = copy.deepcopy(original)
                samples = scenario["bridge_network"]["prover_random"]
                if change == "scalar":
                    samples[0]["value_hex"] = "3"
                elif change == "bound":
                    samples[0]["upper_hex"] = "2"
                elif change == "missing":
                    del scenario["bridge_network"]["prover_random"]
                elif change == "surplus":
                    samples *= 2
                elif change == "proof":
                    scenario["exchanges"][2]["request"]["body"]["value"] = "e30="
                else:
                    scenario["bridge_network"]["connections"][0]["events"][8][
                        "data"
                    ] = "ggA="
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario))
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_decrypted_synthetic_code_matches_exact_bytes(self):
        from pyicloud.hsa2_bridge_prover import TrustedDeviceBridgeProver

        original = TrustedDeviceBridgeProver.decrypt_message
        observed = []

        def wrong(prover, ciphertext):
            plaintext = original(prover, ciphertext)
            observed.append(plaintext)
            return "654322"

        with patch.object(TrustedDeviceBridgeProver, "decrypt_message", wrong):
            with self.assertRaises(AssertionError):
                replay_synthetic(FIXTURES / "auth-bridge-modern-code-204.json")
        self.assertEqual(observed, ["654321"])

    def test_caught_extra_prover_random_call_is_rejected(self):
        import secrets

        import synthetic
        from pyicloud.hsa2_bridge_prover import _P256_ORDER

        original = synthetic.execute
        attempted = []

        def extra(api, scenario, observations):
            value = original(api, scenario, observations)
            try:
                secrets.randbelow(_P256_ORDER)
            except AssertionError:
                attempted.append(True)
            return value

        with patch.object(synthetic, "execute", extra):
            with self.assertRaises(AssertionError):
                replay_synthetic(FIXTURES / "auth-bridge-modern-code-204.json")
        self.assertEqual(attempted, [True])

    def test_bootstrap_wire_and_public_state_are_bound(self):
        original = json.loads((FIXTURES / "auth-bridge-prompt.json").read_text())
        for change in ["nonce", "key", "expiry", "state", "ack", "order", "tail"]:
            with self.subTest(change=change), TemporaryDirectory() as directory:
                scenario = copy.deepcopy(original)
                connection = scenario["bridge_network"]["connections"][0]
                if change == "nonce":
                    connection["bootstrap"]["nonce"] = "AA=="
                elif change == "key":
                    connection["bootstrap"]["public_key"] = "AA=="
                elif change == "expiry":
                    connection["bootstrap"]["expiration_seconds"] -= 1
                elif change == "state":
                    scenario["result"]["auth_state"]["bridge"]["push_token"] = "wrong"
                elif change == "ack":
                    connection["events"][5]["payload"] = ""
                elif change == "order":
                    tape = scenario["bridge_network"]["timeline"]
                    tape[5], tape[7] = tape[7], tape[5]
                else:
                    connection["events"].append({"direction": "close"})
                path = Path(directory) / "scenario.json"
                path.write_text(json.dumps(scenario))
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)

    def test_invalid_signed_bootstrap_is_rejected(self):
        from pyicloud import hsa2_bridge

        encode = hsa2_bridge._encode_connection_message

        def corrupt(public, nonce, signature):
            return encode(public, nonce, bytes(len(signature)))

        with patch.object(hsa2_bridge, "_encode_connection_message", corrupt):
            with self.assertRaises(AssertionError) as failure:
                replay_synthetic(FIXTURES / "auth-bridge-prompt.json")
        error = failure.exception
        reasons = []
        while error is not None:
            reasons.append(str(error))
            error = error.__context__
        self.assertIn("invalid bootstrap signature", reasons)

    def test_consumed_timeline_rejects_caught_reordering(self):
        from bridge_replay import Timeline

        timeline = Timeline([{"surface": "http", "exchange": 0}])
        with self.assertRaises(AssertionError):
            timeline.take({"surface": "http", "exchange": 1})
        with self.assertRaises(AssertionError):
            timeline.take({"surface": "http", "exchange": 0})
        with self.assertRaisesRegex(AssertionError, "caught bridge"):
            timeline.consumed()

    def test_caught_frame_mismatch_cannot_be_retried(self):
        from pyicloud.hsa2_bridge import _RawWebSocketClient

        original = _RawWebSocketClient.send_binary
        attempted = []

        def retry(client, payload):
            if not attempted:
                attempted.append(True)
                try:
                    client._socket.sendall(b"\x82\x81" + bytes(4) + b"x")
                except AssertionError:
                    pass
            return original(client, payload)

        scenario = json.loads((FIXTURES / "auth-bridge-prompt.json").read_text())
        tape = scenario["bridge_network"]["timeline"]
        tape.insert(6, copy.deepcopy(tape[6]))
        with TemporaryDirectory() as directory:
            path = Path(directory) / "scenario.json"
            path.write_text(json.dumps(scenario))
            with patch.object(_RawWebSocketClient, "send_binary", retry):
                with self.assertRaises(AssertionError):
                    replay_synthetic(path)
        self.assertEqual(attempted, [True])

    def test_caught_extra_factory_attempts_remain_failures(self):
        import synthetic
        from cryptography.hazmat.primitives.asymmetric import ec

        original = synthetic.execute
        for factory in ["keypair", "websocket"]:
            attempted = []

            def extra(
                api, scenario, observations, *, attempted=attempted, factory=factory
            ):
                value = original(api, scenario, observations)
                attempted.append(True)
                try:
                    if factory == "keypair":
                        ec.generate_private_key(ec.SECP256R1())
                    else:
                        api._trusted_device_bridge._websocket_factory(
                            "wss://bridge.example.invalid/v2/extra",
                            30,
                            "https://idmsa.apple.com",
                            "synthetic-agent",
                        )
                except AssertionError:
                    pass
                return value

            with (
                self.subTest(factory=factory),
                patch.object(synthetic, "execute", extra),
            ):
                with self.assertRaises(AssertionError):
                    replay_synthetic(FIXTURES / "auth-bridge-prompt.json")
            self.assertEqual(attempted, [True])

    def test_bridge_factories_restore_after_success_and_failure(self):
        import os
        import secrets
        import time
        import uuid

        from cryptography.hazmat.primitives.asymmetric import ec
        from pyicloud import hsa2_bridge

        before = (
            ec.generate_private_key,
            secrets.randbelow,
            hsa2_bridge.socket.create_connection,
            hsa2_bridge.ssl.create_default_context,
            os.urandom,
            uuid.uuid4,
            time.monotonic,
            time.time,
            time.sleep,
        )
        for name in ["prompt", "step0-refused", "nonce-retry", "modern-code-204"]:
            with self.subTest(scenario=name):
                replay_synthetic(FIXTURES / ("auth-bridge-" + name + ".json"))
                self.assertEqual(
                    before,
                    (
                        ec.generate_private_key,
                        secrets.randbelow,
                        hsa2_bridge.socket.create_connection,
                        hsa2_bridge.ssl.create_default_context,
                        os.urandom,
                        uuid.uuid4,
                        time.monotonic,
                        time.time,
                        time.sleep,
                    ),
                )


if __name__ == "__main__":
    unittest.main()

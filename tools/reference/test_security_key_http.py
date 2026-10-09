"""Source-derived assertion submission and session trust, without physical HID I/O."""

import unittest

from synthetic import FIXTURES, replay_synthetic


class SecurityKeyHTTPTests(unittest.TestCase):
    def test_assertion_submission_and_trust(self):
        self.assertEqual(
            replay_synthetic(FIXTURES / "auth-security-key-assertion-accepted.json"), 3
        )

    def test_existing_trusted_device_code(self):
        self.assertEqual(
            replay_synthetic(FIXTURES / "auth-existing-trusted-device-code.json"), 0
        )

    def test_terms_trust_delivery_state(self):
        for variant in ("accepted", "untrusted"):
            with self.subTest(variant=variant):
                self.assertEqual(
                    replay_synthetic(
                        FIXTURES / f"auth-terms-trust-{variant}-notice.json"
                    ), 5
                )

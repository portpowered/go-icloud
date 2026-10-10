"""The source identity guard compares physical paths without weakening its pin."""

import unittest
from pathlib import Path
from unittest.mock import patch

import pyicloud
from icloud import ROOT, SOURCE, verify_reference


class ReferenceCheckoutTests(unittest.TestCase):
    def test_resolved_checkout_matches_imported_source(self):
        with patch(
            "icloud.subprocess.check_output",
            side_effect=[SOURCE["live"]["commit"] + "\n", ""],
        ):
            self.assertIs(verify_reference(), pyicloud.PyiCloudService)

    def test_foreign_import_is_rejected(self):
        with (
            patch(
                "icloud.subprocess.check_output",
                side_effect=[SOURCE["live"]["commit"] + "\n", ""],
            ),
            patch.object(
                pyicloud, "__file__", str(ROOT / "foreign" / "pyicloud" / "__init__.py")
            ),
        ):
            with self.assertRaisesRegex(RuntimeError, "repository virtual environment"):
                verify_reference()

    def test_dirty_checkout_is_rejected(self):
        with patch(
            "icloud.subprocess.check_output",
            side_effect=[SOURCE["live"]["commit"] + "\n", " M pyicloud/base.py\n"],
        ):
            with self.assertRaisesRegex(RuntimeError, "pinned clean source"):
                verify_reference()

    def test_mismatched_pin_is_rejected(self):
        with patch("icloud.subprocess.check_output", side_effect=["unexpected\n", ""]):
            with self.assertRaisesRegex(RuntimeError, "pinned clean source"):
                verify_reference()

    def test_import_uses_physical_checkout(self):
        expected = (ROOT / ".reference" / "pyicloud-live" / "pyicloud").resolve()
        self.assertEqual(Path(pyicloud.__file__).resolve().parent, expected)

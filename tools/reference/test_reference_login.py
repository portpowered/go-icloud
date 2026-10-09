"""Prove the portable reference-login files retain pinned Source cookie behavior."""

import hashlib
import json
import re
import tempfile
import unittest
from datetime import UTC, datetime
from pathlib import Path

from icloud import ROOT, SOURCE
from network_guard import forbid_network
from pyicloud import PyiCloudService
from pyicloud.cookie_jar import PyiCloudCookieJar


class ReferenceLoginTests(unittest.TestCase):
    def test_portable_local_credentials(self):
        fixture = ROOT / "tests/replay/fixtures/synthetic/local/reference-logins.json"
        corpus = json.loads(fixture.read_text(encoding="utf-8"))
        self.assertEqual(corpus["source"]["commit"], SOURCE["live"]["commit"])
        for case in corpus["cases"]:
            with (
                self.subTest(case=case["filename"]),
                tempfile.TemporaryDirectory() as directory,
                forbid_network(),
            ):
                username = case["identity"]["username"]
                self.assertEqual(
                    case["accountKey"],
                    hashlib.sha256(username.encode()).hexdigest()[:24],
                )
                self.assertEqual(case["filename"], re.sub(r"\W", "", username))
                path = Path(directory) / "cookies"
                path.write_text(case["cookieFile"], encoding="utf-8")
                jar = PyiCloudCookieJar(str(path))
                jar.load()
                expected = []
                for cookie in jar:
                    value = {
                        "name": cookie.name,
                        "value": cookie.value,
                        "domain": cookie.domain,
                        "path": cookie.path,
                        "secure": cookie.secure,
                        "hostOnly": not cookie.domain_specified,
                        "httpOnly": cookie.has_nonstandard_attr("HttpOnly"),
                        "maxAge": 0,
                    }
                    if cookie.expires is not None:
                        value["expires"] = (
                            datetime.fromtimestamp(cookie.expires, UTC)
                            .isoformat()
                            .replace("+00:00", "Z")
                        )
                    expected.append(value)
                self.assertEqual(case["expectedCookies"], expected)
                self.assertEqual(path.read_text(encoding="utf-8"), case["cookieFile"])
                api = PyiCloudService(
                    username,
                    authenticate=False,
                    client_id="synthetic-client",
                    cookie_directory=directory,
                    china_mainland=case["identity"]["china"],
                )
                self.assertEqual(case["expectedHeaders"], dict(api.session.headers))

"""Replay reference login-file loading, authentication and Account reads together."""

import json
import tempfile
import unittest
from pathlib import Path

from auth_scenarios import auth_state
from icloud import ROOT, SOURCE, close_reference, verify_reference
from network_guard import forbid_network
from recording import ReplayAdapter


class ReferenceResumeTests(unittest.TestCase):
    def test_reference_import_authentication_and_read(self):
        self.replay_corpus("reference-resume.json", "portos.reference-resume.v1")

    def test_reference_restore_and_resume_again(self):
        self.replay_corpus(
            "reference-resume-repeat.json", "portos.reference-resume-repeat.v1"
        )

    def replay_corpus(self, filename, format_name):
        fixtures = ROOT / "tests/replay/fixtures/synthetic/local"
        local = json.loads(
            (fixtures / "reference-logins.json").read_text(encoding="utf-8")
        )
        corpus = json.loads((fixtures / filename).read_text(encoding="utf-8"))
        self.assertEqual(corpus["source"], SOURCE["live"])
        self.assertEqual(corpus["format"], format_name)
        self.assertEqual(corpus["evidence"], "synthetic; implementation-derived")
        self.assertEqual(len(corpus["cases"]), 4)
        service = verify_reference()
        for case in corpus["cases"]:
            login = next(
                row for row in local["cases"] if row["filename"] == case["localCase"]
            )
            with (
                self.subTest(case=case["name"]),
                tempfile.TemporaryDirectory() as directory,
                forbid_network(),
            ):
                account = Path(directory) / "accounts" / login["accountKey"]
                account.mkdir(parents=True)
                (account / (login["filename"] + ".session")).write_text(
                    json.dumps(login["session"]), encoding="utf-8"
                )
                (account / (login["filename"] + ".cookiejar")).write_text(
                    login["cookieFile"], encoding="utf-8"
                )
                api = service(
                    login["identity"]["username"],
                    authenticate=False,
                    cookie_directory=str(account),
                    china_mainland=login["identity"]["china"],
                    with_family=False,
                )
                adapter = ReplayAdapter(case["exchanges"])
                api.session.mount("https://", adapter)
                api.session.mount("http://", adapter)
                try:
                    api.authenticate()
                    self.assertEqual(auth_state(api), case["authState"])
                    self.assertEqual(
                        [dict(item) for item in api.account.devices], case["devices"]
                    )
                    if "restoredAuthState" in case:
                        self.assertEqual(dict(api.session.headers), case["headers"])
                        self.assertEqual(cookie_state(api), case["cookieState"])
                        close_reference(api)
                        api = service(
                            login["identity"]["username"],
                            authenticate=False,
                            cookie_directory=str(account),
                            china_mainland=login["identity"]["china"],
                            with_family=False,
                        )
                        api.session.mount("https://", adapter)
                        api.session.mount("http://", adapter)
                        api.authenticate()
                        self.assertEqual(auth_state(api), case["restoredAuthState"])
                    adapter.assert_consumed()
                finally:
                    close_reference(api)


def cookie_state(api):
    return [
        {
            "name": cookie.name,
            "value": cookie.value,
            "domain": cookie.domain,
            "path": cookie.path,
            "secure": cookie.secure,
            "domainSpecified": cookie.domain_specified,
            "expires": cookie.expires,
            "httpOnly": cookie.has_nonstandard_attr("HttpOnly"),
        }
        for cookie in api.session.cookies
    ]

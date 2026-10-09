"""Offline paired replay of reference saved-login restoration and repeated resume."""

import json
import tempfile
from pathlib import Path

from auth_scenarios import auth_state
from icloud import ROOT, SOURCE, close_reference, verify_reference
from network_guard import forbid_network
from recording import ReplayAdapter


def require_equal(actual, expected, description):
    if actual != expected:
        raise AssertionError(description)


def replay_reference_resume(filename, format_name):
    fixtures = ROOT / "tests/replay/fixtures/synthetic/local"
    local = json.loads((fixtures / "reference-logins.json").read_text(encoding="utf-8"))
    corpus = json.loads((fixtures / filename).read_text(encoding="utf-8"))
    require_equal(
        corpus["source"], SOURCE["live"], "Reference resume expectation changed"
    )
    require_equal(corpus["format"], format_name, "Reference resume expectation changed")
    require_equal(
        corpus["evidence"],
        "synthetic; implementation-derived",
        "Reference resume expectation changed",
    )
    require_equal(len(corpus["cases"]), 4, "Reference resume expectation changed")
    service = verify_reference()
    exchanges = 0
    for case in corpus["cases"]:
        login = next(
            row for row in local["cases"] if row["filename"] == case["localCase"]
        )
        with tempfile.TemporaryDirectory() as directory, forbid_network():
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
                require_equal(
                    auth_state(api),
                    case["authState"],
                    "Reference resume expectation changed",
                )
                require_equal(
                    [dict(item) for item in api.account.devices],
                    case["devices"],
                    "Reference resume expectation changed",
                )
                if "restoredAuthState" in case:
                    require_equal(
                        dict(api.session.headers),
                        case["headers"],
                        "Reference resume expectation changed",
                    )
                    require_equal(
                        cookie_state(api),
                        case["cookieState"],
                        "Reference resume expectation changed",
                    )
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
                    require_equal(
                        auth_state(api),
                        case["restoredAuthState"],
                        "Reference resume expectation changed",
                    )
                adapter.assert_consumed()
                exchanges += len(case["exchanges"])
            finally:
                close_reference(api)
    return exchanges


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

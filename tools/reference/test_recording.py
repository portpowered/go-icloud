"""Offline synthetic exchange tests; no Apple account or network is required."""

import base64
import io
import json
import shutil
import unittest
from contextlib import ExitStack
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest.mock import patch

import icloud
import replay
import requests
from recording import Recorder, ReplayAdapter, request_record


def pair(request, *, status=200, body=b'{"items":[]}', headers=None):
    return {
        "request": request_record(request),
        "response": {
            "status": status,
            "headers": headers or [["Content-Type", "application/json"]],
            "body": {"encoding": "base64", "value": base64.b64encode(body).decode()},
        },
    }


def prepared(url="https://example.invalid/items?q=one&q=two", **kwargs):
    return requests.Request("POST", url, **kwargs).prepare()


class ReplayTests(unittest.TestCase):
    def test_success_empty_many_and_provider_error(self):
        cases = [
            (b'{"items":[]}', 200),
            (b'{"items":[1,2]}', 200),
            (b'{"error":"denied"}', 401),
        ]
        for body, status in cases:
            with self.subTest(status=status, body=body):
                request = prepared(json={"limit": 2})
                adapter = ReplayAdapter([pair(request, status=status, body=body)])
                response = adapter.send(request)
                self.assertEqual(response.status_code, status)
                self.assertEqual(response.content, body)
                adapter.assert_consumed()

    def test_mismatches_never_consume_or_return_a_response(self):
        original = prepared(json={"limit": 2}, headers={"Authorization": "token"})
        changes = [
            prepared("https://other.invalid/items?q=one&q=two", json={"limit": 2}),
            prepared("https://example.invalid/other?q=one&q=two", json={"limit": 2}),
            prepared("https://example.invalid/items?q=one", json={"limit": 2}),
            prepared(json={"limit": 3}),
            prepared(json={"limit": 2}),
        ]
        method = original.copy()
        method.method = "GET"
        changes.append(method)
        for changed in changes:
            with self.subTest(request=changed):
                adapter = ReplayAdapter([pair(original)])
                with self.assertRaises(AssertionError):
                    adapter.send(changed)
                self.assertEqual(adapter.index, 0)

    def test_duplicate_and_unconsumed_requests(self):
        request = prepared()
        adapter = ReplayAdapter([pair(request)])
        with self.assertRaises(AssertionError):
            adapter.assert_consumed()
        adapter.send(request)
        with self.assertRaises(AssertionError):
            adapter.send(request)

    def test_transport_failure(self):
        request = prepared()
        exchange = {"request": request_record(request), "error": "Timeout"}
        adapter = ReplayAdapter([exchange])
        with self.assertRaises(requests.Timeout):
            adapter.send(request)
        adapter.assert_consumed()

    def test_cookie_headers_survive_session_replay(self):
        session = requests.Session()
        first = session.prepare_request(
            requests.Request("GET", "https://example.invalid/a")
        )
        session.cookies.set("session", "token", domain="example.invalid", path="/")
        session.cookies.set("other", "value", domain="example.invalid", path="/")
        second = session.prepare_request(
            requests.Request("GET", "https://example.invalid/b")
        )
        session.cookies.clear()
        adapter = ReplayAdapter(
            [
                pair(
                    first,
                    headers=[
                        ["Set-Cookie", "session=token; Path=/"],
                        ["Set-Cookie", "other=value; Path=/"],
                    ],
                ),
                pair(second),
            ]
        )
        session.mount("https://", adapter)
        session.get("https://example.invalid/a")
        session.get("https://example.invalid/b")
        adapter.assert_consumed()
        session.close()

    def test_explicit_password_and_code_matchers(self):
        original = prepared(
            json={
                "password": "first-secret",
                "securityCode": {"code": "123456"},
            }
        )
        expectation = pair(original)
        text = json.dumps(expectation)
        self.assertNotIn("first-secret", text)
        self.assertNotIn("123456", text)
        replacement = prepared(
            json={
                "password": "different-secret",
                "securityCode": {"code": "654321"},
            }
        )
        adapter = ReplayAdapter([expectation])
        adapter.send(replacement)
        adapter.assert_consumed()
        for code in ["", "123", "abcdef", "１２３４５６"]:
            with self.subTest(code=code), self.assertRaises(ValueError):
                request_record(prepared(json={"securityCode": {"code": code}}))


class RecorderTests(unittest.TestCase):
    def test_actual_adapter_interception_and_redaction(self):
        with TemporaryDirectory() as directory:
            request = prepared(
                json={
                    "password": "never-save",
                    "securityCode": {"code": "123456"},
                }
            )
            response = requests.Response()
            response.status_code = 200
            response._content = b'{"items":[]}'
            response.headers["Content-Type"] = "application/json"
            with patch.object(
                requests.adapters.HTTPAdapter, "send", return_value=response
            ) as original:
                recorder = Recorder(
                    Path(directory) / "capture", {"commit": "synthetic"}, "login"
                )
                with recorder.installed(), requests.Session() as session:
                    session.send(request)
                original.assert_called_once()
                self.assertEqual(original.call_args.kwargs["timeout"], (15, 60))
            recorded = (recorder.directory / "0001.json").read_text()
            self.assertNotIn("never-save", recorded)
            self.assertNotIn("123456", recorded)
            exchange = json.loads(recorded)
            self.assertEqual(exchange["response"]["status"], 200)
            self.assertEqual(exchange["operation"], "login")
            self.assertIn("captured_at_utc", exchange)

    def test_failure_is_recorded_and_patch_is_restored(self):
        with TemporaryDirectory() as directory:
            with patch.object(
                requests.adapters.HTTPAdapter,
                "send",
                side_effect=requests.ConnectionError("private-message"),
            ) as original:
                recorder = Recorder(Path(directory) / "capture", {}, "devices")
                with self.assertRaises(requests.ConnectionError):
                    with recorder.installed(), requests.Session() as session:
                        session.send(prepared())
                self.assertIs(requests.adapters.HTTPAdapter.send, original)
            exchange = json.loads((recorder.directory / "0001.json").read_text())
            self.assertEqual(exchange["error"], "ConnectionError")
            self.assertNotIn("private-message", json.dumps(exchange))

    def test_invalid_inputs_fail_before_network(self):
        with TemporaryDirectory() as directory:
            with patch.object(requests.adapters.HTTPAdapter, "send") as send:
                recorder = Recorder(Path(directory) / "capture", {}, "login")
                with self.assertRaises(ValueError):
                    with recorder.installed(), requests.Session() as session:
                        session.send(prepared(json={"securityCode": {"code": "bad"}}))
                send.assert_not_called()


class ReferenceSessionTests(unittest.TestCase):
    def test_captured_scenario_replay_and_semantic_mismatch(self):
        service = icloud.verify_reference()
        with TemporaryDirectory() as directory:
            root = Path(directory)
            cookies = root / "cookies"
            api = service(
                "synthetic@example.invalid",
                authenticate=False,
                cookie_directory=str(cookies),
            )
            api.session.data["session_token"] = "synthetic-token"
            api.session.cookies.set(
                "X-APPLE-WEBAUTH-TOKEN",
                "synthetic-cookie",
                domain=".icloud.com",
                path="/",
            )
            api.session._save_session_data()
            request = api.session.prepare_request(
                requests.Request(
                    "POST",
                    api._setup_endpoint + "/validate",
                    data="null",
                )
            )
            api.session.close()
            data = {
                "dsInfo": {"dsid": "synthetic-account", "hsaVersion": 2},
                "hsaTrustedBrowser": True,
                "webservices": {"account": {"url": "https://example.invalid"}},
            }
            response = requests.Response()
            response.status_code = 200
            response._content = json.dumps(data).encode()
            response.headers["Content-Type"] = "application/json"
            recorder = Recorder(root / "capture", icloud.SOURCE["live"], "status")
            with (
                patch.object(
                    requests.adapters.HTTPAdapter,
                    "send",
                    return_value=response,
                ),
                recorder.installed(),
                requests.Session() as session,
            ):
                session.send(request)
            folder = recorder.directory
            shutil.copytree(cookies, folder / "initial-cookies")
            (folder / "scenario.json").write_text(
                json.dumps(
                    {
                        "source": icloud.SOURCE["live"],
                        "operation": "status",
                        "account": {
                            "username": "synthetic@example.invalid",
                            "china": False,
                        },
                        "inputs": {},
                    }
                )
            )
            result = folder / "result.json"
            result.write_text(json.dumps({"trusted": True, "services": ["account"]}))
            self.assertEqual(replay.replay(folder), 1)
            result.write_text(json.dumps({"trusted": False, "services": ["account"]}))
            with self.assertRaisesRegex(AssertionError, "semantic result mismatch"):
                replay.replay(folder)

    def test_real_reference_reuses_tokens_and_persists_response_headers(self):
        service = icloud.verify_reference()
        with TemporaryDirectory() as directory:
            api = service(
                "synthetic@example.invalid",
                authenticate=False,
                cookie_directory=directory,
                with_family=False,
            )
            api.session.data["session_token"] = "synthetic-token"
            api.session.cookies.set(
                "X-APPLE-WEBAUTH-TOKEN",
                "synthetic-cookie",
                domain=".icloud.com",
                path="/",
            )
            request = api.session.prepare_request(
                requests.Request(
                    "POST",
                    api._setup_endpoint + "/validate",
                    data="null",
                )
            )
            body = json.dumps(
                {
                    "dsInfo": {"dsid": "synthetic-account", "hsaVersion": 2},
                    "hsaTrustedBrowser": True,
                    "hsaChallengeRequired": False,
                    "webservices": {"account": {"url": "https://example.invalid"}},
                }
            ).encode()
            adapter = ReplayAdapter(
                [
                    pair(
                        request,
                        body=body,
                        headers=[
                            ["Content-Type", "application/json"],
                            ["X-Apple-Session-Token", "rotated-synthetic-token"],
                        ],
                    )
                ]
            )
            api.session.mount("https://", adapter)
            api.session.mount("http://", adapter)
            api.authenticate()
            adapter.assert_consumed()
            self.assertTrue(api.is_trusted_session)
            self.assertEqual(api.params["dsid"], "synthetic-account")
            self.assertEqual(
                api.session.data["session_token"],
                "rotated-synthetic-token",
            )
            api.session.close()
            loaded = service(
                "synthetic@example.invalid",
                authenticate=False,
                cookie_directory=directory,
            )
            self.assertEqual(
                loaded.session.data["session_token"],
                "rotated-synthetic-token",
            )
            self.assertEqual(
                loaded.session.cookies.get("X-APPLE-WEBAUTH-TOKEN"),
                "synthetic-cookie",
            )
            loaded.session.close()


class CLITests(unittest.TestCase):
    def test_account_and_device_reference_projections(self):
        from types import SimpleNamespace

        from pyicloud.services.account import AccountStorage

        storage = AccountStorage(
            {
                "storageUsageInfo": {
                    "usedStorageInBytes": 1,
                    "totalStorageInBytes": 10,
                },
            }
        )
        api = SimpleNamespace(
            account=SimpleNamespace(storage=storage, devices=[{"name": "synthetic"}]),
            devices=SimpleNamespace(
                devices={
                    "synthetic": SimpleNamespace(data={"id": "synthetic"}),
                }
            ),
        )
        self.assertEqual(
            icloud.run_read(api, SimpleNamespace(command="account"))["used_bytes"],
            1,
        )
        self.assertEqual(
            icloud.run_read(api, SimpleNamespace(command="devices")),
            [{"id": "synthetic"}],
        )

    def test_monitor_is_stopped_before_session_close(self):
        from types import SimpleNamespace
        from unittest.mock import Mock

        devices = SimpleNamespace(stop_event=Mock(), _monitor=Mock())
        devices._monitor.is_alive.return_value = False
        api = SimpleNamespace(_devices=devices, session=Mock())
        icloud.close_reference(api)
        devices.stop_event.set.assert_called_once()
        devices._monitor.join.assert_called_once_with(timeout=5)
        api.session.close.assert_called_once()

    def test_login_and_passwordless_session_reuse(self):
        with TemporaryDirectory() as directory, ExitStack() as stack:
            calls = []

            class Service:
                def __init__(self, username, **kwargs):
                    calls.append(kwargs)
                    self.session = requests.Session()
                    self.requires_2fa = kwargs["password"] is not None
                    self.requires_2sa = False
                    self.is_trusted_session = True
                    self.data = {"webservices": {"photos": {}}}

                def authenticate(self):
                    self.session.post("https://example.invalid/validate", data="null")

                def validate_2fa_code(self, code):
                    self.session.post(
                        "https://example.invalid/verify",
                        json={
                            "securityCode": {"code": code},
                        },
                    )
                    return code == "123456"

            response = requests.Response()
            response.status_code = 200
            response._content = b"{}"
            stack.enter_context(
                patch.object(
                    requests.adapters.HTTPAdapter,
                    "send",
                    return_value=response,
                )
            )
            stack.enter_context(
                patch.object(
                    icloud,
                    "verify_reference",
                    return_value=Service,
                )
            )
            stack.enter_context(
                patch.object(
                    icloud,
                    "state_root",
                    return_value=Path(directory),
                )
            )
            stack.enter_context(
                patch.object(
                    icloud,
                    "secure_directory",
                    side_effect=lambda p: p.mkdir(parents=True, exist_ok=True),
                )
            )
            password = stack.enter_context(
                patch.object(
                    icloud.getpass,
                    "getpass",
                    side_effect=["never-save", "123456"],
                )
            )
            stack.enter_context(patch("sys.stdout", new_callable=io.StringIO))
            self.assertEqual(
                icloud.main(
                    [
                        "login",
                        "--username",
                        "synthetic@example.invalid",
                    ]
                ),
                0,
            )
            self.assertEqual(icloud.main(["status"]), 0)
            self.assertEqual(password.call_count, 2)
            self.assertEqual(calls[0]["password"], "never-save")
            self.assertIsNone(calls[1]["password"])
            self.assertFalse(calls[0]["authenticate"])
            self.assertFalse(calls[0]["with_family"])
            for file in Path(directory).rglob("*.json"):
                self.assertNotIn("never-save", file.read_text())
                self.assertNotIn("123456", file.read_text())
            self.assertEqual(len(list(Path(directory).rglob("result.json"))), 2)

    def test_missing_login_and_invalid_selectors_do_not_send(self):
        with TemporaryDirectory() as directory, ExitStack() as stack:
            stack.enter_context(patch.object(icloud, "verify_reference"))
            stack.enter_context(
                patch.object(
                    icloud,
                    "state_root",
                    return_value=Path(directory),
                )
            )
            stack.enter_context(patch("sys.stderr", new_callable=io.StringIO))
            self.assertEqual(icloud.main(["status"]), 2)
        for args in [["devices", "--node", "folder"], ["photos", "--limit", "0"]]:
            with patch("sys.stderr", new_callable=io.StringIO):
                with self.assertRaises(SystemExit):
                    icloud.main(args)

    def test_provider_errors_do_not_print_personal_messages(self):
        with (
            patch.object(
                icloud,
                "verify_reference",
                side_effect=RuntimeError("private-email@example.invalid"),
            ),
            patch("sys.stderr", new_callable=io.StringIO) as output,
        ):
            self.assertEqual(icloud.main(["status"]), 1)
        self.assertNotIn("private-email", output.getvalue())

    def test_pinned_reference_is_installed(self):
        self.assertEqual(icloud.verify_reference().__name__, "PyiCloudService")
        with (
            patch.object(
                icloud.subprocess,
                "check_output",
                side_effect=["wrong-revision", ""],
            ),
            self.assertRaises(RuntimeError),
        ):
            icloud.verify_reference()


if __name__ == "__main__":
    unittest.main()

"""Private paired HTTP recording and strict offline replay for the reference."""

import base64
import io
import json
import re
from contextlib import contextmanager
from datetime import UTC, datetime
from email.message import Message
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch
from urllib.parse import parse_qsl, urlsplit

import requests
from urllib3._collections import HTTPHeaderDict
from urllib3.response import HTTPResponse


def redact_inputs(value):
    """Never persist a password or an entered verification code."""
    if isinstance(value, list):
        return [redact_inputs(item) for item in value]
    if not isinstance(value, dict):
        return value
    result = {}
    for key, item in value.items():
        if key.lower() == "password":
            if not isinstance(item, str) or not item:
                raise ValueError("Password must be nonempty")
            result[key] = "<redacted:nonempty-password>"
        elif key.lower() in {"code", "verificationcode"}:
            if not isinstance(item, str) or not re.fullmatch(r"[0-9]{6}", item):
                raise ValueError("Verification code must contain six digits")
            result[key] = "<redacted:six-digit-code>"
        else:
            result[key] = redact_inputs(item)
    return result


def body_record(body, *, request=False):
    if body is None:
        return {"encoding": "base64", "value": ""}
    if isinstance(body, str):
        body = body.encode("utf-8")
    if not isinstance(body, bytes):
        raise ValueError("Streaming request bodies are not supported by this recorder")
    if request:
        try:
            value = json.loads(body)
        except (ValueError, UnicodeDecodeError):
            pass
        else:
            scrubbed = redact_inputs(value)
            if scrubbed != value:
                return {"encoding": "json-redacted", "value": scrubbed}
    return {"encoding": "base64", "value": base64.b64encode(body).decode("ascii")}


def request_record(request):
    url = urlsplit(request.url)
    if url.username or url.password:
        raise ValueError("Credentials in URLs are not supported")
    return {
        "method": request.method,
        "origin": f"{url.scheme}://{url.netloc}",
        "path": url.path or "/",
        "query": [list(pair) for pair in parse_qsl(url.query, keep_blank_values=True)],
        "headers": [[key.lower(), value] for key, value in request.headers.items()],
        "body": body_record(request.body, request=True),
    }


def response_record(response):
    raw_headers = getattr(response.raw, "headers", None)
    if raw_headers is not None and hasattr(raw_headers, "getlist"):
        headers = [
            [key, value] for key in raw_headers for value in raw_headers.getlist(key)
        ]
    else:
        headers = list(map(list, response.headers.items()))
    return {
        "status": response.status_code,
        "headers": headers,
        "body": body_record(response.content),
    }


class Recorder:
    """Record each adapter exchange before reference-library response handling."""

    def __init__(self, directory, source, operation):
        self.directory = Path(directory)
        self.directory.mkdir(parents=True, exist_ok=False)
        self.source = source
        self.operation = operation
        self.count = 0

    @contextmanager
    def installed(self):
        original = requests.adapters.HTTPAdapter.send

        def send(adapter, request, **kwargs):
            # Validate and scrub sensitive interactive input before sending.
            expected = request_record(request)
            kwargs["timeout"] = kwargs.get("timeout") or (15, 60)
            started = datetime.now(UTC).isoformat()
            try:
                response = original(adapter, request, **kwargs)
                recorded_response = response_record(response)
            except requests.RequestException as error:
                self.write(expected, started, error=type(error).__name__)
                raise
            self.write(expected, started, response=recorded_response)
            return response

        with patch.object(requests.adapters.HTTPAdapter, "send", send):
            yield self

    def write(self, request, started, **outcome):
        self.count += 1
        exchange = {
            "format": "portos.http-exchange.v1",
            "evidence": "private-captured",
            "captured_at_utc": started,
            "source": self.source,
            "operation": self.operation,
            "sequence": self.count,
            "redactions": {
                "password": "nonempty string",
                "verification_code": "six ASCII digits",
            },
            "request": request,
            **outcome,
        }
        destination = self.directory / f"{self.count:04d}.json"
        destination.write_text(json.dumps(exchange, indent=2) + "\n", encoding="utf-8")


def comparable(request):
    value = dict(request)
    value["headers"] = sorted(request["headers"])
    # A redacted body has a structural matcher; its original wire size differs.
    if request["body"]["encoding"] == "json-redacted":
        value["headers"] = [p for p in value["headers"] if p[0] != "content-length"]
    return value


class ReplayAdapter(requests.adapters.BaseAdapter):
    """Reject any mismatch before exposing its response; never contact a network."""

    def __init__(self, exchanges):
        super().__init__()
        self.exchanges = list(exchanges)
        self.index = 0

    def send(self, request, **kwargs):
        if self.index == len(self.exchanges):
            raise AssertionError("Unexpected or duplicate HTTP request")
        pair = self.exchanges[self.index]
        if comparable(request_record(request)) != comparable(pair["request"]):
            raise AssertionError(f"HTTP request mismatch at exchange {self.index + 1}")
        self.index += 1
        if "error" in pair:
            error_type = getattr(requests.exceptions, pair["error"], None)
            if not isinstance(error_type, type) or not issubclass(
                error_type, requests.RequestException
            ):
                raise AssertionError("Unknown recorded transport failure")
            raise error_type("Recorded transport failure")
        wire = pair["response"]
        if wire["body"]["encoding"] != "base64":
            raise AssertionError("Unsupported response body encoding")
        response = requests.Response()
        response.status_code = wire["status"]
        response.headers.update(wire["headers"])
        response._content = base64.b64decode(wire["body"]["value"], validate=True)
        response.request = request
        response.url = request.url
        message = Message()
        for key, value in wire["headers"]:
            message[key] = value
        response.raw = HTTPResponse(
            body=io.BytesIO(response.content),
            headers=HTTPHeaderDict(wire["headers"]),
            preload_content=False,
        )
        response.raw._original_response = SimpleNamespace(msg=message)
        requests.cookies.extract_cookies_to_jar(response.cookies, request, response.raw)
        return response

    def assert_consumed(self):
        if self.index != len(self.exchanges):
            raise AssertionError("Unconsumed HTTP exchanges")

    def close(self):
        pass

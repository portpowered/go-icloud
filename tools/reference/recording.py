"""Private paired HTTP recording and strict offline replay for the reference."""

import base64
import io
import json
import re
from contextlib import contextmanager
from datetime import UTC, datetime
from email.message import Message
from email.parser import BytesParser
from email.policy import default as email_policy
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


def multipart_parts(content_type, body):
    """Decode ordered multipart fields without ignoring their payload bytes."""
    message = BytesParser(policy=email_policy).parsebytes(
        b"Content-Type: " + content_type.encode("ascii") + b"\r\n\r\n" + body
    )
    if (
        not message.is_multipart()
        or message.defects
        or message.preamble is not None
        or message.epilogue not in (None, "")
    ):
        raise AssertionError("Invalid multipart request")
    parts = []
    for part in message.iter_parts():
        if part.defects or part.is_multipart():
            raise AssertionError("Invalid multipart field")
        parts.append(
            {
                "headers": [[key.lower(), value] for key, value in part.items()],
                "body": body_record(part.get_payload(decode=True)),
            }
        )
    return parts


def matches_request(actual, expected):
    """Apply only explicit fixture match rules, keeping all other fields strict."""
    actual = comparable(actual)
    expected = comparable(expected)
    rule = expected["body"]
    if rule["encoding"] == "json-pattern":
        if actual["body"]["encoding"] != "base64":
            return False
        value = json.loads(base64.b64decode(actual["body"]["value"], validate=True))
        for matcher in rule["matchers"]:
            parent = value
            sample = rule["value"]
            for key in matcher["path"][:-1]:
                parent = parent[key]
                sample = sample[key]
            key = matcher["path"][-1]
            if not isinstance(parent[key], str) or not re.fullmatch(
                matcher["pattern"], parent[key]
            ):
                return False
            parent[key] = sample[key]
        if json.dumps(value, sort_keys=True) != json.dumps(
            rule["value"], sort_keys=True
        ):
            return False
        actual["body"] = rule
    elif rule["encoding"] == "multipart":
        content_type = dict(actual["headers"]).get("content-type", "")
        if not re.fullmatch(rule["content_type_pattern"], content_type):
            return False
        if actual["body"]["encoding"] != "base64":
            return False
        parts = multipart_parts(
            content_type, base64.b64decode(actual["body"]["value"], validate=True)
        )
        if parts != rule["parts"]:
            return False
        actual["body"] = rule
        actual["headers"] = [
            [key, dict(expected["headers"])[key] if key == "content-type" else value]
            for key, value in actual["headers"]
        ]
    return actual == expected


class ReplayAdapter(requests.adapters.BaseAdapter):
    """Reject any mismatch before exposing its response; never contact a network."""

    def __init__(self, exchanges):
        super().__init__()
        self.exchanges = list(exchanges)
        self.index = 0
        self.failed = False

    def send(self, request, **kwargs):
        if self.failed:
            raise AssertionError("Replay already rejected HTTP traffic")
        try:
            return self._send(request, **kwargs)
        except AssertionError:
            self.failed = True
            raise

    def _send(self, request, **kwargs):
        if self.index == len(self.exchanges):
            raise AssertionError("Unexpected or duplicate HTTP request")
        pair = self.exchanges[self.index]
        length = request.headers.get("Content-Length")
        if length is not None:
            body = request.body or b""
            if isinstance(body, str):
                body = body.encode("utf-8")
            if (
                not isinstance(body, bytes)
                or not re.fullmatch("[0-9]+", length)
                or int(length) != len(body)
            ):
                raise AssertionError("Request Content-Length does not match body bytes")
        try:
            matched = matches_request(request_record(request), pair["request"])
        except (ValueError, KeyError, IndexError, TypeError) as error:
            raise AssertionError("Invalid request or fixture match rule") from error
        if not matched:
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
        if self.failed:
            raise AssertionError("Rejected HTTP traffic was caught by reference code")
        if self.index != len(self.exchanges):
            raise AssertionError("Unconsumed HTTP exchanges")

    def close(self):
        pass

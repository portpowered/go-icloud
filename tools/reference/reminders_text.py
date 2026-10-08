"""Offline reference replay for portable Reminders text documents."""

import base64

from icloud import SOURCE
from pyicloud.services.reminders._protocol import _decode_crdt_document


def replay_reminders_text(corpus):
    if corpus["format"] != "portos.reminders-text.v1":
        raise ValueError("Unsupported Reminders text corpus")
    if corpus["source"] != SOURCE["live"] or corpus["evidence"] != (
        "synthetic; implementation-derived"
    ):
        raise ValueError("Reminders document provenance changed")
    for case in corpus["cases"]:
        if ("result" in case) == ("error" in case):
            raise ValueError("Document requires exactly one outcome")
        if case["input"]["encoding"] != "base64":
            raise ValueError("Unsupported document encoding")
        data = base64.b64decode(case["input"]["value"], validate=True)
        try:
            result = _decode_crdt_document(data)
        except Exception as failure:
            expected = case.get("error")
            if expected is None or expected != {
                "type": type(failure).__name__,
                "message": str(failure),
            }:
                raise ValueError("Reference document failure changed") from failure
        else:
            if "error" in case or "result" not in case:
                raise ValueError("Reference document unexpectedly succeeded")
            raw = result if isinstance(result, bytes) else result.encode("utf-8")
            expected = {
                "encoding": "base64",
                "value": base64.b64encode(raw).decode(),
                "source_type": type(result).__name__,
            }
            if case["result"] != expected:
                raise ValueError("Reference decoded document changed")
    return len(corpus["cases"])

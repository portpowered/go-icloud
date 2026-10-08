"""Replay a private captured read scenario without contacting Apple."""

import argparse
import json
import logging
import shutil
import sys
from datetime import datetime
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest.mock import patch

from icloud import (
    SOURCE,
    close_reference,
    run_read,
    serialize,
    state_root,
    verify_reference,
)
from network_guard import forbid_network
from recording import ReplayAdapter


def replay(directory):
    directory = Path(directory)
    scenario = json.loads((directory / "scenario.json").read_text(encoding="utf-8"))
    if scenario["source"] != SOURCE["live"] or scenario["operation"] == "login":
        raise ValueError("Replay requires a read scenario from the pinned live source")
    pairs = [
        json.loads(file.read_text(encoding="utf-8"))
        for file in sorted(directory.glob("[0-9][0-9][0-9][0-9].json"))
    ]
    if not pairs:
        raise ValueError("No recorded exchanges")
    for index, pair in enumerate(pairs, 1):
        if pair["sequence"] != index or pair["source"] != scenario["source"]:
            raise ValueError("Invalid exchange sequence or source")
    capture_time = datetime.fromisoformat(pairs[0]["captured_at_utc"]).timestamp()
    expected_error = directory / "error.json"
    if not expected_error.exists() and not (directory / "result.json").exists():
        raise ValueError("Capture has no completed result or recorded failure")
    error_type = (
        json.loads(expected_error.read_text(encoding="utf-8"))["type"]
        if expected_error.exists()
        else None
    )
    adapter = ReplayAdapter(pairs)
    service = verify_reference()
    with TemporaryDirectory() as temporary:
        cookies = Path(temporary) / "cookies"
        shutil.copytree(directory / "initial-cookies", cookies)
        api = None
        with (
            patch("time.time", return_value=capture_time),
            forbid_network(),
        ):
            try:
                identity = scenario["account"]
                api = service(
                    identity["username"],
                    cookie_directory=str(cookies),
                    china_mainland=identity["china"],
                    with_family=False,
                    authenticate=False,
                )
                api.session.mount("https://", adapter)
                api.session.mount("http://", adapter)
                api.authenticate()
                args = argparse.Namespace(
                    command=scenario["operation"], **scenario["inputs"]
                )
                result = run_read(api, args)
                if error_type:
                    raise AssertionError("Expected reference failure was not raised")
                expected = json.loads(
                    (directory / "result.json").read_text(encoding="utf-8")
                )
                actual = json.loads(json.dumps(result, default=serialize))
                if actual != expected:
                    raise AssertionError("Reference semantic result mismatch")
            except AssertionError:
                raise
            except Exception as error:
                if type(error).__name__ != error_type:
                    raise AssertionError(
                        "Unexpected reference failure during replay"
                    ) from None
            finally:
                if api:
                    close_reference(api)
            adapter.assert_consumed()
    return adapter.index


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory", type=Path, nargs="?")
    parser.add_argument(
        "--all", action="store_true", help="Replay completed private reads"
    )
    args = parser.parse_args()
    if bool(args.directory) == args.all:
        parser.error("Provide a capture directory or --all")
    logging.disable(logging.CRITICAL)
    try:
        directories = [args.directory]
        if args.all:
            directories = [
                folder
                for folder in sorted((state_root() / "captures").iterdir())
                if (folder / "initial-cookies").is_dir()
                and (
                    (folder / "result.json").exists()
                    or (folder / "error.json").exists()
                )
                and json.loads((folder / "scenario.json").read_text())["operation"]
                != "login"
            ]
        if not directories:
            raise ValueError("No completed replayable captures")
        count = sum(replay(folder) for folder in directories)
    except Exception as error:
        print(f"Replay failed ({type(error).__name__}).", file=sys.stderr)
        return 1
    print(
        f"Offline replay passed: {len(directories)} scenarios, {count} exchanges "
        "and matching semantic results/failures."
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())

"""Interactive reference CLI used to collect evidence for the Go migration."""

import argparse
import csv
import getpass
import hashlib
import json
import logging
import os
import shutil
import subprocess
import sys
from datetime import UTC, datetime
from itertools import islice
from pathlib import Path
from uuid import uuid4

from recording import Recorder

ROOT = Path(__file__).resolve().parents[2]
SOURCE = json.loads((Path(__file__).parent / "source.json").read_text())


def secure_directory(directory):
    directory.mkdir(parents=True, exist_ok=True)
    if os.name == "nt":
        identity = subprocess.check_output(
            ["whoami", "/user", "/fo", "csv", "/nh"], text=True
        )
        sid = next(csv.reader([identity.strip()]))[1]
        subprocess.run(
            [
                "icacls",
                str(directory),
                "/inheritance:r",
                "/grant:r",
                f"*{sid}:(OI)(CI)F",
            ],
            check=True,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
    else:
        directory.chmod(0o700)


def state_root():
    local = os.environ.get("LOCALAPPDATA")
    base = Path(local) if local else Path.home() / ".local" / "share"
    directory = base / "go-icloud" / "reference"
    secure_directory(directory)
    return directory


def verify_reference():
    checkout = ROOT / ".reference" / "pyicloud-live"
    revision = subprocess.check_output(
        ["git", "-C", str(checkout), "rev-parse", "HEAD"], text=True
    ).strip()
    dirty = subprocess.check_output(
        ["git", "-C", str(checkout), "status", "--porcelain"], text=True
    ).strip()
    if revision != SOURCE["live"]["commit"] or dirty:
        raise RuntimeError("Reference checkout does not match the pinned clean source")
    import pyicloud

    if Path(pyicloud.__file__).resolve().parent != checkout / "pyicloud":
        raise RuntimeError("Run the CLI using the repository virtual environment")
    return pyicloud.PyiCloudService


def serialize(value):
    if hasattr(value, "model_dump"):
        return value.model_dump(mode="json")
    if isinstance(value, datetime):
        return value.isoformat()
    raise TypeError(f"Unsupported result type: {type(value).__name__}")


def run_read(api, args):
    if args.command == "status":
        return {
            "trusted": api.is_trusted_session,
            "services": sorted(api.data.get("webservices", {})),
        }
    if args.command == "account":
        account = api.account
        storage = account.storage
        return {
            "devices": [dict(device) for device in account.devices],
            "used_bytes": storage.usage.used_storage_in_bytes,
            "total_bytes": storage.usage.total_storage_in_bytes,
        }
    if args.command == "devices":
        return [device.data for device in api.devices.devices.values()]
    if args.command == "drive":
        drive = api.drive
        if args.node:
            from pyicloud.services.drive import DriveNode

            folder = DriveNode(drive, drive.get_node_data(args.node))
        else:
            folder = drive.root
        return [child.data for child in islice(folder.get_children(), args.limit)]
    if args.command == "albums":
        return [
            {"id": album.id, "name": album.name}
            for album in islice(api.photos.albums, args.limit)
        ]
    if args.command == "photos":
        photos = api.photos
        album = photos.albums[args.album] if args.album else photos.all
        return [
            {"id": photo.id, "filename": photo.filename}
            for photo in islice(album, args.limit)
        ]
    if args.command == "reminder-zones":
        return api.reminders._raw._client.zones_list()
    if args.command == "reminders":
        service = api.reminders
        if args.list_id:
            return service.list_reminders(
                args.list_id, include_completed=True, results_limit=args.limit
            )
        return list(islice(service.lists(), args.limit))
    raise ValueError("Unknown read operation")


def complete_login(api):
    if api.requires_2fa:
        print("Enter the verification code delivered by Apple.")
        code = getpass.getpass("Verification code: ")
        if len(code) != 6 or not code.isascii() or not code.isdigit():
            raise ValueError("Verification code must contain six ASCII digits")
        if not api.validate_2fa_code(code):
            raise RuntimeError("Verification was rejected")
    elif api.requires_2sa:
        raise RuntimeError("Legacy two-step authentication is not supported yet")
    if not api.is_trusted_session and not api.trust_session():
        raise RuntimeError("Session trust was rejected")


def close_reference(api):
    """Stop reference-owned polling before closing its HTTP session."""
    devices = getattr(api, "_devices", None)
    if devices is not None:
        devices.stop_event.set()
        monitor = getattr(devices, "_monitor", None)
        if monitor is not None:
            monitor.join(timeout=5)
            if monitor.is_alive():
                raise RuntimeError("Find My monitor did not stop")
    cleanup = getattr(api, "_clear_trusted_device_bridge_state", None)
    if cleanup:
        cleanup()
    api.session.close()


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "command",
        choices=[
            "login",
            "status",
            "account",
            "devices",
            "drive",
            "albums",
            "photos",
            "reminders",
            "reminder-zones",
        ],
    )
    parser.add_argument("--username", help="Apple ID; required for first login")
    parser.add_argument("--china", action="store_true", help="Mainland China account")
    parser.add_argument(
        "--limit", type=int, default=20, help="Maximum returned items (1..1000)"
    )
    parser.add_argument("--node", help="Drive folder ID for drive")
    parser.add_argument("--album", help="Album ID or name for photos")
    parser.add_argument("--list-id", help="Reminder list ID for reminders")
    args = parser.parse_args(argv)
    if not 1 <= args.limit <= 1000:
        parser.error("--limit must be between 1 and 1000")
    if args.username and args.command != "login":
        parser.error("Use login to select an account")
    if (
        (args.node and args.command != "drive")
        or (args.album and args.command != "photos")
        or (args.list_id and args.command != "reminders")
    ):
        parser.error("The selector does not apply to this command")
    # Upstream errors can contain URLs, account names, and raw provider messages.
    logging.disable(logging.CRITICAL)
    recorder = None
    api = None
    try:
        service_type = verify_reference()
        state = state_root()
        identity_path = state / "active-account.json"
        if args.command == "login":
            username = args.username or input("Apple ID: ").strip()
            if not username:
                raise ValueError("Apple ID is required")
            identity = {"username": username, "china": args.china}
            password = getpass.getpass("Apple ID password (not saved): ")
            if not password:
                raise ValueError("Password is required")
        else:
            if not identity_path.exists():
                print("Log in first: .\\icloud.ps1 login", file=sys.stderr)
                return 2
            identity = json.loads(identity_path.read_text(encoding="utf-8"))
            password = None
        account_key = hashlib.sha256(identity["username"].encode()).hexdigest()[:24]
        cookies = state / "accounts" / account_key
        secure_directory(cookies)
        stamp = datetime.now(UTC).strftime("%Y%m%dT%H%M%SZ")
        recording_path = (
            state / "captures" / f"{stamp}-{args.command}-{uuid4().hex[:8]}"
        )
        recorder = Recorder(recording_path, SOURCE["live"], args.command)
        # Private snapshots let replay reproduce the initial token/cookie state.
        shutil.copytree(cookies, recording_path / "initial-cookies")
        (recording_path / "scenario.json").write_text(
            json.dumps(
                {
                    "format": "portos.reference-scenario.v1",
                    "source": SOURCE["live"],
                    "evidence": "private-captured",
                    "operation": args.command,
                    "account": identity,
                    "inputs": {
                        "limit": args.limit,
                        "node": args.node,
                        "album": args.album,
                        "list_id": args.list_id,
                        "china": identity["china"],
                    },
                    "result": "result.json",
                    "http_exchanges": "numbered JSON files in sequence order",
                    "uncaptured_edges": ["trusted-device bridge TLS/socket frames"],
                },
                indent=2,
            )
            + "\n",
            encoding="utf-8",
        )
        with recorder.installed():
            api = service_type(
                identity["username"],
                password=password,
                cookie_directory=str(cookies),
                china_mainland=identity["china"],
                with_family=False,
                authenticate=False,
            )
            api.authenticate()
            if args.command == "login":
                complete_login(api)
                identity_path.write_text(json.dumps(identity), encoding="utf-8")
                result = {"authenticated": True, "trusted": api.is_trusted_session}
            else:
                if api.requires_2fa or api.requires_2sa:
                    raise RuntimeError("Session expired; run login again")
                result = run_read(api, args)
            (recording_path / "result.json").write_text(
                json.dumps(result, default=serialize, indent=2) + "\n", encoding="utf-8"
            )
        count = len(result) if isinstance(result, list) else None
        print(
            f"{args.command}: completed"
            + (f" ({count} results)" if count is not None else "")
        )
        print(
            f"Recorded {recorder.count} HTTP exchanges. "
            f"Private results: {recording_path}"
        )
        return 0
    except KeyboardInterrupt:
        print("Cancelled.", file=sys.stderr)
        return 130
    except Exception as error:
        print(f"Command failed ({type(error).__name__}).", file=sys.stderr)
        if recorder:
            (recorder.directory / "error.json").write_text(
                json.dumps({"type": type(error).__name__}) + "\n", encoding="utf-8"
            )
            print(f"Private exchanges: {recorder.directory}", file=sys.stderr)
        if "Auth" in type(error).__name__ or "Login" in type(error).__name__:
            print("Authentication failed; run login again.", file=sys.stderr)
        return 1
    finally:
        if api:
            close_reference(api)


if __name__ == "__main__":
    sys.exit(main())

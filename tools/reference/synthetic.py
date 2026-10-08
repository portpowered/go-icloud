"""Run portable, implementation-derived service scenarios at the HTTP seam."""

import argparse
import base64
import io
import json
from dataclasses import asdict
from tempfile import TemporaryDirectory

from icloud import ROOT, SOURCE, close_reference, verify_reference
from network_guard import forbid_network
from recording import ReplayAdapter
from scenario_inputs import decode_input, project, synthetic_entropy

FIXTURES = ROOT / "tests" / "replay" / "fixtures" / "synthetic" / "http"


def execute(api, scenario, observations=None):
    from pyicloud.services.account import AccountService
    from pyicloud.services.drive import DriveNode, DriveService

    state = scenario["initial_state"]
    params = dict(state["params"])
    if scenario["service"] == "findmy":
        from pyicloud.services.findmyiphone import FindMyiPhoneServiceManager

        manager = FindMyiPhoneServiceManager(
            state["origin"],
            state["token_origin"],
            api.session,
            params,
            with_family=False,
            refresh_interval=86400,
        )
        api._devices = manager
        if scenario["operation"] == "devices":
            return [device.data for device in manager.devices.values()]
        device = manager[scenario["device_id"]]
        return getattr(device, scenario["operation"])(*scenario["inputs"])
    if scenario["service"] == "drive":
        service = DriveService(
            state["origin"], state["document_origin"], api.session, params
        )
        arguments = scenario["inputs"]
        if scenario["operation"] == "move_nodes_to_node":
            arguments = [
                [DriveNode(service, node) for node in arguments[0]],
                DriveNode(service, arguments[1]),
            ]
        if scenario["operation"] == "send_file":
            descriptor = arguments[1]
            file = io.BytesIO(base64.b64decode(descriptor["body"], validate=True))
            file.name = descriptor["name"]
            arguments = [arguments[0], file, *arguments[2:]]
        result = getattr(service, scenario["operation"])(
            *arguments, **scenario.get("keyword_inputs", {})
        )
        if scenario["operation"] == "get_file":
            return {
                "status": result.status_code,
                "headers": list(map(list, result.headers.items())),
                "body": base64.b64encode(result.content).decode("ascii"),
            }
        return result
    if scenario["service"] == "account":
        service = AccountService(state["origin"], api.session, False, params)
        value = getattr(service, scenario["operation"])
        if scenario["operation"] == "family":
            return [member.full_name for member in value]
        if scenario["operation"] == "storage":
            return {
                "used_bytes": value.usage.used_storage_in_bytes,
                "total_bytes": value.usage.total_storage_in_bytes,
            }
        return value
    if scenario["service"] == "reminders":
        from pyicloud.services.reminders.service import RemindersService

        service = RemindersService(state["origin"], api.session, params)
        operation = scenario["operation"]
        arguments = decode_input(scenario["inputs"])
        try:
            value = getattr(service, operation)(
                *arguments, **decode_input(scenario.get("keyword_inputs", {}))
            )
        finally:
            if observations is not None and "observe_arguments" in scenario:
                observations["arguments"] = [
                    project(arguments[index]) for index in scenario["observe_arguments"]
                ]
        if "observe_arguments" in scenario:
            return {
                "value": project(value),
                "arguments": [
                    project(arguments[index]) for index in scenario["observe_arguments"]
                ],
            }
        if operation in {"lists", "iter_changes", "reminders"}:
            return [item.model_dump(mode="json") for item in value]
        return project(value)
    if scenario["service"] == "photos":
        from pyicloud.services.photos import PhotosService

        service = PhotosService(
            state["origin"],
            api.session,
            params,
            None,
            state.get("shared_streams_origin"),
        )
        operation = scenario["operation"]
        if operation == "indexing":
            return {
                "state": service._root_library.indexing_state,
                "sync_token": service._root_library.current_sync_token,
            }
        if operation == "libraries":
            return [
                {"id": key, "scope": library.scope}
                for key, library in service.libraries.items()
            ]
        if operation == "shared_streams":
            return list(service.shared_streams)
        if operation in {
            "albums_snapshot",
            "create_album",
            "album_count",
            "album_photos",
            "album_rename",
            "album_delete",
            "album_add_photo",
            "photo_get",
            "photo_download",
            "photo_favorite",
            "photo_delete",
            "upload_reserve",
            "upload_register",
            "upload_status",
            "upload_bytes",
            "upload_pipeline",
            "stream_albums",
            "stream_count",
            "stream_photos",
            "stream_get",
            "stream_download",
        }:
            from photo_scenarios import execute_photos

            return execute_photos(service, scenario)
        value = getattr(service, operation)(
            *scenario["inputs"], **scenario.get("keyword_inputs", {})
        )
        if operation == "iter_changes":
            return [
                {
                    **asdict(item),
                    "modified": item.modified.isoformat() if item.modified else None,
                }
                for item in value
            ]
        return value
    raise ValueError("Unregistered synthetic service")


def replay_synthetic(path):
    scenario = json.loads(path.read_text(encoding="utf-8"))
    if scenario.get("format") != "portos.service-scenario.v1":
        raise AssertionError("Unsupported service scenario format")
    if scenario["source"] != SOURCE["live"]:
        raise AssertionError("Synthetic source pin mismatch")
    if scenario["evidence"] != "synthetic; implementation-derived":
        raise AssertionError("Synthetic evidence classification mismatch")
    service = verify_reference()
    with TemporaryDirectory() as directory, forbid_network():
        api = service(
            "synthetic@example.invalid",
            authenticate=False,
            client_id="synthetic-client",
            cookie_directory=directory,
            with_family=False,
        )
        api.session.headers.clear()
        api.session.headers.update(scenario["initial_state"]["headers"])
        api.session.data.update(scenario["initial_state"].get("session_data", {}))
        for cookie in scenario["initial_state"].get("cookies", []):
            api.session.cookies.set(**cookie)
        adapter = ReplayAdapter(scenario["exchanges"])
        api.session.mount("https://", adapter)
        api.session.mount("http://", adapter)
        try:
            observations = {}
            try:
                with synthetic_entropy(scenario):
                    result = execute(api, scenario, observations)
            except AssertionError:
                raise
            except Exception as error:
                from pyicloud.common.cloudkit.client import CloudKitApiError

                expected = scenario.get("error")
                if expected != {"type": type(error).__name__, "message": str(error)}:
                    raise AssertionError("Synthetic semantic error mismatch") from error
                if isinstance(error, CloudKitApiError) and (
                    "error_payload" not in scenario
                    or scenario["error_payload"] != project(error.payload)
                ):
                    raise AssertionError("Synthetic error payload mismatch") from error
                if scenario.get("error_arguments") != observations.get("arguments"):
                    raise AssertionError(
                        "Synthetic error argument state mismatch"
                    ) from error
            else:
                if (
                    "error" in scenario
                    or json.loads(json.dumps(result)) != scenario["result"]
                ):
                    raise AssertionError("Synthetic semantic result mismatch")
            adapter.assert_consumed()
        finally:
            close_reference(api)
    return len(scenario["exchanges"])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.parse_args()
    paths = sorted(FIXTURES.glob("*.json"))
    count = sum(replay_synthetic(path) for path in paths)
    print(f"Synthetic replay passed: {len(paths)} scenarios, {count} paired exchanges")


if __name__ == "__main__":
    main()

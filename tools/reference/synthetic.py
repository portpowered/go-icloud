"""Run portable, implementation-derived service scenarios at the HTTP seam."""

import argparse
import base64
import io
import json
from contextlib import contextmanager
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
    if scenario["service"] == "auth":
        from auth_scenarios import execute_auth

        return execute_auth(api, scenario, observations)
    if scenario["service"] == "findmy":
        from pyicloud.services.findmyiphone import FindMyiPhoneServiceManager

        try:
            with observe_findmy_refresh(api, scenario, observations):
                manager = FindMyiPhoneServiceManager(
                    state["origin"],
                    state["token_origin"],
                    api.session,
                    params,
                    with_family=state.get("with_family", False),
                    refresh_interval=86400,
                    family_poll_delay=state.get("family_poll_delay", 0.5),
                    family_poll_max_retries=state.get("family_poll_max_retries", 5),
                )
        finally:
            if (
                observations is not None
                and scenario["operation"] == "devices_with_auth_state"
            ):
                from auth_scenarios import auth_state

                observations["auth_state"] = auth_state(api)
        api._devices = manager
        if scenario["operation"] == "monitor_flow":
            from findmy_monitor import execute_monitor

            return execute_monitor(manager, scenario)
        if scenario["operation"] == "device_description":
            from findmy_scenarios import execute_description

            return execute_description(manager, scenario)
        if scenario["operation"] == "refresh_flow":
            from findmy_scenarios import execute_refresh

            return execute_refresh(manager, scenario, observations)
        if scenario["operation"] == "devices_with_auth_state":
            from auth_scenarios import auth_state

            return {
                "devices": [device.data for device in manager.devices.values()],
                "auth_state": auth_state(api),
            }
        if scenario["operation"] == "devices":
            return [device.data for device in manager.devices.values()]
        device = manager[scenario["device_id"]]
        return getattr(device, scenario["operation"])(*scenario["inputs"])
    if scenario["service"] == "drive":
        service = DriveService(
            state["origin"], state["document_origin"], api.session, params
        )
        arguments = scenario["inputs"]
        file = None
        if scenario["operation"] == "move_nodes_to_node":
            arguments = [
                [DriveNode(service, node) for node in arguments[0]],
                DriveNode(service, arguments[1]),
            ]
        if scenario["operation"] == "send_file":
            descriptor = arguments[1]
            file = io.BytesIO(base64.b64decode(descriptor["body"], validate=True))
            file.name = descriptor["name"]
            file.seek(descriptor.get("position", 0))
            arguments = [arguments[0], file, *arguments[2:]]
        try:
            if scenario["operation"] == "node_flow":
                from drive_scenarios import execute_nodes

                return execute_nodes(service, scenario, observations)
            result = getattr(service, scenario["operation"])(
                *arguments, **scenario.get("keyword_inputs", {})
            )
            if scenario["operation"] == "get_file":
                try:
                    return {
                        "status": result.status_code,
                        "headers": list(map(list, result.headers.items())),
                        "body": base64.b64encode(result.content).decode("ascii"),
                    }
                finally:
                    result.close()
            if scenario.get("observe_transfer"):
                return {
                    "value": result,
                    "params": dict(service.params),
                    "file_position": file.tell() if file is not None else None,
                }
            return result
        finally:
            if observations is not None:
                observations["arguments"] = scenario["inputs"]
                node_position = observations.get("drive_state", {}).get("file_position")
                observations["drive_state"] = {
                    "params": dict(service.params),
                    "file_position": file.tell() if file is not None else node_position,
                }
            if file is not None:
                file.close()
    if scenario["service"] == "account":
        service = AccountService(
            state["origin"], api.session, state.get("china_mainland", False), params
        )
        if scenario["operation"] == "family_photos":
            photos = []
            for member in service.family:
                response = member.get_photo()
                try:
                    photos.append(
                        {
                            "member_id": member.dsid,
                            "status": response.status_code,
                            "headers": list(map(list, response.headers.items())),
                            "body": base64.b64encode(response.raw.read()).decode(
                                "ascii"
                            ),
                        }
                    )
                finally:
                    response.close()
            return photos
        value = getattr(service, scenario["operation"])
        if scenario["operation"] == "family":
            return [member.full_name for member in value]
        if scenario["operation"] == "storage":
            return {
                "used_bytes": value.usage.used_storage_in_bytes,
                "total_bytes": value.usage.total_storage_in_bytes,
            }
        return value
    if scenario["service"] == "reminders" and scenario["operation"] == "legacy_startup":
        response = api.session.get(state["origin"] + "/rd/startup", params=params)
        try:
            payload = response.json()
            return {"lists": payload["Collections"], "reminders": payload["Reminders"]}
        finally:
            response.close()
    if scenario["service"] == "reminders":
        from pyicloud.services.reminders.service import RemindersService

        service = RemindersService(state["origin"], api.session, params)
        operation = scenario["operation"]
        if operation == "zones":
            return project(service._raw._client.zones_list())
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
            photos_upload_url=state.get("photos_upload_origin"),
            **{
                key: state[key]
                for key in ["upload_hydration_timeout", "upload_hydration_interval"]
                if key in state
            },
        )
        operation = scenario["operation"]
        if operation in {"container_changes", "container_lookup"}:
            scope = scenario.get("container_scope", "private")
            assert scope in {"private", "shared"}, "Unknown Photos container scope"
            client = (
                service.private_client if scope == "private" else service._shared_client
            )
            kwargs = dict(scenario.get("keyword_inputs", {}))
            if operation == "container_lookup":
                from pyicloud.common.cloudkit import CKZoneIDReq

                kwargs["zone_id"] = CKZoneIDReq(**kwargs["zone_id"])
                return project(client.lookup(**kwargs))
            return project(client.database_changes(**kwargs))
        if "library" in scenario:
            service = service.libraries[scenario["library"]]
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
            "recently_added_photos",
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
            "service_upload",
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
            try:
                return [
                    {
                        **asdict(item),
                        "modified": (
                            item.modified.isoformat() if item.modified else None
                        ),
                    }
                    for item in value
                ]
            finally:
                if observations is not None:
                    library = getattr(service, "_root_library", service)
                    observations["photos_sync_token"] = library.current_sync_token
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
            scenario["initial_state"].get("account_name", "synthetic@example.invalid"),
            password=scenario["initial_state"].get("synthetic_password"),
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
                from bridge_replay import bridge_network

                with (
                    synthetic_entropy(scenario),
                    bridge_network(api, scenario, adapter),
                ):
                    result = execute(api, scenario, observations)
            except AssertionError:
                raise
            except Exception as error:
                from pyicloud.common.cloudkit.client import CloudKitApiError
                from pyicloud.services.reminders.client import RemindersApiError

                expected = scenario.get("error")
                if expected != {"type": type(error).__name__, "message": str(error)}:
                    raise AssertionError("Synthetic semantic error mismatch") from error
                if isinstance(error, (CloudKitApiError, RemindersApiError)) and (
                    "error_payload" not in scenario
                    or scenario["error_payload"] != project(error.payload)
                ):
                    raise AssertionError("Synthetic error payload mismatch") from error
                if scenario["service"] == "photos":
                    from photo_scenarios import album_result, photo_result
                    from pyicloud.services.photos_cloudkit.models import (
                        PhotosServiceException,
                    )

                    if isinstance(error, PhotosServiceException):
                        resources = {
                            "photo": photo_result(error.photo)
                            if error.photo is not None
                            else None,
                            "album": album_result(error.album)
                            if error.album is not None
                            else None,
                        }
                        if "error_resources" not in scenario or (
                            scenario["error_resources"] != resources
                        ):
                            raise AssertionError(
                                "Synthetic Photos error resource mismatch"
                            ) from error
                if scenario.get("error_arguments") != observations.get("arguments"):
                    raise AssertionError(
                        "Synthetic error argument state mismatch"
                    ) from error
                if scenario["service"] == "auth" and scenario.get(
                    "error_auth_state"
                ) != observations.get("auth_state"):
                    raise AssertionError(
                        "Synthetic auth error state mismatch"
                    ) from error
                if (
                    scenario["service"] == "findmy"
                    and scenario["operation"] == "devices_with_auth_state"
                    and scenario.get("error_auth_state")
                    != observations.get("auth_state")
                ):
                    raise AssertionError(
                        "Synthetic Find My refresh auth state mismatch"
                    ) from error
                if scenario["service"] == "drive" and scenario.get(
                    "error_drive_state"
                ) != observations.get("drive_state"):
                    raise AssertionError(
                        "Synthetic drive error state mismatch"
                    ) from error
                if scenario["service"] == "drive" and scenario.get(
                    "error_node_state"
                ) != observations.get("node_state"):
                    raise AssertionError(
                        "Synthetic drive node state mismatch"
                    ) from error
                if scenario["service"] == "findmy" and scenario.get(
                    "error_findmy_state"
                ) != observations.get("findmy_state"):
                    raise AssertionError(
                        "Synthetic Find My error state mismatch"
                    ) from error
                if scenario["service"] in {"auth", "drive", "findmy"}:
                    from auth_scenarios import auth_error_context

                    if scenario.get("error_context") != auth_error_context(error):
                        raise AssertionError(
                            "Synthetic service error context mismatch"
                        ) from error
            else:
                if (
                    "error" in scenario
                    or json.loads(json.dumps(result)) != scenario["result"]
                ):
                    raise AssertionError("Synthetic semantic result mismatch")
            if "refresh_auth_state" in scenario and scenario[
                "refresh_auth_state"
            ] != observations.get("refresh_auth_state"):
                raise AssertionError(
                    "Synthetic pre-retry authentication state mismatch"
                )
            if (
                scenario["service"] == "photos"
                and scenario["operation"] == "iter_changes"
            ):
                if "photos_sync_token" not in scenario or scenario[
                    "photos_sync_token"
                ] != observations.get("photos_sync_token"):
                    raise AssertionError("Synthetic Photos cursor state mismatch")
            adapter.assert_consumed()
        finally:
            close_reference(api)
    return len(scenario["exchanges"])


@contextmanager
def observe_findmy_refresh(api, scenario, observations):
    """Observe Source authentication before the bounded Find My retry."""
    if "refresh_auth_state" not in scenario or observations is None:
        yield
        return
    from auth_scenarios import auth_state

    original = api.authenticate

    def authenticate(*args, **kwargs):
        result = original(*args, **kwargs)
        observations["refresh_auth_state"] = auth_state(api)
        return result

    api.authenticate = authenticate
    try:
        yield
    finally:
        api.authenticate = original


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.parse_args()
    paths = sorted(FIXTURES.glob("*.json"))
    count = sum(replay_synthetic(path) for path in paths)
    print(f"Synthetic replay passed: {len(paths)} scenarios, {count} paired exchanges")


if __name__ == "__main__":
    main()

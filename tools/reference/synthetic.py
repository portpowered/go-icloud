"""Run portable, implementation-derived service scenarios at the HTTP seam."""

import argparse
import json
from tempfile import TemporaryDirectory

from icloud import ROOT, SOURCE, close_reference, verify_reference
from network_guard import forbid_network
from recording import ReplayAdapter

FIXTURES = ROOT / "tests" / "replay" / "fixtures" / "synthetic" / "http"


def execute(api, scenario):
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
        return getattr(service, scenario["operation"])(*arguments)
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
        value = getattr(service, operation)(
            *scenario["inputs"], **scenario.get("keyword_inputs", {})
        )
        if operation in {"lists", "iter_changes", "reminders"}:
            return [item.model_dump(mode="json") for item in value]
        if hasattr(value, "model_dump"):
            return value.model_dump(mode="json")
        return value
    raise ValueError("Unregistered synthetic service")


def replay_synthetic(path):
    scenario = json.loads(path.read_text(encoding="utf-8"))
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
        adapter = ReplayAdapter(scenario["exchanges"])
        api.session.mount("https://", adapter)
        api.session.mount("http://", adapter)
        try:
            try:
                result = execute(api, scenario)
            except AssertionError:
                raise
            except Exception as error:
                expected = scenario.get("error")
                if expected != {"type": type(error).__name__, "message": str(error)}:
                    raise AssertionError("Synthetic semantic error mismatch") from error
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

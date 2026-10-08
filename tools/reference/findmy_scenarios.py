"""Exercise selected Find My refresh transitions using real manager methods."""

from copy import deepcopy


def manager_state(manager):
    # A failed refresh stops the monitor; reading device.data would initiate
    # another refresh. Snapshot the received cache without causing new traffic.
    return {
        "devices": [deepcopy(device._content) for device in manager.devices.values()],
        "user_info": deepcopy(dict(manager.user_info)) if manager.user_info else None,
        "server_context": deepcopy(manager._server_ctx),
    }


def execute_refresh(manager, scenario, observations):
    values = [manager_state(manager)]
    try:
        for call in scenario["inputs"]:
            manager.refresh(**call)
            values.append(manager_state(manager))
        return values
    finally:
        observations["findmy_state"] = manager_state(manager)
        observations["arguments"] = scenario["inputs"]

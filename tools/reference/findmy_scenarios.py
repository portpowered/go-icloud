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


def execute_description(manager, scenario):
    """Bind public reference getters to portable copied device descriptions."""
    device = manager[scenario["device_id"]]
    additional = scenario["inputs"][0]["additionalStatus"]
    for field in additional:
        if field in device.data:
            assert device[field] == getattr(device, field) == device.data[field]
    assert len(manager) == len(manager.devices)
    assert [item.data["id"] for item in manager] == list(manager.devices)
    return {
        "device": deepcopy(device.data),
        "name": device.name,
        "model": device.model,
        "modelName": device.model_name,
        "deviceType": device.device_type,
        "location": deepcopy(device.location),
        "capabilities": {
            "sound": device.sound_available,
            "messaging": device.messaging_available,
            "erase": device.erase_available,
            "lostMode": device.lost_mode_available,
            "location": device.location_available,
        },
        "status": deepcopy(device.status(additional)),
    }

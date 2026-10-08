"""Exercise auth endpoints and project explicit portable account session state."""

from pathlib import Path

from recording import response_record
from scenario_inputs import project


def auth_error_context(error):
    response = getattr(error, "response", None)
    return {
        "reason": getattr(error, "reason", None),
        "code": getattr(error, "code", None),
        "response": response_record(response) if response is not None else None,
    }


def auth_state(api):
    result = {
        "account": api.data,
        "params": dict(api.params),
        "session_data": dict(api.session.data),
        "challenge": api._auth_data,
        "requires_mfa": api._requires_mfa,
        "cookies": [
            {
                "name": cookie.name,
                "value": cookie.value,
                "domain": cookie.domain,
                "path": cookie.path,
                "secure": cookie.secure,
                "expires": cookie.expires,
                "discard": cookie.discard,
                "attributes": cookie._rest,
            }
            for cookie in api.session.cookies
        ],
        "webservices": api.webservices,
        "requires_2fa": api.requires_2fa,
        "requires_2sa": api.requires_2sa,
        "trusted_session": api.is_trusted_session,
        "delivery_method": api.two_factor_delivery_method,
        "code_requested": api._two_factor_code_requested,
        "session_file_exists": Path(api.session.session_path).exists(),
        "cookie_file_exists": Path(api.session.cookiejar_path).exists(),
    }
    if api._trusted_device_bridge_state is not None:
        from bridge_replay import bridge_state

        result["bridge"] = bridge_state(api._trusted_device_bridge_state)
    if api.two_factor_delivery_notice is not None:
        result["delivery_notice"] = api.two_factor_delivery_notice
    return result


def execute_auth(api, scenario, observations):
    state = scenario["initial_state"]
    api.data = state.get("account_data", {})
    api.params.update(state["params"])
    api._auth_data = state.get("auth_data", {})
    api._requires_mfa = state.get("requires_mfa", False)
    api._accept_terms = state.get("accept_terms", False)
    if "delivery_method" in state:
        api._set_two_factor_delivery_state(state["delivery_method"])
    api._update_state()
    if state.get("persist_session"):
        api.session._save_session_data()
    arguments = scenario["inputs"]
    try:
        if scenario["operation"] == "flow":
            value = []
            for call in arguments:
                method = getattr(api, call["operation"])
                value.append(method(*call.get("inputs", []), **call.get("kwargs", {})))
        else:
            value = getattr(api, scenario["operation"])
            if callable(value):
                value = value(*arguments, **scenario.get("keyword_inputs", {}))
        return {
            "value": project(value),
            "auth_state": auth_state(api),
            "arguments": arguments,
        }
    finally:
        observations["auth_state"] = auth_state(api)
        observations["arguments"] = arguments

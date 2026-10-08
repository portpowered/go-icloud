"""Run the actual reference monitor with declared virtual wait completions."""

import math
from datetime import datetime
from unittest.mock import patch

from findmy_scenarios import manager_state


def monitor_cookies(manager):
    return [
        {
            "name": cookie.name,
            "value": cookie.value,
            "domain": cookie.domain,
            "path": cookie.path,
            "secure": cookie.secure,
            "hostOnly": not cookie.domain_specified,
        }
        for cookie in manager.session.cookies
    ]


def execute_monitor(manager, scenario):
    from pyicloud.services.findmyiphone import _monitor_thread

    manager.stop_event.set()
    manager._monitor.join(timeout=5)
    assert not manager.is_alive, "reference monitor did not stop"
    events = scenario["entropy"]["findmy_monitor_trace"]
    interval = scenario["inputs"][0]["intervalSeconds"]
    assert events and events[-1]["stopped"] is True
    assert all(event["stopped"] is False for event in events[:-1])
    assert type(interval) in {int, float} and math.isfinite(interval) and interval > 0
    for event in events:
        for key in ["delaySeconds", "completedAtUnix"]:
            value = event[key]
            assert type(value) in {int, float} and math.isfinite(value)
    clock = [scenario["entropy"]["unix_seconds"]]
    consumed = []
    rejected = []
    result = [
        {
            "state": manager_state(manager),
            "failed": False,
            "httpStatus": 0,
            "cookies": monitor_cookies(manager),
        }
    ]

    class ReplayClock(datetime):
        @classmethod
        def now(cls, tz=None):
            return datetime.fromtimestamp(clock[0], tz=tz)

    class ReplayStop:
        def wait(self, timeout):
            try:
                assert not rejected and len(consumed) < len(events)
                event = events[len(consumed)]
                assert timeout == event["delaySeconds"] == interval
                assert type(event["stopped"]) is bool
                assert event["completedAtUnix"] >= clock[0]
                if not event["stopped"]:
                    assert event["completedAtUnix"] > clock[0] + interval
                clock[0] = event["completedAtUnix"]
                consumed.append(event)
                return event["stopped"]
            except (AssertionError, KeyError, TypeError) as error:
                rejected.append(True)
                raise AssertionError("Rejected Find My monitor wait") from error

    def refresh(locate):
        assert locate is False, "monitor requested locations"
        failed, status = False, 0
        try:
            manager._refresh_client(locate=locate)
        except Exception as error:
            failed = True
            response = getattr(error, "response", None)
            status = response.status_code if response is not None else 0
            raise
        finally:
            result.append(
                {
                    "state": manager_state(manager),
                    "failed": failed,
                    "httpStatus": status,
                    "cookies": monitor_cookies(manager),
                }
            )

    with patch("pyicloud.services.findmyiphone.datetime", ReplayClock):
        _monitor_thread(interval, refresh, ReplayStop())
    assert not rejected and len(consumed) == len(events), "unconsumed monitor waits"
    return result

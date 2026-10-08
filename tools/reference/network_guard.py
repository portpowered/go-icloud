"""Block real HTTP and socket connections during reference replay."""

from contextlib import ExitStack, contextmanager
from unittest.mock import patch


@contextmanager
def forbid_network():
    attempts = []

    def denied(*args, **kwargs):
        attempts.append(True)
        raise AssertionError("Network access forbidden during replay")

    with ExitStack() as stack:
        for target in [
            "requests.adapters.HTTPAdapter.send",
            "socket.socket.connect",
            "socket.socket.connect_ex",
            "socket.create_connection",
        ]:
            stack.enter_context(
                patch(
                    target,
                    side_effect=denied,
                )
            )
        try:
            yield
        finally:
            if attempts:
                raise AssertionError("Network access forbidden during replay")

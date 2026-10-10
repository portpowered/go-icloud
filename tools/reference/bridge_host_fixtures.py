"""Execute the pinned host resolver with invented bootstrap inputs, offline.

Only its actual function and environment constant are extracted; this tool
does not import the service, initialize credentials, or open a socket.
"""

import argparse
import ast
import json
import subprocess
from pathlib import Path
from types import SimpleNamespace
from urllib.parse import urlparse


class PyiCloudTrustedDevicePromptException(Exception):
    """Offline replacement for the resolver's provider error type."""


def source_resolver(checkout, revision):
    actual = subprocess.check_output(
        ["git", "-C", str(checkout), "rev-parse", "HEAD"], text=True
    ).strip()
    if actual != revision:
        raise RuntimeError("Host fixture source revision does not match pin")
    path = checkout / "pyicloud" / "hsa2_bridge.py"
    tree = ast.parse(path.read_text(encoding="utf-8"))
    selected = [
        node
        for node in tree.body
        if (isinstance(node, ast.FunctionDef) and node.name == "_resolve_websocket_host")
        or (
            isinstance(node, ast.AnnAssign)
            and isinstance(node.target, ast.Name)
            and node.target.id == "WEBSOCKET_ENVIRONMENT_HOSTS"
        )
    ]
    if len(selected) != 2:
        raise RuntimeError("Pinned resolver inventory changed")
    namespace = {
        "urlparse": urlparse,
        "Hsa2BootContext": SimpleNamespace,
        "PyiCloudTrustedDevicePromptException": PyiCloudTrustedDevicePromptException,
    }
    exec(compile(ast.Module(body=selected, type_ignores=[]), str(path), "exec"), namespace)
    return namespace["_resolve_websocket_host"]


def fixtures(resolve):
    inputs = [
        ("url-case", {"webSocketUrl": "wss://MiXeD.ExAmPlE/a"}),
        ("bare-case", {"webSocketUrl": "MiXeD.ExAmPlE/a"}),
        ("url-userinfo-port", {"webSocketUrl": "wss://user@MiXeD.ExAmPlE:8443/a"}),
        ("zone-raw", {"webSocketUrl": "wss://[FE80::ABCD%En0]/a"}),
        ("zone-escaped", {"webSocketUrl": "wss://[FE80::ABCD%25En0]/a"}),
        ("zone-case", {"webSocketUrl": "wss://[FE80::ABCD%ETHERNET]/a"}),
        ("ipv6", {"webSocketUrl": "wss://[2001:DB8::ABCD]/a"}),
        ("unicode-dot", {"webSocketUrl": "wss://\u0130.Example/a"}),
        ("unicode-final-sigma", {"webSocketUrl": "wss://\u039f\u03a3.Example/a"}),
        ("percent-malformed", {"webSocketUrl": "wss://A%ZZ.Example/a"}),
        ("percent-preserved", {"webSocketUrl": "wss://A%2F.Example/a"}),
        ("ipvfuture", {"webSocketUrl": "wss://[v1.ABC]/a"}),
        ("empty-authority", {"webSocketUrl": "wss:///path"}),
        ("production", {"apnsEnvironment": "prod"}),
        ("sandbox", {"apnsEnvironment": "sandbox"}),
        ("empty-url-environment", {"webSocketUrl": "", "apnsEnvironment": "prod"}),
        ("explicit-over-environment", {"webSocketUrl": "wss://EXAMPLE/a", "apnsEnvironment": "sandbox"}),
        ("missing", {}),
        ("unknown-environment", {"apnsEnvironment": "other"}),
    ]
    rows = []
    for name, data in inputs:
        row = {"name": name, "input": data}
        try:
            row["host"] = resolve(SimpleNamespace(bridge_initiate_data=data))
        except PyiCloudTrustedDevicePromptException:
            row["error"] = True
        rows.append(row)
    return rows


def main():
    root = Path(__file__).resolve().parents[2]
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, default=root / ".reference" / "pyicloud-live")
    args = parser.parse_args()
    pin = json.loads((root / "tools/reference/source.json").read_text())["live"]
    document = {
        "format": "portos.bridge-host.v1",
        "source": {"commit": pin["commit"], "function": "_resolve_websocket_host"},
        "cases": fixtures(source_resolver(args.source, pin["commit"])),
    }
    target = root / "tests/replay/fixtures/synthetic/bridge-host.json"
    target.write_text(json.dumps(document, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()

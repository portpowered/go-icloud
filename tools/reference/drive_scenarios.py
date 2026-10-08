"""Exercise Drive folder navigation and node-backed endpoint calls."""

import base64
from copy import deepcopy


def node_result(node):
    def date(value):
        return value.isoformat() if value is not None else None

    return {
        "data": deepcopy(node.data),
        "name": node.name,
        "type": node.type,
        "size": node.size,
        "date_changed": date(node.date_changed),
        "date_modified": date(node.date_modified),
        "date_last_open": date(node.date_last_open),
    }


def execute_nodes(service, scenario, observations):
    root = None
    node = None
    values = []
    try:
        root = getattr(service, scenario.get("node_selector", "root"))
        node = root
        for call in scenario["inputs"]:
            operation = call["operation"]
            args = call.get("inputs", [])
            kwargs = call.get("kwargs", {})
            if operation == "root":
                node = service.root
                value = node_result(node)
            elif operation == "trash":
                node = service.trash
                value = node_result(node)
            elif operation in {"refresh_root", "refresh_trash"}:
                getattr(service, operation)()
                node = getattr(service, operation.removeprefix("refresh_"))
                value = node_result(node)
            elif operation == "children":
                value = [node_result(child) for child in node.get_children(**kwargs)]
            elif operation in {"get", "getitem", "service_getitem"}:
                if operation == "get":
                    node = node.get(*args)
                elif operation == "getitem":
                    node = node[args[0]]
                else:
                    node = service[args[0]]
                value = node_result(node)
            elif operation == "metadata":
                value = node_result(node)
            elif operation == "open":
                response = node.open(**kwargs)
                try:
                    value = {
                        "status": response.status_code,
                        "headers": list(map(list, response.headers.items())),
                        "body": base64.b64encode(response.content).decode(),
                    }
                finally:
                    response.close()
            else:
                value = getattr(node, operation)(*args, **kwargs)
            values.append(value)
        return {"values": values, "node": node_result(node), "root": node_result(root)}
    finally:
        if observations is not None:
            observations["node_state"] = {
                "node": node_result(node) if node is not None else None,
                "root": node_result(root) if root is not None else None,
            }

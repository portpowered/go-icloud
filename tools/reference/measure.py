"""Measure reference function/branch coverage from offline private scenarios."""

import argparse
import ast
import fnmatch
import json
import logging
import sys
from pathlib import Path

import coverage
from icloud import ROOT, SOURCE, state_root


def scoped_files(root, policy):
    files = set()
    for pattern in policy["include"]:
        files.update(root.glob(pattern))
    return sorted(
        file
        for file in files
        if file.is_file()
        and not any(
            fnmatch.fnmatch(file.relative_to(root).as_posix(), rule["glob"])
            for rule in policy["exclude"]
        )
    )


def create_measurement(data_file, files):
    measurement = coverage.Coverage(
        data_file=str(data_file),
        branch=True,
        config_file=False,
        include=[str(file) for file in files],
        concurrency=["thread"],
    )
    # config_file=False still inherits coverage.py's pragma/type-checking and
    # partial-branch defaults. Scope decisions belong in our audited policy.
    for option in [
        "report:exclude_lines",
        "report:exclude_also",
        "report:partial_branches",
        "report:partial_also",
        "report:partial_branches_always",
    ]:
        measurement.set_option(option, [])
    # An entirely empty partial-pattern list becomes an empty regex in
    # coverage.py 7.16.2 and incorrectly marks every branch as suppressed.
    measurement.set_option("report:partial_branches", [r"(?!)"])
    return measurement


def definitions(tree, prefix=""):
    for node in ast.iter_child_nodes(tree):
        if isinstance(node, ast.ClassDef):
            yield from definitions(node, prefix + node.name + ".")
        elif isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            yield prefix + node.name, node
            yield from definitions(node, prefix + node.name + ".")
        else:
            yield from definitions(node, prefix)


def body_lines(node, statements):
    body = node.body
    if (
        body
        and isinstance(body[0], ast.Expr)
        and isinstance(body[0].value, ast.Constant)
    ):
        if isinstance(body[0].value.value, str):
            body = body[1:]
    if not body:
        return set()
    lines = set(range(body[0].lineno, node.end_lineno + 1)) & statements
    for child in ast.walk(node):
        if child is node:
            continue
        if isinstance(child, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
            lines -= set(range(child.lineno, child.end_lineno + 1))
    return lines


def summarize(measurement, files, root, policy=None):
    entries = []
    exclusions = (policy or {}).get("function_exclusions", [])
    excluded_functions = []
    exclusion_keys = [(rule["file"], rule["function"]) for rule in exclusions]
    if len(set(exclusion_keys)) != len(exclusion_keys):
        raise ValueError("Duplicate function exclusion")
    network_edges = []
    for file in files:
        _, statements, excluded, missing, _ = measurement.analysis2(str(file))
        statements = set(statements) - set(excluded)
        executed = statements - set(missing)
        branches = measurement.branch_stats(str(file))
        tree = ast.parse(file.read_text(encoding="utf-8"))
        for name, node in definitions(tree):
            key = (file.relative_to(root).as_posix(), name)
            if key in exclusion_keys:
                rule = exclusions[exclusion_keys.index(key)]
                if not isinstance(rule["reason"], str) or not rule["reason"].strip():
                    raise ValueError("Function exclusion requires a reason")
                excluded_functions.append({**rule, "definition_line": node.lineno})
                continue
            lines = body_lines(node, statements)
            entry_lines = sorted(lines)
            function_branches = [
                stat for line, stat in branches.items() if line in lines
            ]
            entries.append(
                {
                    "file": file.relative_to(root).as_posix(),
                    "function": name,
                    "definition_line": node.lineno,
                    "body_entered": bool(entry_lines and entry_lines[0] in executed),
                    "statements": len(lines),
                    "covered_statements": len(lines & executed),
                    "missing_lines": sorted(lines - executed),
                    "branch_exits": sum(total for total, _ in function_branches),
                    "covered_branch_exits": sum(
                        taken for _, taken in function_branches
                    ),
                }
            )
        for node in ast.walk(tree):
            if not isinstance(node, ast.Call) or not isinstance(
                node.func, ast.Attribute
            ):
                continue
            receiver = ast.unparse(node.func.value)
            if node.func.attr not in {
                "get",
                "post",
                "put",
                "patch",
                "delete",
                "request",
                "send",
                "connect",
                "send_binary",
                "raw_post",
            }:
                continue
            if not any(
                word in receiver.lower()
                for word in ["session", "_http", "socket", "transport"]
            ):
                continue
            network_edges.append(
                {
                    "file": file.relative_to(root).as_posix(),
                    "line": node.lineno,
                    "call": ast.unparse(node.func),
                    "target_expression": ast.unparse(node.args[0])
                    if node.args
                    else next(
                        (
                            ast.unparse(arg.value)
                            for arg in node.keywords
                            if arg.arg in {"url", "path"}
                        ),
                        "unresolved",
                    ),
                    "executed": node.lineno in executed,
                    "audit_status": (
                        "candidate; AST discovery is not a complete endpoint proof"
                    ),
                }
            )
    if len(excluded_functions) != len(exclusions):
        raise ValueError(
            "Function exclusion does not resolve uniquely in pinned source"
        )
    total = sum(item["statements"] for item in entries)
    hit = sum(item["covered_statements"] for item in entries)
    return {
        "format": "portos.reference-function-coverage.v1",
        "source": SOURCE["live"],
        "evidence": (
            "offline paired replay; captured, synthetic and setup reported separately"
        ),
        "measurement": {
            "tool": "coverage.py",
            "version": coverage.__version__,
            "branch": True,
        },
        "summary": {
            "functions": len(entries),
            "entered_functions": sum(item["body_entered"] for item in entries),
            "function_body_statements": total,
            "covered_function_body_statements": hit,
            "function_body_statement_percent": round(100 * hit / total, 2)
            if total
            else 0,
            "branch_exits": sum(item["branch_exits"] for item in entries),
            "covered_branch_exits": sum(
                item["covered_branch_exits"] for item in entries
            ),
        },
        "functions": entries,
        "excluded_functions": excluded_functions,
        "network_edge_candidates": network_edges,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--min-function-percent", type=float, default=0)
    parser.add_argument("--synthetic-only", action="store_true")
    args = parser.parse_args()
    policy = json.loads((Path(__file__).parent / "coverage-scope.json").read_text())
    root = ROOT / ".reference" / "pyicloud-live" / "pyicloud"
    files = scoped_files(root, policy)
    output = state_root() / "coverage"
    output.mkdir(exist_ok=True)
    logging.disable(logging.CRITICAL)
    measurement = create_measurement(output / ".coverage", files)
    measurement.start()
    try:
        # Import after tracing starts, so constructor/import behavior is visible.
        measurement.switch_context("setup:reference-import")
        from reference_resume import replay_reference_resume
        from reminders_text import replay_reminders_text
        from replay import replay
        from socket_replay import FIXTURES as SOCKET_FIXTURES
        from socket_replay import replay_socket
        from synthetic import FIXTURES, replay_synthetic

        scenarios = []
        captures = state_root() / "captures"
        folders = (
            sorted(captures.iterdir())
            if captures.is_dir() and not args.synthetic_only
            else []
        )
        for folder in folders:
            if not (folder / "initial-cookies").is_dir():
                continue
            if (
                not (folder / "result.json").exists()
                and not (folder / "error.json").exists()
            ):
                continue
            scenario = json.loads((folder / "scenario.json").read_text())
            if scenario["operation"] == "login":
                continue
            measurement.switch_context(
                "captured:" + scenario["operation"] + ":" + folder.name
            )
            count = replay(folder)
            scenarios.append(
                {
                    "name": folder.name,
                    "operation": scenario["operation"],
                    "exchanges": count,
                    "evidence": "captured",
                }
            )
        for path in sorted(FIXTURES.glob("*.json")):
            measurement.switch_context("synthetic:" + path.stem)
            count = replay_synthetic(path)
            scenarios.append(
                {
                    "name": path.stem,
                    "operation": json.loads(path.read_text())["operation"],
                    "exchanges": count,
                    "evidence": "synthetic",
                }
            )
        for path in sorted(SOCKET_FIXTURES.glob("*.json")):
            measurement.switch_context("synthetic:socket:" + path.stem)
            count = replay_socket(path)
            scenarios.append(
                {
                    "name": path.stem,
                    "operation": json.loads(path.read_text())["operation"],
                    "socket_events": count,
                    "evidence": "synthetic",
                }
            )
        measurement.switch_context("synthetic:document:reminders-text")
        corpus = json.loads(
            (
                ROOT / "tests/replay/fixtures/synthetic/binary/reminders-text.json"
            ).read_text(encoding="utf-8")
        )
        count = replay_reminders_text(corpus)
        scenarios.append(
            {
                "name": "reminders-text",
                "operation": "decode-text",
                "documents": count,
                "evidence": "synthetic",
            }
        )
        for name, format_name in [
            ("reference-resume", "portos.reference-resume.v1"),
            ("reference-resume-repeat", "portos.reference-resume-repeat.v1"),
        ]:
            measurement.switch_context("synthetic:local:" + name)
            count = replay_reference_resume(name + ".json", format_name)
            scenarios.append(
                {
                    "name": name,
                    "operation": "restore-authenticate-account-read",
                    "exchanges": count,
                    "evidence": "synthetic",
                }
            )
    finally:
        measurement.stop()
        measurement.save()
    if not scenarios:
        raise ValueError("No completed scenarios available for coverage")
    report = summarize(measurement, files, root, policy)
    report["measurement"]["implicit_exclusions"] = {
        "lines": measurement.get_option("report:exclude_lines"),
        "additional_lines": measurement.get_option("report:exclude_also"),
        "partial_branches": measurement.get_option("report:partial_branches"),
        "additional_partial_branches": measurement.get_option("report:partial_also"),
        "always_partial_branches": measurement.get_option(
            "report:partial_branches_always"
        ),
    }
    report["by_evidence"] = {}
    for evidence in ["captured", "synthetic", "setup"]:
        measurement.get_data().set_query_contexts(["^" + evidence + ":"])
        report["by_evidence"][evidence] = summarize(measurement, files, root, policy)[
            "summary"
        ]
    measurement.get_data().set_query_contexts(None)
    report["scope"] = policy
    report["scenarios"] = scenarios
    (output / "function-coverage.json").write_text(json.dumps(report, indent=2) + "\n")
    measurement.json_report(
        outfile=str(output / "line-branch-coverage.json"), show_contexts=True
    )
    measurement.html_report(directory=str(output / "html"))
    summary = report["summary"]
    print(json.dumps(summary, indent=2))
    print(f"Private coverage reports: {output}")
    percent = 100 * summary["entered_functions"] / summary["functions"]
    return 0 if percent >= args.min_function_percent else 1


if __name__ == "__main__":
    sys.exit(main())

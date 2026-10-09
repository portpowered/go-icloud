"""Controls proving coverage counts executed bodies rather than imports."""

import ast
import socket
import unittest
from pathlib import Path
from tempfile import TemporaryDirectory

from measure import body_lines, create_measurement, definitions, scoped_files, summarize
from network_guard import forbid_network


class MeasurementTests(unittest.TestCase):
    def test_source_pragmas_do_not_hide_statements_or_branches(self):
        with TemporaryDirectory() as directory:
            root = Path(directory)
            file = root / "operations.py"
            source = (
                "def operation(flag):\n"
                "    if flag:  # pragma: no cover\n"
                "        return 1\n"
                "    if flag:  # pragma: no branch\n"
                "        return 2\n"
                "    return 3\n"
                "TYPE_CHECKING = False\n"
                "if TYPE_CHECKING:\n"
                "    hidden_import = 1\n"
            )
            file.write_text(source, encoding="utf-8")
            measurement = create_measurement(root / ".coverage", [file])
            for option in [
                "report:exclude_lines",
                "report:exclude_also",
                "report:partial_also",
                "report:partial_branches_always",
            ]:
                self.assertEqual(measurement.get_option(option), [])
            self.assertEqual(
                measurement.get_option("report:partial_branches"), [r"(?!)"]
            )
            measurement.start()
            try:
                namespace = {}
                exec(compile(source, str(file), "exec"), namespace)
                self.assertEqual(namespace["operation"](False), 3)
            finally:
                measurement.stop()
            _, statements, excluded, missing, _ = measurement.analysis2(str(file))
            self.assertEqual(excluded, [])
            self.assertTrue({3, 5, 9} <= set(statements) & set(missing))
            self.assertEqual(
                measurement.branch_stats(str(file)), {2: (2, 1), 4: (2, 1), 8: (2, 1)}
            )
            report = summarize(measurement, [file], root)
            self.assertEqual(report["summary"]["function_body_statements"], 5)
            self.assertEqual(report["summary"]["covered_function_body_statements"], 3)
            self.assertEqual(report["summary"]["branch_exits"], 4)
            self.assertEqual(report["summary"]["covered_branch_exits"], 2)

    def test_definition_and_docstring_do_not_count_as_a_function_body(self):
        tree = ast.parse('def operation():\n    "doc"\n    return 1\n')
        _, function = next(definitions(tree))
        self.assertEqual(body_lines(function, {1, 2, 3}), {3})
        self.assertFalse(body_lines(function, {1, 2}))

    def test_nested_function_is_counted_separately(self):
        tree = ast.parse(
            "def outer():\n    def inner():\n        return 1\n    return inner()\n"
        )
        functions = list(definitions(tree))
        self.assertEqual([name for name, _ in functions], ["outer", "outer.inner"])
        self.assertEqual(body_lines(functions[0][1], {1, 2, 3, 4}), {4})
        self.assertEqual(body_lines(functions[1][1], {1, 2, 3, 4}), {3})

    def test_unexecuted_files_remain_in_scope_and_generated_exclusion_is_explicit(self):
        with TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "services").mkdir()
            for name in ["read.py", "unavailable.py", "model_pb2.py"]:
                (root / "services" / name).write_text("def operation():\n    pass\n")
            policy = {
                "include": ["services/**/*.py"],
                "exclude": [{"glob": "**/*_pb2.py", "reason": "generated"}],
            }
            self.assertEqual(
                [file.name for file in scoped_files(root, policy)],
                ["read.py", "unavailable.py"],
            )

    def test_real_socket_connections_are_forbidden(self):
        for operation in ["connect", "connect_ex", "create_connection"]:
            with (
                self.subTest(operation=operation),
                self.assertRaisesRegex(AssertionError, "Network access forbidden"),
            ):
                with forbid_network(), socket.socket() as connection:
                    # Even a dependency that catches the immediate exception
                    # cannot make the surrounding replay context pass.
                    with self.assertRaises(AssertionError):
                        if operation == "create_connection":
                            socket.create_connection(("127.0.0.1", 1))
                        else:
                            getattr(connection, operation)(("127.0.0.1", 1))

    def test_named_exclusions_are_auditable_and_must_resolve(self):
        class Measurement:
            def analysis2(self, file):
                return file, [1, 2, 3, 4], [], [4], ""

            def branch_stats(self, file):
                return {}

        with TemporaryDirectory() as directory:
            root = Path(directory)
            file = root / "services.py"
            file.write_text(
                "def requested():\n    return 1\ndef excluded():\n    return 2\n"
            )
            rule = {
                "file": "services.py",
                "function": "excluded",
                "reason": "Owner excluded this service",
            }
            policy = {"function_exclusions": [rule]}
            report = summarize(Measurement(), [file], root, policy)
            self.assertEqual(report["summary"]["functions"], 1)
            self.assertEqual(report["functions"][0]["function"], "requested")
            self.assertEqual(
                report["excluded_functions"], [{**rule, "definition_line": 3}]
            )
            for rules in [
                [{**rule, "function": "unknown"}],
                [rule, rule],
                [{**rule, "reason": ""}],
                [{**rule, "reason": "   "}],
                [{**rule, "reason": True}],
            ]:
                with self.subTest(rules=rules), self.assertRaises(ValueError):
                    summarize(
                        Measurement(), [file], root, {"function_exclusions": rules}
                    )


if __name__ == "__main__":
    unittest.main()

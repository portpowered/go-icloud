"""Controls proving coverage counts executed bodies rather than imports."""

import ast
import socket
import unittest
from pathlib import Path
from tempfile import TemporaryDirectory

from measure import body_lines, definitions, scoped_files
from network_guard import forbid_network


class MeasurementTests(unittest.TestCase):
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
        with forbid_network(), socket.socket() as connection:
            with self.assertRaisesRegex(AssertionError, "Network access forbidden"):
                connection.connect(("127.0.0.1", 1))
            with self.assertRaises(AssertionError):
                connection.connect_ex(("127.0.0.1", 1))
            with self.assertRaises(AssertionError):
                socket.create_connection(("127.0.0.1", 1))


if __name__ == "__main__":
    unittest.main()

import copy
import unittest

from solution import apply_json_patch


class JsonPatchTests(unittest.TestCase):
    def test_add_replace_remove_and_escaping(self):
        doc = {"a/b": {"items": [1, 2, 3]}, "flag": True}
        ops = [
            {"op": "add", "path": "/a~1b/items/1", "value": 9},
            {"op": "replace", "path": "/flag", "value": False},
            {"op": "remove", "path": "/a~1b/items/0"},
        ]
        self.assertEqual(apply_json_patch(doc, ops), {"a/b": {"items": [9, 2, 3]}, "flag": False})
        self.assertEqual(doc, {"a/b": {"items": [1, 2, 3]}, "flag": True})

    def test_move_copy_and_test(self):
        doc = {"src": {"value": 7}, "arr": ["x"]}
        ops = [
            {"op": "copy", "from": "/src/value", "path": "/arr/-"},
            {"op": "move", "from": "/src/value", "path": "/moved"},
            {"op": "test", "path": "/arr/1", "value": 7},
        ]
        self.assertEqual(apply_json_patch(doc, ops), {"src": {}, "arr": ["x", 7], "moved": 7})

    def test_invalid_operations_raise(self):
        bad_ops = [
            [{"op": "remove", "path": "/missing"}],
            [{"op": "add", "path": "/arr/4", "value": 1}],
            [{"op": "replace", "path": "/arr/-", "value": 1}],
            [{"op": "move", "from": "/arr/0", "path": "/arr/0/x"}],
            [{"op": "test", "path": "/arr/0", "value": 99}],
            [{"op": "unknown", "path": "/arr/0"}],
        ]
        for ops in bad_ops:
            with self.subTest(ops=ops):
                with self.assertRaises((ValueError, KeyError, IndexError, AssertionError)):
                    apply_json_patch({"arr": [1, 2]}, copy.deepcopy(ops))


if __name__ == "__main__":
    unittest.main()

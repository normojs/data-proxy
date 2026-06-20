import unittest

from solution import flatten_json


class FlattenJsonTests(unittest.TestCase):
    def test_nested_dicts_lists_and_empty_containers(self):
        value = {
            "user": {"name": "Ada", "tags": ["x", "y"]},
            "empty": {},
            "items": [],
            "ok": True,
        }
        self.assertEqual(
            flatten_json(value),
            {
                "user.name": "Ada",
                "user.tags[0]": "x",
                "user.tags[1]": "y",
                "empty": {},
                "items": [],
                "ok": True,
            },
        )

    def test_root_scalar_and_invalid_key(self):
        self.assertEqual(flatten_json(7), {"": 7})
        with self.assertRaises(ValueError):
            flatten_json({"bad.key": 1})
        with self.assertRaises(ValueError):
            flatten_json({"bad[0]": 1})


if __name__ == "__main__":
    unittest.main()

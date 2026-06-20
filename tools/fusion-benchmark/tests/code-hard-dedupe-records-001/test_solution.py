import unittest

from solution import dedupe_records


class DedupeRecordsTests(unittest.TestCase):
    def test_merge_order_and_none_rules(self):
        records = [
            {"id": "a", "name": "old", "score": None},
            {"id": "b", "name": "bee"},
            {"id": "a", "name": None, "score": 7, "extra": {"x": 1}},
            {"id": "b", "name": "new"},
        ]
        out = dedupe_records(records)
        self.assertEqual(out, [{"id": "a", "name": "old", "score": 7, "extra": {"x": 1}}, {"id": "b", "name": "new"}])
        out[0]["extra"]["x"] = 99
        self.assertEqual(records[2]["extra"]["x"], 1)

    def test_missing_id_raises(self):
        with self.assertRaises(ValueError):
            dedupe_records([{"id": "a"}, {"name": "missing"}])


if __name__ == "__main__":
    unittest.main()

import unittest

from solution import filter_rows


ROWS = [
    {"id": 1, "status": "open", "priority": 3, "owner": "alice", "title": "billing refund", "vip": True},
    {"id": 2, "status": "closed", "priority": 1, "owner": "bob", "title": "login issue", "vip": False},
    {"id": 3, "status": "open", "priority": 5, "owner": "carol", "title": "refund escalation", "vip": False},
    {"id": 4, "status": "pending", "priority": 4, "title": "vip onboarding", "vip": True},
]


class FilterDslTests(unittest.TestCase):
    def test_boolean_precedence_and_parentheses(self):
        expr = 'status == "open" and (priority >= 4 or contains(title, "billing"))'
        self.assertEqual(filter_rows(ROWS, expr), [1, 3])

    def test_exists_not_and_missing_fields(self):
        self.assertEqual(filter_rows(ROWS, 'not exists(owner) or owner == "bob"'), [2, 4])
        self.assertEqual(filter_rows(ROWS, 'owner != "alice" and exists(owner)'), [2, 3])
        self.assertEqual(filter_rows(ROWS, 'missing == 3 or contains(missing, "x")'), [])

    def test_numbers_strings_and_bools(self):
        self.assertEqual(filter_rows(ROWS, 'vip == true and priority > 2'), [1, 4])
        self.assertEqual(filter_rows(ROWS, 'priority < 4 or status != "open"'), [1, 2, 4])
        self.assertEqual(filter_rows(ROWS, 'contains(title, "refund") and vip == false'), [3])

    def test_invalid_expressions_raise_value_error(self):
        bad = [
            "",
            "status ==",
            "priority >< 3",
            "contains(title)",
            "unknown_func(title)",
            "(status == \"open\"",
            "status == open",
        ]
        for expr in bad:
            with self.subTest(expr=expr):
                with self.assertRaises(ValueError):
                    filter_rows(ROWS, expr)


if __name__ == "__main__":
    unittest.main()

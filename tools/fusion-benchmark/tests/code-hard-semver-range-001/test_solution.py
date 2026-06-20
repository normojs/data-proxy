import unittest

from solution import satisfies_range


class SemverRangeTests(unittest.TestCase):
    def test_comparators_and_spaces(self):
        self.assertTrue(satisfies_range("1.2.3", ">=1.0.0 <2.0.0"))
        self.assertFalse(satisfies_range("2.0.0", ">=1.0.0 <2.0.0"))
        self.assertTrue(satisfies_range("1.2.3", "=1.2.3"))
        self.assertFalse(satisfies_range("1.2.4", "=1.2.3"))
        self.assertTrue(satisfies_range("1.2.3", ">1.2.2 <=1.2.3"))

    def test_caret_and_tilde(self):
        self.assertTrue(satisfies_range("1.4.5", "^1.2.3"))
        self.assertFalse(satisfies_range("2.0.0", "^1.2.3"))
        self.assertTrue(satisfies_range("0.2.5", "^0.2.3"))
        self.assertFalse(satisfies_range("0.3.0", "^0.2.3"))
        self.assertTrue(satisfies_range("1.2.9", "~1.2.3"))
        self.assertFalse(satisfies_range("1.3.0", "~1.2.3"))

    def test_or_ranges_and_invalid_inputs(self):
        self.assertTrue(satisfies_range("3.1.0", "^1.0.0 || >=3.0.0 <4.0.0"))
        self.assertFalse(satisfies_range("2.5.0", "^1.0.0 || >=3.0.0 <4.0.0"))
        for version, expr in [("1.2", ">=1.0.0"), ("1.2.3", ""), ("1.2.3", ">>1.0.0")]:
            with self.subTest(version=version, expr=expr):
                with self.assertRaises(ValueError):
                    satisfies_range(version, expr)


if __name__ == "__main__":
    unittest.main()

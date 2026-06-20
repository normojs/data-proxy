import fnmatch
import random
import unittest

from solution import wildcard_match


class WildcardMatchTests(unittest.TestCase):
    def test_core_cases(self):
        self.assertTrue(wildcard_match("abcdef", "a*e?"))
        self.assertTrue(wildcard_match("", "*"))
        self.assertFalse(wildcard_match("", "?"))
        self.assertFalse(wildcard_match("abc", "a*d"))
        self.assertTrue(wildcard_match("abc", "***a?c***"))
        self.assertFalse(wildcard_match("abc", "a"))

    def test_random_against_fnmatchcase(self):
        rng = random.Random(3202)
        alphabet = "abc"
        pattern_alphabet = "abc*?"
        for _ in range(200):
            text = "".join(rng.choice(alphabet) for _ in range(rng.randint(0, 18)))
            pattern = "".join(rng.choice(pattern_alphabet) for _ in range(rng.randint(0, 12)))
            self.assertEqual(wildcard_match(text, pattern), fnmatch.fnmatchcase(text, pattern))

    def test_large_greedy_shape(self):
        text = "a" * 5000 + "b" + "c" * 5000
        self.assertTrue(wildcard_match(text, "a*b*c*"))
        self.assertFalse(wildcard_match(text, "a*b*d*"))


if __name__ == "__main__":
    unittest.main()

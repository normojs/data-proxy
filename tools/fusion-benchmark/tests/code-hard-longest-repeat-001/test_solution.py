import random
import unittest

from solution import longest_repeated_substring_length


def brute(s):
    best = 0
    for length in range(1, len(s) + 1):
        seen = set()
        ok = False
        for i in range(0, len(s) - length + 1):
            sub = s[i : i + length]
            if sub in seen:
                ok = True
                break
            seen.add(sub)
        if ok:
            best = length
    return best


class LongestRepeatedSubstringTests(unittest.TestCase):
    def test_known_cases(self):
        self.assertEqual(longest_repeated_substring_length("banana"), 3)
        self.assertEqual(longest_repeated_substring_length("abcd"), 0)
        self.assertEqual(longest_repeated_substring_length("aaaa"), 3)
        self.assertEqual(longest_repeated_substring_length(""), 0)

    def test_random_against_brute(self):
        rng = random.Random(3203)
        for _ in range(80):
            s = "".join(rng.choice("abcd") for _ in range(rng.randint(0, 35)))
            self.assertEqual(longest_repeated_substring_length(s), brute(s))


if __name__ == "__main__":
    unittest.main()

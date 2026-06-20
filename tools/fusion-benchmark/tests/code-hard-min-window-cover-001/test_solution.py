import random
import unittest
from collections import Counter

from solution import min_window_cover


def brute(text, required):
    need = Counter(required)
    if not need:
        return ""
    best = None
    for i in range(len(text)):
        have = Counter()
        for j in range(i, len(text)):
            have[text[j]] += 1
            if all(have[ch] >= count for ch, count in need.items()):
                cand = text[i : j + 1]
                if best is None or len(cand) < len(best):
                    best = cand
                break
    return best or ""


class MinWindowCoverTests(unittest.TestCase):
    def test_examples_and_tie(self):
        self.assertEqual(min_window_cover("ADOBECODEBANC", "ABC"), "BANC")
        self.assertEqual(min_window_cover("aaflslflsldkalskaaa", "aaa"), "aaa")
        self.assertEqual(min_window_cover("abc", ""), "")
        self.assertEqual(min_window_cover("abc", "z"), "")

    def test_unicode_and_random(self):
        self.assertEqual(min_window_cover("甲乙丙甲丁乙", "乙甲丁"), "甲丁乙")
        rng = random.Random(3201)
        alphabet = "abca"
        for _ in range(80):
            text = "".join(rng.choice(alphabet) for _ in range(rng.randint(0, 18)))
            required = "".join(rng.choice(alphabet) for _ in range(rng.randint(0, 5)))
            self.assertEqual(min_window_cover(text, required), brute(text, required))


if __name__ == "__main__":
    unittest.main()

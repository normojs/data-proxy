import random
import unittest

from solution import count_pattern_occurrences


def brute(text, patterns):
    out = []
    for pattern in patterns:
        count = 0
        start = 0
        while True:
            pos = text.find(pattern, start)
            if pos == -1:
                break
            count += 1
            start = pos + 1
        out.append(count)
    return out


class PatternCountTests(unittest.TestCase):
    def test_overlapping_and_duplicates(self):
        self.assertEqual(count_pattern_occurrences("aaaa", ["a", "aa", "aaa", "b", "aa"]), [4, 3, 2, 0, 3])
        self.assertEqual(count_pattern_occurrences("", ["a", ""]), [0, 0])
        self.assertEqual(count_pattern_occurrences("abcababc", ["abc", "ab", "bc", "cab"]), [2, 3, 2, 1])

    def test_random_against_bruteforce(self):
        rng = random.Random(8181)
        alphabet = "abcd"
        for _ in range(120):
            text = "".join(rng.choice(alphabet) for _ in range(rng.randint(0, 80)))
            patterns = [
                "".join(rng.choice(alphabet) for _ in range(rng.randint(1, 7)))
                for _ in range(rng.randint(0, 30))
            ]
            self.assertEqual(count_pattern_occurrences(text, patterns), brute(text, patterns))

    def test_large_repeated_text(self):
        text = "ababa" * 3000
        patterns = ["aba", "bab", "ababa", "baab"]
        self.assertEqual(count_pattern_occurrences(text, patterns), brute(text, patterns))


if __name__ == "__main__":
    unittest.main()

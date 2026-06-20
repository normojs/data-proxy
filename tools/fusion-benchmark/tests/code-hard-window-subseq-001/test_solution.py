import random
import unittest

from solution import min_window_subsequence


def brute(source, target):
    if target == "":
        return (0, -1)
    best = None
    for i in range(len(source)):
        j = 0
        for end in range(i, len(source)):
            if source[end] == target[j]:
                j += 1
                if j == len(target):
                    cand = (i, end)
                    if best is None or end - i < best[1] - best[0]:
                        best = cand
                    break
    return best if best is not None else (-1, -1)


class MinWindowSubsequenceTests(unittest.TestCase):
    def test_examples(self):
        self.assertEqual(min_window_subsequence("abcdebdde", "bde"), (1, 4))
        self.assertEqual(min_window_subsequence("abc", ""), (0, -1))
        self.assertEqual(min_window_subsequence("abc", "ac"), (0, 2))
        self.assertEqual(min_window_subsequence("abc", "d"), (-1, -1))
        self.assertEqual(min_window_subsequence("bbdeabde", "bde"), (1, 3))

    def test_random_small_against_bruteforce(self):
        rng = random.Random(1209)
        alphabet = "abcd"
        for _ in range(150):
            source = "".join(rng.choice(alphabet) for _ in range(rng.randint(0, 35)))
            target = "".join(rng.choice(alphabet) for _ in range(rng.randint(0, 6)))
            self.assertEqual(min_window_subsequence(source, target), brute(source, target))

    def test_repeated_character_performance(self):
        source = "a" * 5000 + "b" + "a" * 5000 + "c"
        self.assertEqual(min_window_subsequence(source, "abc"), (4999, 10001))


if __name__ == "__main__":
    unittest.main()

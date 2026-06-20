import random
import unittest

from solution import sliding_top_k_sum


class SlidingTopKTests(unittest.TestCase):
    def test_examples_and_edges(self):
        self.assertEqual(sliding_top_k_sum([5, 1, 3, 9, 2], 3, 2), [8, 12, 12])
        self.assertEqual(sliding_top_k_sum([4, 4, 4], 2, 1), [4, 4])
        self.assertEqual(sliding_top_k_sum([1, 2, 3], 2, 0), [0, 0])
        self.assertEqual(sliding_top_k_sum([1, 2], 5, 2), [])

    def test_duplicates_and_negatives(self):
        values = [-5, -1, -1, 7, 7, 0]
        self.assertEqual(sliding_top_k_sum(values, 4, 3), [5, 13, 14])
        self.assertEqual(sliding_top_k_sum(values, 4, 10), [0, 12, 13])

    def test_random_against_naive(self):
        rng = random.Random(41)
        values = [rng.randrange(-30, 31) for _ in range(120)]
        for window in [1, 2, 7, 31]:
            for k in [0, 1, 3, 10, 40]:
                expected = []
                for i in range(0, len(values) - window + 1):
                    expected.append(sum(sorted(values[i : i + window], reverse=True)[:k]))
                self.assertEqual(sliding_top_k_sum(values, window, k), expected)


if __name__ == "__main__":
    unittest.main()

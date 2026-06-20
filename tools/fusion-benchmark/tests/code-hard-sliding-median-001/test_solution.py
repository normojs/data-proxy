import random
import unittest

from solution import sliding_median


def naive(values, window):
    if window <= 0:
        raise ValueError("window must be positive")
    if window > len(values):
        return []
    mid = (window - 1) // 2
    return [sorted(values[i : i + window])[mid] for i in range(len(values) - window + 1)]


class SlidingMedianTests(unittest.TestCase):
    def test_edges_and_duplicates(self):
        self.assertEqual(sliding_median([5, 1, 9, 2, 2], 3), [5, 2, 2])
        self.assertEqual(sliding_median([4, 4, 4], 2), [4, 4])
        self.assertEqual(sliding_median([1, 2], 5), [])
        with self.assertRaises(ValueError):
            sliding_median([1, 2], 0)

    def test_random_against_naive(self):
        rng = random.Random(5150)
        for _ in range(150):
            values = [rng.randint(-25, 25) for _ in range(rng.randint(1, 80))]
            window = rng.randint(1, len(values) + 5)
            self.assertEqual(sliding_median(values, window), naive(values, window))

    def test_large_alternating_values(self):
        values = [i % 17 - 8 for i in range(6000)]
        self.assertEqual(sliding_median(values, 51), naive(values, 51))


if __name__ == "__main__":
    unittest.main()

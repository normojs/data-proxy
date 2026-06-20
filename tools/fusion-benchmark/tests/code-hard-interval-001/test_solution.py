import random
import unittest

from solution import covered_length


def brute(intervals):
    points = set()
    for start, end in intervals:
        if start <= end:
            points.update(range(start, end + 1))
    return len(points)


class CoveredLengthTests(unittest.TestCase):
    def test_empty_and_reversed_ranges(self):
        self.assertEqual(covered_length([]), 0)
        self.assertEqual(covered_length([(5, 3), (10, 9)]), 0)

    def test_overlapping_and_adjacent_ranges(self):
        self.assertEqual(covered_length([(1, 3), (3, 5), (10, 10), (6, 9)]), 10)
        self.assertEqual(covered_length([(-3, -1), (-2, 2), (4, 4)]), 7)

    def test_large_endpoints_without_expanding_points(self):
        self.assertEqual(covered_length([(-10**12, -10**12), (10**12 - 2, 10**12)]), 4)
        self.assertEqual(
            covered_length([(0, 10**12), (5, 9), (10**12 + 1, 10**12 + 3)]),
            10**12 + 4,
        )

    def test_random_small_against_bruteforce(self):
        rng = random.Random(1729)
        for _ in range(200):
            intervals = []
            for _ in range(30):
                a = rng.randint(-30, 30)
                b = rng.randint(-30, 30)
                intervals.append((a, b))
            self.assertEqual(covered_length(intervals), brute(intervals))


if __name__ == "__main__":
    unittest.main()

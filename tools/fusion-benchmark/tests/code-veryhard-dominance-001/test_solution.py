import random
import unittest

from solution import dominance_counts


class DominanceCountsTests(unittest.TestCase):
    def test_basic_boundaries(self):
        points = [(5, 5), (5, 2), (1, 7), (8, 1), (8, 9)]
        queries = [(5, 5), (6, 0), (0, 0), (9, 9), (5, 10)]
        self.assertEqual(dominance_counts(points, queries), [2, 2, 5, 0, 0])

    def test_duplicates_and_negative_coordinates(self):
        points = [(-1, -1), (-1, -1), (0, -2), (3, 4), (3, 4)]
        queries = [(-1, -1), (0, -1), (3, 4), (4, 4), (-5, -5)]
        self.assertEqual(dominance_counts(points, queries), [4, 2, 2, 0, 5])

    def test_random_against_naive(self):
        rng = random.Random(29)
        points = [(rng.randrange(-20, 21), rng.randrange(-20, 21)) for _ in range(300)]
        queries = [(rng.randrange(-25, 26), rng.randrange(-25, 26)) for _ in range(200)]
        expected = [sum(1 for px, py in points if px >= x and py >= y) for x, y in queries]
        self.assertEqual(dominance_counts(points, queries), expected)


if __name__ == "__main__":
    unittest.main()

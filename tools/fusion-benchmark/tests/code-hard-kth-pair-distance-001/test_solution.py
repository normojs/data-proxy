import random
import unittest

from solution import kth_pair_distance


def brute(nums, k):
    distances = []
    for i in range(len(nums)):
        for j in range(i + 1, len(nums)):
            distances.append(abs(nums[i] - nums[j]))
    distances.sort()
    if not 1 <= k <= len(distances):
        raise ValueError("bad k")
    return distances[k - 1]


class KthPairDistanceTests(unittest.TestCase):
    def test_duplicates_and_invalid_k(self):
        self.assertEqual(kth_pair_distance([1, 3, 1], 1), 0)
        self.assertEqual(kth_pair_distance([1, 6, 1], 3), 5)
        with self.assertRaises(ValueError):
            kth_pair_distance([1], 1)
        with self.assertRaises(ValueError):
            kth_pair_distance([1, 2, 3], 0)

    def test_random_against_brute(self):
        rng = random.Random(3103)
        for _ in range(100):
            n = rng.randint(2, 18)
            nums = [rng.randint(-10, 10) for _ in range(n)]
            k = rng.randint(1, n * (n - 1) // 2)
            self.assertEqual(kth_pair_distance(nums, k), brute(nums, k))


if __name__ == "__main__":
    unittest.main()

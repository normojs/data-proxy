import random
import unittest

from solution import count_subarrays_at_most_k_distinct


def brute(nums, k):
    if k <= 0:
        return 0
    total = 0
    for i in range(len(nums)):
        seen = {}
        for j in range(i, len(nums)):
            seen[nums[j]] = seen.get(nums[j], 0) + 1
            if len(seen) <= k:
                total += 1
    return total


class CountSubarraysTests(unittest.TestCase):
    def test_examples_and_edges(self):
        self.assertEqual(count_subarrays_at_most_k_distinct([1, 2, 1, 2, 3], 2), 12)
        self.assertEqual(count_subarrays_at_most_k_distinct([1, 1, 1], 1), 6)
        self.assertEqual(count_subarrays_at_most_k_distinct([1, 2], 0), 0)
        self.assertEqual(count_subarrays_at_most_k_distinct([], 3), 0)

    def test_random_against_brute(self):
        rng = random.Random(3102)
        for _ in range(150):
            nums = [rng.randint(-3, 3) for _ in range(rng.randint(0, 18))]
            k = rng.randint(-1, 6)
            self.assertEqual(count_subarrays_at_most_k_distinct(nums, k), brute(nums, k))


if __name__ == "__main__":
    unittest.main()

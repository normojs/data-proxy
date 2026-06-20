import random
import unittest

from solution import shortest_subarray_at_least


def brute(nums, target):
    best = len(nums) + 1
    for i in range(len(nums)):
        total = 0
        for j in range(i, len(nums)):
            total += nums[j]
            if total >= target:
                best = min(best, j - i + 1)
                break
    return -1 if best == len(nums) + 1 else best


class ShortestSubarrayTests(unittest.TestCase):
    def test_examples_and_edges(self):
        self.assertEqual(shortest_subarray_at_least([2, -1, 2], 3), 3)
        self.assertEqual(shortest_subarray_at_least([1, 2, 3], 5), 2)
        self.assertEqual(shortest_subarray_at_least([-5, -2, -1], 1), -1)
        self.assertEqual(shortest_subarray_at_least([10, -100, 20, 30], 25), 1)
        self.assertEqual(shortest_subarray_at_least([], 1), -1)

    def test_random_against_bruteforce(self):
        rng = random.Random(4444)
        for _ in range(220):
            nums = [rng.randint(-10, 15) for _ in range(rng.randint(0, 45))]
            target = rng.randint(-5, 40)
            self.assertEqual(shortest_subarray_at_least(nums, target), brute(nums, target))

    def test_large_negative_prefix_shape(self):
        nums = [-1] * 5000 + [3] * 5000
        self.assertEqual(shortest_subarray_at_least(nums, 2997), 999)


if __name__ == "__main__":
    unittest.main()

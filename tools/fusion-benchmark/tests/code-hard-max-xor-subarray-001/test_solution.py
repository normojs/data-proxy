import random
import unittest

from solution import max_xor_subarray


def brute(nums):
    best = 0
    for i in range(len(nums)):
        cur = 0
        for j in range(i, len(nums)):
            cur ^= nums[j]
            best = max(best, cur)
    return best


class MaxXorSubarrayTests(unittest.TestCase):
    def test_examples_and_empty(self):
        self.assertEqual(max_xor_subarray([]), 0)
        self.assertEqual(max_xor_subarray([8, 1, 2, 12]), 15)
        self.assertEqual(max_xor_subarray([0, 0, 0]), 0)

    def test_random_against_brute(self):
        rng = random.Random(3104)
        for _ in range(120):
            nums = [rng.randint(0, 255) for _ in range(rng.randint(0, 25))]
            self.assertEqual(max_xor_subarray(nums), brute(nums))


if __name__ == "__main__":
    unittest.main()

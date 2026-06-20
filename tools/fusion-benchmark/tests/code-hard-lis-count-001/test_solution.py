import random
import unittest

from solution import count_lis


def brute(values, mod=1_000_000_007):
    if not values:
        return (0, 0)
    n = len(values)
    length = [1] * n
    count = [1] * n
    for i in range(n):
        for j in range(i):
            if values[j] < values[i]:
                if length[j] + 1 > length[i]:
                    length[i] = length[j] + 1
                    count[i] = count[j]
                elif length[j] + 1 == length[i]:
                    count[i] = (count[i] + count[j]) % mod
    best = max(length)
    return (best, sum(c for l, c in zip(length, count) if l == best) % mod)


class CountLisTests(unittest.TestCase):
    def test_examples_and_duplicates(self):
        self.assertEqual(count_lis([]), (0, 0))
        self.assertEqual(count_lis([1, 3, 5, 4, 7]), (4, 2))
        self.assertEqual(count_lis([2, 2, 2, 2]), (1, 4))
        self.assertEqual(count_lis([3, 1, 2, 2, 4]), (3, 2))

    def test_random_against_bruteforce(self):
        rng = random.Random(6161)
        for _ in range(160):
            values = [rng.randint(-8, 12) for _ in range(rng.randint(0, 35))]
            self.assertEqual(count_lis(values), brute(values))

    def test_large_count_modulo(self):
        values = []
        for block in range(1200):
            values.extend([block, block])
        self.assertEqual(count_lis(values, 1_000_000_007), (1200, pow(2, 1200, 1_000_000_007)))


if __name__ == "__main__":
    unittest.main()

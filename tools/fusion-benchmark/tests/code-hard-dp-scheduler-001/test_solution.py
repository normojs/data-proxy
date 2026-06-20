import random
import unittest

from solution import max_weight_schedule


def brute(intervals, k):
    valid = [(s, e, w) for s, e, w in intervals if s <= e]
    valid.sort(key=lambda x: (x[1], x[0]))
    best = 0

    def dfs(index, chosen, last_end, total):
        nonlocal best
        best = max(best, total)
        if chosen == k or index == len(valid):
            return
        if total + sum(max(0, item[2]) for item in valid[index:]) <= best:
            return
        for i in range(index, len(valid)):
            s, e, w = valid[i]
            if s > last_end:
                dfs(i + 1, chosen + 1, e, total + w)

    dfs(0, 0, -10**30, 0)
    return best


class MaxWeightScheduleTests(unittest.TestCase):
    def test_examples_and_edge_cases(self):
        self.assertEqual(max_weight_schedule([], 3), 0)
        self.assertEqual(max_weight_schedule([(3, 2, 100), (1, 1, -5)], 2), 0)
        self.assertEqual(max_weight_schedule([(1, 3, 4), (4, 6, 7), (3, 5, 100)], 2), 100)
        self.assertEqual(max_weight_schedule([(1, 2, 5), (3, 4, 6), (5, 6, 7)], 2), 13)

    def test_random_small_against_bruteforce(self):
        rng = random.Random(2401)
        for _ in range(120):
            n = rng.randint(0, 11)
            k = rng.randint(0, 5)
            intervals = []
            for _ in range(n):
                a = rng.randint(-8, 8)
                b = rng.randint(-8, 8)
                w = rng.randint(-6, 15)
                intervals.append((a, b, w))
            self.assertEqual(max_weight_schedule(intervals, k), brute(intervals, k))

    def test_large_performance_shape(self):
        intervals = [(i * 3, i * 3 + 1, (i % 17) + 1) for i in range(5000)]
        self.assertEqual(max_weight_schedule(intervals, 50), 50 * 17)


if __name__ == "__main__":
    unittest.main()

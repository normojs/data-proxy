import random
import unittest
from collections import Counter

from solution import top_k_frequent


def reference(items, k):
    if k <= 0:
        return []
    counts = Counter(items)
    return [value for value, _ in sorted(counts.items(), key=lambda kv: (-kv[1], kv[0]))[:k]]


class TopKFrequentTests(unittest.TestCase):
    def test_tie_breaking_and_edges(self):
        self.assertEqual(top_k_frequent([4, 2, 4, 3, 2, 1], 2), [2, 4])
        self.assertEqual(top_k_frequent([5, 5, 6], 10), [5, 6])
        self.assertEqual(top_k_frequent([1, 2], 0), [])

    def test_random_against_reference(self):
        rng = random.Random(3105)
        for _ in range(120):
            items = [rng.randint(-6, 6) for _ in range(rng.randint(0, 50))]
            k = rng.randint(-1, 15)
            self.assertEqual(top_k_frequent(items, k), reference(items, k))


if __name__ == "__main__":
    unittest.main()

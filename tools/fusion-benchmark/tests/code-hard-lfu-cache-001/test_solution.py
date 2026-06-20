import unittest

from solution import LFUCache


class LFUCacheTests(unittest.TestCase):
    def test_evicts_least_frequent_then_lru(self):
        cache = LFUCache(2)
        cache.put("a", 1)
        cache.put("b", 2)
        self.assertEqual(cache.get("a"), 1)
        cache.put("c", 3)
        self.assertEqual(cache.get("b"), -1)
        self.assertEqual(cache.get("c"), 3)
        self.assertEqual(cache.get("a"), 1)

    def test_update_counts_as_use_and_zero_capacity(self):
        cache = LFUCache(2)
        cache.put("a", 1)
        cache.put("b", 2)
        cache.put("a", 10)
        cache.put("c", 3)
        self.assertEqual(cache.get("a"), 10)
        self.assertEqual(cache.get("b"), -1)
        empty = LFUCache(0)
        empty.put("x", 1)
        self.assertEqual(empty.get("x"), -1)

    def test_lru_tie_break_with_same_frequency(self):
        cache = LFUCache(2)
        cache.put("x", 1)
        cache.put("y", 2)
        cache.put("z", 3)
        self.assertEqual(cache.get("x"), -1)
        self.assertEqual(cache.get("y"), 2)
        self.assertEqual(cache.get("z"), 3)


if __name__ == "__main__":
    unittest.main()

import unittest

from solution import LruTtlCache


class LruTtlCacheTests(unittest.TestCase):
    def test_expiry_and_lru_eviction(self):
        cache = LruTtlCache(2)
        cache.put("a", 1, now=0, ttl=5)
        cache.put("b", 2, now=1, ttl=10)
        self.assertEqual(cache.get("a", now=4), 1)
        cache.put("c", 3, now=5, ttl=10)
        self.assertEqual(cache.get("a", now=5), -1)
        self.assertEqual(cache.get("b", now=5), 2)
        self.assertEqual(cache.get("c", now=5), 3)

    def test_get_refreshes_recency_not_expiry(self):
        cache = LruTtlCache(2)
        cache.put("a", 1, now=0, ttl=3)
        cache.put("b", 2, now=0, ttl=10)
        self.assertEqual(cache.get("a", now=2), 1)
        self.assertEqual(cache.get("a", now=3), -1)
        cache.put("c", 3, now=4, ttl=10)
        self.assertEqual(cache.get("b", now=4), 2)
        self.assertEqual(cache.get("c", now=4), 3)

    def test_overwrite_and_zero_capacity(self):
        cache = LruTtlCache(1)
        cache.put("x", 1, now=0, ttl=10)
        cache.put("x", 2, now=5, ttl=10)
        self.assertEqual(cache.get("x", now=14), 2)
        self.assertEqual(cache.get("x", now=15), -1)
        empty = LruTtlCache(0)
        empty.put("x", 1, now=0, ttl=10)
        self.assertEqual(empty.get("x", now=1), -1)


if __name__ == "__main__":
    unittest.main()

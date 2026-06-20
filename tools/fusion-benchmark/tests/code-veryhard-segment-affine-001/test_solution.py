import random
import unittest

from solution import process_queries


class SegmentAffineTests(unittest.TestCase):
    def test_mixed_operations(self):
        values = [1, 2, 3, 4, 5]
        queries = [
            ("sum", 0, 4),
            ("add", 1, 3, 10),
            ("sum", 0, 2),
            ("mul", 2, 4, 3),
            ("sum", 2, 4),
            ("set", 0, 1, 7),
            ("sum", 0, 4),
            ("sum", 1, 1),
        ]
        self.assertEqual(process_queries(values, queries, 1_000_000_007), [15, 26, 96, 110, 7])

    def test_modulo_and_singletons(self):
        values = [10, 20, 30]
        queries = [
            ("mul", 0, 2, 5),
            ("add", 0, 0, 99),
            ("sum", 0, 0),
            ("sum", 0, 2),
            ("set", 2, 2, -7),
            ("sum", 1, 2),
        ]
        self.assertEqual(process_queries(values, queries, 101), [48, 96, 93])

    def test_random_against_naive(self):
        rng = random.Random(17)
        values = [rng.randrange(-50, 50) for _ in range(80)]
        arr = values[:]
        queries = []
        expected = []
        mod = 997
        for _ in range(500):
            op = rng.choice(["add", "mul", "set", "sum"])
            l = rng.randrange(len(arr))
            r = rng.randrange(l, len(arr))
            if op == "sum":
                queries.append((op, l, r))
                expected.append(sum(arr[l : r + 1]) % mod)
            else:
                x = rng.randrange(-20, 20)
                queries.append((op, l, r, x))
                for i in range(l, r + 1):
                    if op == "add":
                        arr[i] = (arr[i] + x) % mod
                    elif op == "mul":
                        arr[i] = (arr[i] * x) % mod
                    else:
                        arr[i] = x % mod
        self.assertEqual(process_queries(values, queries, mod), expected)


if __name__ == "__main__":
    unittest.main()

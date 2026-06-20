import random
import unittest

from solution import process_operations


def reference(operations):
    values = []
    out = []
    for op in operations:
        if op[0] == "add":
            values.append(op[1])
            values.sort()
        elif op[0] == "remove":
            try:
                values.pop(values.index(op[1]))
            except ValueError:
                pass
        elif op[0] == "kth":
            k = op[1]
            out.append(values[k - 1] if 1 <= k <= len(values) else None)
        elif op[0] == "count_le":
            out.append(sum(1 for x in values if x <= op[1]))
    return out


class ProcessOperationsTests(unittest.TestCase):
    def test_order_statistics_and_duplicates(self):
        ops = [
            ("add", 5), ("add", 1), ("add", 5), ("count_le", 5),
            ("kth", 2), ("remove", 5), ("kth", 2), ("remove", 42),
            ("count_le", 4), ("kth", 0), ("kth", 5),
        ]
        self.assertEqual(process_operations(ops), [3, 5, 5, 1, None, None])

    def test_random_against_reference(self):
        rng = random.Random(9001)
        for _ in range(100):
            ops = []
            for _ in range(250):
                kind = rng.choice(["add", "remove", "kth", "count_le"])
                if kind == "kth":
                    ops.append((kind, rng.randint(0, 40)))
                else:
                    ops.append((kind, rng.randint(-20, 20)))
            self.assertEqual(process_operations(ops), reference(ops))

    def test_large_coordinate_range(self):
        ops = []
        for i in range(1, 8000):
            ops.append(("add", i * 10**9))
        ops.extend([("count_le", 3_000_000_000_000), ("kth", 7999), ("remove", 10**9), ("kth", 1)])
        self.assertEqual(process_operations(ops), [3000, 7_999_000_000_000, 2_000_000_000])


if __name__ == "__main__":
    unittest.main()

import random
import unittest

from solution import normalize_intervals


def reference(intervals):
    valid = sorted((a, b) for a, b in intervals if a <= b)
    out = []
    for a, b in valid:
        if not out or a > out[-1][1] + 1:
            out.append([a, b])
        else:
            out[-1][1] = max(out[-1][1], b)
    return [tuple(x) for x in out]


class NormalizeIntervalsTests(unittest.TestCase):
    def test_merges_overlapping_and_touching(self):
        intervals = [(5, 7), (1, 3), (4, 4), (10, 8), (-2, -1), (-1, 0)]
        self.assertEqual(normalize_intervals(intervals), [(-2, 0), (1, 7)])

    def test_empty_and_invalid(self):
        self.assertEqual(normalize_intervals([]), [])
        self.assertEqual(normalize_intervals([(3, 1), (9, 8)]), [])

    def test_random_against_reference(self):
        rng = random.Random(3101)
        for _ in range(100):
            intervals = []
            for _ in range(rng.randint(0, 60)):
                a = rng.randint(-30, 30)
                b = rng.randint(-30, 30)
                intervals.append((a, b))
            self.assertEqual(normalize_intervals(intervals), reference(intervals))


if __name__ == "__main__":
    unittest.main()

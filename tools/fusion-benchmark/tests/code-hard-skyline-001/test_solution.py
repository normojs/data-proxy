import random
import unittest

from solution import skyline_keypoints


def brute(buildings):
    valid = [(l, r, h) for l, r, h in buildings if l < r and h > 0]
    if not valid:
        return []
    xs = sorted({x for l, r, _ in valid for x in (l, r)})
    out = []
    for x in xs:
        height = max((h for l, r, h in valid if l <= x < r), default=0)
        if not out or out[-1][1] != height:
            out.append((x, height))
    if out[-1][1] != 0:
        out.append((max(r for _, r, _ in valid), 0))
    return out


class SkylineTests(unittest.TestCase):
    def test_known_case_and_ignored_buildings(self):
        buildings = [(2, 9, 10), (3, 7, 15), (5, 12, 12), (15, 20, 10), (19, 24, 8), (1, 1, 99)]
        self.assertEqual(skyline_keypoints(buildings), [(2, 10), (3, 15), (7, 12), (12, 0), (15, 10), (20, 8), (24, 0)])
        self.assertEqual(skyline_keypoints([(1, 1, 5), (2, 3, 0)]), [])

    def test_random_against_brute(self):
        rng = random.Random(3221)
        for _ in range(80):
            buildings = []
            for _ in range(rng.randint(0, 12)):
                l = rng.randint(0, 20)
                r = rng.randint(0, 20)
                h = rng.randint(0, 10)
                buildings.append((l, r, h))
            self.assertEqual(skyline_keypoints(buildings), brute(buildings))


if __name__ == "__main__":
    unittest.main()

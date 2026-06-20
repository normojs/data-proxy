import random
import unittest

from solution import components_after_removals


def brute(n, edges, removals):
    active = set(range(len(edges)))
    out = []
    for idx in removals:
        if idx < 0 or idx >= len(edges):
            raise ValueError("bad edge index")
        active.remove(idx)
        parent = list(range(n))

        def find(x):
            while parent[x] != x:
                parent[x] = parent[parent[x]]
                x = parent[x]
            return x

        for i in active:
            a, b = edges[i]
            ra, rb = find(a), find(b)
            if ra != rb:
                parent[ra] = rb
        out.append(len({find(i) for i in range(n)}))
    return out


class ComponentsAfterRemovalsTests(unittest.TestCase):
    def test_chain_and_invalid_index(self):
        edges = [(0, 1), (1, 2), (2, 3)]
        self.assertEqual(components_after_removals(4, edges, [1, 0, 2]), [2, 3, 4])
        with self.assertRaises(ValueError):
            components_after_removals(2, [(0, 1)], [1])

    def test_random_against_brute(self):
        rng = random.Random(3107)
        for _ in range(60):
            n = rng.randint(1, 12)
            edges = []
            for _ in range(rng.randint(0, 20)):
                a, b = rng.randrange(n), rng.randrange(n)
                edges.append((a, b))
            order = list(range(len(edges)))
            rng.shuffle(order)
            removals = order[: rng.randint(0, len(order))]
            self.assertEqual(components_after_removals(n, edges, removals), brute(n, edges, removals))


if __name__ == "__main__":
    unittest.main()

import random
import unittest
from collections import deque

from solution import min_reversals


def reference(n, edges, source, target):
    graph = [[] for _ in range(n)]
    for u, v in edges:
        graph[u].append((v, 0))
        graph[v].append((u, 1))
    dist = [10**9] * n
    dist[source] = 0
    dq = deque([source])
    while dq:
        node = dq.popleft()
        for nxt, cost in graph[node]:
            nd = dist[node] + cost
            if nd < dist[nxt]:
                dist[nxt] = nd
                if cost:
                    dq.append(nxt)
                else:
                    dq.appendleft(nxt)
    return -1 if dist[target] == 10**9 else dist[target]


class MinReversalsTests(unittest.TestCase):
    def test_basic_paths(self):
        self.assertEqual(min_reversals(4, [(0, 1), (2, 1), (2, 3)], 0, 3), 1)
        self.assertEqual(min_reversals(4, [(0, 1), (1, 2), (2, 3)], 0, 3), 0)
        self.assertEqual(min_reversals(4, [(1, 0), (2, 1), (3, 2)], 0, 3), 3)
        self.assertEqual(min_reversals(5, [(0, 1), (3, 4)], 0, 4), -1)

    def test_random_graphs(self):
        rng = random.Random(7331)
        for _ in range(150):
            n = rng.randint(2, 18)
            m = rng.randint(0, 45)
            edges = []
            for _ in range(m):
                u = rng.randrange(n)
                v = rng.randrange(n)
                if u != v:
                    edges.append((u, v))
            s = rng.randrange(n)
            t = rng.randrange(n)
            self.assertEqual(min_reversals(n, edges, s, t), reference(n, edges, s, t))

    def test_large_zero_cost_chain(self):
        n = 20000
        edges = [(i, i + 1) for i in range(n - 1)]
        edges.extend((i + 1, i) for i in range(0, n - 1, 5))
        self.assertEqual(min_reversals(n, edges, 0, n - 1), 0)


if __name__ == "__main__":
    unittest.main()

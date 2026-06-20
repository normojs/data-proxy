import random
import unittest
from collections import deque

from solution import min_project_time


def reference(n, durations, dependencies):
    graph = [[] for _ in range(n)]
    indeg = [0] * n
    for a, b in dependencies:
        graph[a].append(b)
        indeg[b] += 1
    ready = deque(i for i, degree in enumerate(indeg) if degree == 0)
    finish = durations[:]
    seen = 0
    while ready:
        node = ready.popleft()
        seen += 1
        for nxt in graph[node]:
            finish[nxt] = max(finish[nxt], finish[node] + durations[nxt])
            indeg[nxt] -= 1
            if indeg[nxt] == 0:
                ready.append(nxt)
    return max(finish, default=0) if seen == n else -1


class MinProjectTimeTests(unittest.TestCase):
    def test_parallel_chain_and_cycle(self):
        self.assertEqual(min_project_time(0, [], []), 0)
        self.assertEqual(min_project_time(4, [3, 2, 5, 7], [(0, 2), (1, 2), (2, 3)]), 15)
        self.assertEqual(min_project_time(3, [4, 5, 6], [(0, 1), (1, 2), (2, 0)]), -1)
        self.assertEqual(min_project_time(3, [10, 1, 1], []), 10)

    def test_random_dags_against_reference(self):
        rng = random.Random(3301)
        for _ in range(160):
            n = rng.randint(1, 25)
            durations = [rng.randint(1, 20) for _ in range(n)]
            order = list(range(n))
            rng.shuffle(order)
            pos = {node: i for i, node in enumerate(order)}
            deps = []
            for a in range(n):
                for b in range(n):
                    if pos[a] < pos[b] and rng.random() < 0.08:
                        deps.append((a, b))
            self.assertEqual(min_project_time(n, durations, deps), reference(n, durations, deps))

    def test_large_layered_graph(self):
        n = 10000
        durations = [(i % 7) + 1 for i in range(n)]
        deps = [(i, i + 100) for i in range(n - 100)]
        self.assertEqual(min_project_time(n, durations, deps), reference(n, durations, deps))


if __name__ == "__main__":
    unittest.main()

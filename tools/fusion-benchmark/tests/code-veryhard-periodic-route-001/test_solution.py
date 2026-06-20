import heapq
import random
import unittest

from solution import earliest_arrival


def reference(n, edges, source, target, start_time=0):
    graph = [[] for _ in range(n)]
    for edge in edges:
        graph[edge[0]].append(edge)
    dist = [10**30] * n
    dist[source] = start_time
    heap = [(start_time, source)]
    while heap:
        time, node = heapq.heappop(heap)
        if time != dist[node]:
            continue
        if node == target:
            return time
        for _, v, first, period, duration in graph[node]:
            if period == 0:
                if time > first:
                    continue
                depart = first
            elif time <= first:
                depart = first
            else:
                depart = first + ((time - first + period - 1) // period) * period
            arrival = depart + duration
            if arrival < dist[v]:
                dist[v] = arrival
                heapq.heappush(heap, (arrival, v))
    return -1


class EarliestArrivalTests(unittest.TestCase):
    def test_waiting_and_one_shot_edges(self):
        edges = [(0, 1, 5, 10, 2), (1, 2, 9, 3, 4), (0, 2, 30, 0, 1)]
        self.assertEqual(earliest_arrival(3, edges, 0, 2, 0), 13)
        self.assertEqual(earliest_arrival(3, [(0, 2, 30, 0, 1)], 0, 2, 31), -1)

    def test_random_small_against_reference(self):
        rng = random.Random(88)
        for _ in range(120):
            n = rng.randint(2, 12)
            edges = []
            for _ in range(rng.randint(0, 35)):
                u = rng.randrange(n)
                v = rng.randrange(n)
                if u == v:
                    continue
                first = rng.randint(0, 20)
                period = rng.choice([0, 1, 2, 3, 5, 8])
                duration = rng.randint(0, 10)
                edges.append((u, v, first, period, duration))
            source = rng.randrange(n)
            target = rng.randrange(n)
            start = rng.randint(0, 15)
            self.assertEqual(earliest_arrival(n, edges, source, target, start), reference(n, edges, source, target, start))

    def test_large_chain(self):
        n = 5000
        edges = [(i, i + 1, i % 7, 7, 1) for i in range(n - 1)]
        self.assertEqual(earliest_arrival(n, edges, 0, n - 1, 0), reference(n, edges, 0, n - 1, 0))


if __name__ == "__main__":
    unittest.main()

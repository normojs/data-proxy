import random
import unittest
from collections import deque

from solution import batch_layers


def reference(tasks, dependencies):
    names = sorted(set(tasks))
    graph = {name: [] for name in names}
    indeg = {name: 0 for name in names}
    for before, after in dependencies:
        graph[before].append(after)
        indeg[after] += 1
    ready = sorted(name for name in names if indeg[name] == 0)
    layers = []
    seen = 0
    while ready:
        layer = ready
        layers.append(layer)
        seen += len(layer)
        nxt = []
        for name in layer:
            for after in graph[name]:
                indeg[after] -= 1
                if indeg[after] == 0:
                    nxt.append(after)
        ready = sorted(nxt)
    if seen != len(names):
        raise ValueError("cycle")
    return layers


class BatchLayersTests(unittest.TestCase):
    def test_basic_layers_and_sorting(self):
        tasks = ["build", "lint", "test", "deploy", "package"]
        deps = [("build", "test"), ("lint", "test"), ("test", "package"), ("package", "deploy")]
        self.assertEqual(batch_layers(tasks, deps), [["build", "lint"], ["test"], ["package"], ["deploy"]])

    def test_cycle_and_unknown_dependency(self):
        with self.assertRaises(ValueError):
            batch_layers(["a", "b"], [("a", "b"), ("b", "a")])
        with self.assertRaises(ValueError):
            batch_layers(["a"], [("a", "missing")])

    def test_random_dags_against_reference(self):
        rng = random.Random(222)
        for _ in range(100):
            n = rng.randint(1, 35)
            tasks = [f"t{i}" for i in range(n)]
            order = tasks[:]
            rng.shuffle(order)
            pos = {name: i for i, name in enumerate(order)}
            deps = []
            for a in tasks:
                for b in tasks:
                    if pos[a] < pos[b] and rng.random() < 0.06:
                        deps.append((a, b))
            self.assertEqual(batch_layers(tasks, deps), reference(tasks, deps))

    def test_large_chain_shape(self):
        tasks = [f"task-{i}" for i in range(3000)]
        deps = [(tasks[i], tasks[i + 1]) for i in range(len(tasks) - 1)]
        layers = batch_layers(tasks, deps)
        self.assertEqual(len(layers), len(tasks))
        self.assertEqual(layers[0], ["task-0"])
        self.assertEqual(layers[-1], [f"task-{len(tasks)-1}"])


if __name__ == "__main__":
    unittest.main()

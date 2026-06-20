import unittest

from solution import shortest_bridge


class ShortestBridgeTests(unittest.TestCase):
    def test_small_examples(self):
        self.assertEqual(shortest_bridge([[0, 1], [1, 0]]), 1)
        self.assertEqual(shortest_bridge([[0, 1, 0], [0, 0, 0], [0, 0, 1]]), 2)
        self.assertEqual(shortest_bridge([[1, 1, 0, 0, 0], [1, 0, 0, 0, 1], [0, 0, 0, 1, 1]]), 2)

    def test_invalid_empty_grid(self):
        with self.assertRaises(ValueError):
            shortest_bridge([])
        with self.assertRaises(ValueError):
            shortest_bridge([[]])


if __name__ == "__main__":
    unittest.main()

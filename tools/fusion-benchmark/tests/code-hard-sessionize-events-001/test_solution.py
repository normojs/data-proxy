import unittest

from solution import sessionize_events


class SessionizeEventsTests(unittest.TestCase):
    def test_unsorted_events_and_user_key_order(self):
        events = [(20, "b"), (1, "a"), (4, "a"), (10, "a"), (21, "b"), (30, "b")]
        self.assertEqual(
            sessionize_events(events, 3),
            {
                "a": [(1, 4, 2), (10, 10, 1)],
                "b": [(20, 21, 2), (30, 30, 1)],
            },
        )

    def test_zero_gap_and_invalid_gap(self):
        self.assertEqual(sessionize_events([(1, "u"), (1, "u"), (2, "u")], 0), {"u": [(1, 1, 2), (2, 2, 1)]})
        with self.assertRaises(ValueError):
            sessionize_events([], -1)


if __name__ == "__main__":
    unittest.main()

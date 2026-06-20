import random
import unittest
from collections import defaultdict, deque

from solution import rolling_rate_limit


def reference(events, window_seconds, limit):
    if window_seconds <= 0 or limit < 0:
        raise ValueError("bad config")
    queues = defaultdict(deque)
    out = []
    for ts, user in events:
        q = queues[user]
        while q and q[0] <= ts - window_seconds:
            q.popleft()
        allowed = len(q) < limit
        out.append(allowed)
        if allowed:
            q.append(ts)
    return out


class RollingRateLimitTests(unittest.TestCase):
    def test_basic_window_and_rejected_not_counted(self):
        events = [(0, "a"), (1, "a"), (2, "a"), (5, "a"), (6, "a"), (6, "b")]
        self.assertEqual(rolling_rate_limit(events, 5, 2), [True, True, False, True, True, True])
        self.assertEqual(rolling_rate_limit(events, 5, 0), [False, False, False, False, False, False])
        with self.assertRaises(ValueError):
            rolling_rate_limit(events, 0, 2)

    def test_random_against_reference(self):
        rng = random.Random(3217)
        for _ in range(80):
            t = 0
            events = []
            for _ in range(rng.randint(0, 40)):
                t += rng.randint(0, 3)
                events.append((t, rng.choice(["a", "b", "c"])))
            window = rng.randint(1, 8)
            limit = rng.randint(0, 5)
            self.assertEqual(rolling_rate_limit(events, window, limit), reference(events, window, limit))


if __name__ == "__main__":
    unittest.main()

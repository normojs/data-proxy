import ipaddress
import random
import unittest

from solution import summarize_ipv4_addresses


def reference(addresses):
    ints = sorted({int(ipaddress.IPv4Address(addr)) for addr in addresses})
    ranges = []
    i = 0
    while i < len(ints):
        start = ints[i]
        end = start
        i += 1
        while i < len(ints) and ints[i] == end + 1:
            end = ints[i]
            i += 1
        ranges.append((ipaddress.IPv4Address(start), ipaddress.IPv4Address(end)))
    return [str(net) for net in ipaddress.summarize_address_range(*rng) for rng in []] if False else [
        str(net)
        for start, end in ranges
        for net in ipaddress.summarize_address_range(start, end)
    ]


class CidrSummarizeTests(unittest.TestCase):
    def test_examples_duplicates_and_ordering(self):
        self.assertEqual(summarize_ipv4_addresses([]), [])
        self.assertEqual(summarize_ipv4_addresses(["192.168.0.1", "192.168.0.1"]), ["192.168.0.1/32"])
        self.assertEqual(
            summarize_ipv4_addresses(["10.0.0.0", "10.0.0.1", "10.0.0.2", "10.0.0.3"]),
            ["10.0.0.0/30"],
        )
        self.assertEqual(
            summarize_ipv4_addresses(["10.0.0.1", "10.0.0.2", "10.0.0.4"]),
            ["10.0.0.1/32", "10.0.0.2/32", "10.0.0.4/32"],
        )

    def test_random_against_ipaddress(self):
        rng = random.Random(7070)
        for _ in range(120):
            base = rng.randint(0, 2**24)
            addresses = [str(ipaddress.IPv4Address(base + rng.randint(0, 255))) for _ in range(80)]
            self.assertEqual(summarize_ipv4_addresses(addresses), reference(addresses))

    def test_large_contiguous_block(self):
        addresses = [f"172.16.{i // 256}.{i % 256}" for i in range(4096)]
        self.assertEqual(summarize_ipv4_addresses(addresses), ["172.16.0.0/20"])


if __name__ == "__main__":
    unittest.main()

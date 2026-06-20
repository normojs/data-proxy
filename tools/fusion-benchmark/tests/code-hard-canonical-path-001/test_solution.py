import unittest

from solution import canonicalize_path


class CanonicalizePathTests(unittest.TestCase):
    def test_absolute_paths(self):
        self.assertEqual(canonicalize_path("/a//b/./c/../"), "/a/b")
        self.assertEqual(canonicalize_path("/../"), "/")
        self.assertEqual(canonicalize_path("////"), "/")

    def test_relative_paths_preserve_unresolved_parents(self):
        self.assertEqual(canonicalize_path("a/b/../../c"), "c")
        self.assertEqual(canonicalize_path("../../a/./b"), "../../a/b")
        self.assertEqual(canonicalize_path("a/../../b"), "../b")
        self.assertEqual(canonicalize_path(""), ".")
        self.assertEqual(canonicalize_path("."), ".")


if __name__ == "__main__":
    unittest.main()

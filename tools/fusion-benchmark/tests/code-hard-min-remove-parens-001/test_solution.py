import unittest

from solution import min_remove_parentheses


class MinRemoveParenthesesTests(unittest.TestCase):
    def test_basic_valid_outputs(self):
        self.assertEqual(min_remove_parentheses("a)b(c)d"), "ab(c)d")
        self.assertEqual(min_remove_parentheses("))(("), "")
        self.assertEqual(min_remove_parentheses("abc"), "abc")
        self.assertEqual(min_remove_parentheses("lee(t(c)o)de)"), "lee(t(c)o)de")

    def test_lexicographically_smallest_minimal_result(self):
        self.assertEqual(min_remove_parentheses("(a)())()"), "(a())()")
        self.assertEqual(min_remove_parentheses("())()("), "()()")

    def test_large_balanced_shape(self):
        s = "(" * 1000 + "x" + ")" * 1000 + ")" * 100
        self.assertEqual(min_remove_parentheses(s), "(" * 1000 + "x" + ")" * 1000)


if __name__ == "__main__":
    unittest.main()

import unittest

from solution import eval_rpn


class EvalRpnTests(unittest.TestCase):
    def test_arithmetic_and_truncating_division(self):
        self.assertEqual(eval_rpn(["2", "1", "+", "3", "*"]), 9)
        self.assertEqual(eval_rpn(["7", "-3", "/"]), -2)
        self.assertEqual(eval_rpn(["-7", "3", "/"]), -2)
        self.assertEqual(eval_rpn(["-7", "3", "%"]), -1)
        self.assertEqual(eval_rpn(["7", "-3", "%"]), 1)

    def test_invalid_inputs(self):
        for tokens in [["+"], ["1", "2"], ["1", "0", "/"], ["1", "0", "%"], ["x"]]:
            with self.subTest(tokens=tokens):
                with self.assertRaises(ValueError):
                    eval_rpn(tokens)


if __name__ == "__main__":
    unittest.main()

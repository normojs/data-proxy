import unittest

from solution import eval_formula


class EvalFormulaTests(unittest.TestCase):
    def test_precedence_unary_and_variables(self):
        env = {"a": 7, "b": -3, "long_name": 10}
        self.assertEqual(eval_formula("a + b * 4", env), -5)
        self.assertEqual(eval_formula("-(a + b) * +2", env), -8)
        self.assertEqual(eval_formula("long_name - a * (b + 5)", env), -4)
        self.assertEqual(eval_formula("--a + +-b", env), 10)

    def test_truncating_division_and_modulo(self):
        self.assertEqual(eval_formula("7 / 3", {}), 2)
        self.assertEqual(eval_formula("-7 / 3", {}), -2)
        self.assertEqual(eval_formula("7 / -3", {}), -2)
        self.assertEqual(eval_formula("-7 % 3", {}), -1)
        self.assertEqual(eval_formula("7 % -3", {}), 1)
        self.assertEqual(eval_formula("18 / 4 * 3 + 18 % 4", {}), 14)

    def test_whitespace_and_large_expression(self):
        expr = " + ".join(f"({i} * x - {i % 5})" for i in range(80))
        expected = sum(i * 3 - (i % 5) for i in range(80))
        self.assertEqual(eval_formula(expr, {"x": 3}), expected)

    def test_invalid_inputs_raise_value_error(self):
        bad = ["", "a +", "()", "1 ** 2", "unknown + 1", "5 / 0", "5 % 0", "1 2", "(1 + 2"]
        for expr in bad:
            with self.subTest(expr=expr):
                with self.assertRaises(ValueError):
                    eval_formula(expr, {"a": 1})


if __name__ == "__main__":
    unittest.main()

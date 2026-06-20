import random
import unittest

from solution import simplify_polynomial


def eval_poly(poly, x):
    total = 0
    for term in poly.split("+"):
        if not term:
            continue
        sign = -1 if term.startswith("-") else 1
        body = term[1:] if term.startswith("-") else term
        if "x" not in body:
            total += int(term)
        elif body == "x":
            total += sign * x
        elif body.endswith("*x"):
            total += sign * int(body[:-2]) * x
        elif body.startswith("x^"):
            total += sign * (x ** int(body[2:]))
        else:
            coef, power = body.split("*x^")
            total += sign * int(coef) * (x ** int(power))
    return total


class SimplifyPolynomialTests(unittest.TestCase):
    def test_basic_simplification(self):
        self.assertEqual(simplify_polynomial("x + x + 2 - 3"), "2*x-1")
        self.assertEqual(simplify_polynomial("(x + 1) * (x - 1)"), "x^2-1")
        self.assertEqual(simplify_polynomial("2*x*(x+3) - x^2"), "x^2+6*x")
        self.assertEqual(simplify_polynomial("0*x + 0"), "0")

    def test_equivalence_on_generated_expressions(self):
        exprs = [
            "(x+2)*(x+3)*(x-5)",
            "x^4 - 2*x^3 + x - (x^2 - 1)*(x+1)",
            "((x-1)*(x-1)) + 4*x - 4",
            "3*(x^2 + 2*x + 1) - (x+1)*(x+1)",
        ]
        for expr in exprs:
            poly = simplify_polynomial(expr)
            for x in range(-5, 6):
                self.assertEqual(eval_poly(poly.replace("-", "+-"), x), eval(expr.replace("^", "**"), {"x": x}))

    def test_random_linear_products(self):
        rng = random.Random(9191)
        for _ in range(40):
            a = rng.randint(-5, 5)
            b = rng.randint(-5, 5)
            c = rng.randint(-5, 5)
            expr = f"(x+{a})*(x+{b})+{c}*x"
            poly = simplify_polynomial(expr)
            for x in range(-4, 5):
                self.assertEqual(eval_poly(poly.replace("-", "+-"), x), eval(expr.replace("^", "**"), {"x": x}))


if __name__ == "__main__":
    unittest.main()

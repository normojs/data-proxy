import unittest

from solution import render_template


class RenderTemplateTests(unittest.TestCase):
    def test_replaces_variables_and_escapes_braces(self):
        tpl = "Hello {{ name }}, count={{count}}; literal={{{{x}}}}"
        self.assertEqual(render_template(tpl, {"name": "Ada", "count": 3}), "Hello Ada, count=3; literal={{x}}")

    def test_missing_and_malformed_placeholders(self):
        with self.assertRaises(ValueError):
            render_template("Hello {{ missing }}", {})
        with self.assertRaises(ValueError):
            render_template("Hello {{ 123 }}", {"x": 1})
        with self.assertRaises(ValueError):
            render_template("Hello {{ name ", {"name": "Ada"})

    def test_underscore_names_and_non_string_values(self):
        self.assertEqual(render_template("{{ _x1 }}={{ value }}", {"_x1": "k", "value": False}), "k=False")


if __name__ == "__main__":
    unittest.main()

import unittest

from solution import parse_json_lines


class ParseJsonLinesTests(unittest.TestCase):
    def test_values_and_blank_lines(self):
        text = '\n{"a":1}\n  \n[1,2]\ntrue\n"hi"\n'
        self.assertEqual(parse_json_lines(text), [{"a": 1}, [1, 2], True, "hi"])

    def test_malformed_line_reports_line_number(self):
        with self.assertRaises(ValueError) as ctx:
            parse_json_lines('{"ok": true}\n{bad}\n42')
        self.assertIn("2", str(ctx.exception))

    def test_trailing_garbage_is_rejected(self):
        with self.assertRaises(ValueError):
            parse_json_lines('{"a":1} extra')


if __name__ == "__main__":
    unittest.main()

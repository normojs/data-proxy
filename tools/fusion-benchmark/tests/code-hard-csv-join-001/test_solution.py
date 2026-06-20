import csv
import io
import unittest

from solution import csv_join


class CsvJoinTests(unittest.TestCase):
    def test_quotes_and_newlines(self):
        rows = [["a,b", 'c"d', None], ["plain", "line\nbreak", 5]]
        text = csv_join(rows)
        self.assertEqual(text, '"a,b","c""d",\nplain,"line\nbreak",5')
        self.assertFalse(text.endswith("\n"))
        self.assertEqual(list(csv.reader(io.StringIO(text))), [["a,b", 'c"d', ""], ["plain", "line\nbreak", "5"]])

    def test_empty_rows_and_carriage_return(self):
        self.assertEqual(csv_join([]), "")
        self.assertEqual(csv_join([[], ["x\ry"]]), '\n"x\ry"')


if __name__ == "__main__":
    unittest.main()

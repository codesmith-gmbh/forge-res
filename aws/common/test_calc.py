import unittest
import aws.common.calc as calc


class TestCalc(unittest.TestCase):

    def test_something(self):
        tests = {
            'x': 1,  # simple Sequence
            'x-1': 0,  # Sequence starting with 0
            '8000 + x': 8001,  # Sequence starting with 8001
            '2 * (x-1)': 0  # even Sequence starting with 0
        }
        for expr, expected in tests.items():
            t = calc.CalcTransformer()
            parser = calc.parser(t)
            res = parser.parse(expr)
            self.assertEqual(expected, res)


if __name__ == '__main__':
    unittest.main()

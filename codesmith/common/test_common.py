import unittest
import codesmith.common.common as c

EVENT = {
    'StackId': 'arn:aws:cloudformation:us-west-2:123456789012:stack/teststack/51af3dc0-da77-11e4-872e-1234567db123'
}


class TestCommon(unittest.TestCase):
    def test_is_same_region(self):
        # 1. Correctness
        for r1, r2 in [
            (None, None),
            ('us-east-1', 'us-east-1'),
            (None, 'us-west-2'),
            ('us-west-2', None)
        ]:
            self.assertTrue(c.is_same_region(EVENT, r1, r2), "{} {}".format(r1, r2))

        # 2. Completeness
        for r1, r2 in [
            ('us-east-1', 'us-east-2'),
            (None, 'us-west-1'),
            ('us-west-1', None)
        ]:
            self.assertFalse(c.is_same_region(EVENT, r1, r2), "{} {}".format(r1, r2))


if __name__ == '__main__':
    unittest.main()

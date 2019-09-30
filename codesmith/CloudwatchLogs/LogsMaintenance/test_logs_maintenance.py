import unittest
import codesmith.CloudwatchLogs.LogsMaintenance.logs_maintenance as lm


class TestLogsMaintenance(unittest.TestCase):
    def test_fetch_regions(self):
        self.assertIn('eu-west-1', lm.fetch_regions())


if __name__ == '__main__':
    unittest.main()

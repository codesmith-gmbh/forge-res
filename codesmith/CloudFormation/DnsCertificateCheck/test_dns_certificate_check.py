import unittest
import codesmith.CloudFormation.DnsCertificateCheck.dns_certificate_check as dcc


class TestDnsCertificateCheck(unittest.TestCase):
    def test_certificate_region(self):
        for (cert, region) in [
            ('arn:aws:acm:eu-west-1:account:certificate/12345678-1234-1234-1234-123456789012', 'eu-west-1'),
            ('arn:aws:acm:us-east-1:account:certificate/12345678-1234-1234-1234-123456789012', 'us-east-1')
        ]:
            self.assertEqual(region, dcc.certificate_region(cert))

        with self.assertRaises(AttributeError):
            dcc.certificate_region("StackId-ResourceId-12345678")


if __name__ == '__main__':
    unittest.main()

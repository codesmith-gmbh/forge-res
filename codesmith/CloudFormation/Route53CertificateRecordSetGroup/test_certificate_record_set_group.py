import unittest

import codesmith.CloudFormation.Route53CertificateRecordSetGroup.certificate_record_set_group as cr


class TestCertificateRecordSetGroup(unittest.TestCase):
    def test_validate_properties(self):
        # 1. correctness
        for prop in [
            {
                'CertificateArn': 'arn:aws:acm:us-east-1:012345678901:certificate/000000000-1111-2222-3333-444444444444',
                'HostedZoneName': 'codesmith.ch',
            },
            {
                'CertificateArn': 'arn:aws:acm:us-east-1:012345678901:certificate/000000000-1111-2222-3333-444444444444',
                'HostedZoneName': 'codesmith.ch',
                'WithCaaRecords': 'false',
            }
        ]:
            properties = cr.validate_properties(prop)
            self.assertIsNotNone(properties, '{}'.format(prop))
            self.assertIsNotNone(properties.hosted_zone_id, 'HostedZoneId for {}'.format(prop))

        # 2. completeness
        for prop in [
            {},
            {
                'HostedZoneName': 'codesmith.ch',
            },
            {
                'CertificateArn': 'arn:aws:acm:us-east-1:012345678901:certificate/000000000-1111-2222-3333-444444444444',
            },
            {
                'CertificateArn': 'arn:aws:acm:us-east-1:012345678901:certificate/000000000-1111-2222-3333-444444444444',
                'HostedZoneName': 'error.ch',
            },
            {
                'CertificateArn': 'arn:aws:acm:us-east-1:012345678901:certificate/000000000-1111-2222-3333-444444444444',
                'HostedZoneId': '???',
            },
            {
                'CertificateArn': 'arn:aws:acm:us-east-1:012345678901:certificate/000000000-1111-2222-3333-444444444444',
                'HostedZoneName': 'codesmith.ch.',
                'HostedZoneId': '????',
            },
            {
                'CertificateArn': 'arn:aws:acm:us-east-1:012345678901:certificate/000000000-1111-2222-3333-444444444444',
                'HostedZoneName': 'codesmith.ch.',
                'WithCaaRecords': 'hello',
            },
        ]:
            with self.assertRaises(Exception, msg="{}".format(prop)):
                cr.validate_properties(prop)


if __name__ == '__main__':
    unittest.main()

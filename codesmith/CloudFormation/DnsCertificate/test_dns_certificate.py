import unittest

from box import Box

import codesmith.CloudFormation.DnsCertificate.dns_certificate as dc

TEST_DOMAINS = ['test-forge.codesmith.ch.', 'test-forge-san.codesmith.ch.']

SNS_MESSAGE_TEST_EVENT = {
    'StackId': 'arn:aws:cloudformation:eu-west-1:999999999999:stack/test-stack/00000000-0000-0000-0000-000000000000',
    'LogicalResourceId': 'TestResource'
}


class TestDnsCertificate(unittest.TestCase):
    def test_needs_new(self):
        # 1. correctness
        for ps in [
            (
                    {
                        'DomainName': 'test.com',
                        'Region': 'us-east-1',
                        'SubjectAlternativeNames': ['hello1.test.com', 'hello2.test.com'],
                        'Tags': [{'Key': 'key', 'Value': 'value1'}],
                        'HostedZoneId': 'ABCD'
                    },
                    {
                        'DomainName': 'hello3.test.com',
                        'Region': 'us-east-1',
                        'SubjectAlternativeNames': ['hello2.test.com', 'hello1.test.com'],
                        'Tags': [{'Key': 'key', 'Value': 'value1'}],
                        'HostedZoneId': 'ABCD'
                    }
            ),
            (
                    {
                        'DomainName': 'test.com',
                        'Region': 'us-east-1',
                        'SubjectAlternativeNames': ['hello1.test.com', 'hello2.test.com'],
                        'Tags': [{'Key': 'key', 'Value': 'value1'}],
                        'HostedZoneId': 'ABCD'
                    },
                    {
                        'DomainName': 'test.com',
                        'Region': 'eu-west-1',
                        'SubjectAlternativeNames': ['hello2.test.com', 'hello1.test.com'],
                        'Tags': [{'Key': 'key', 'Value': 'value1'}],
                        'HostedZoneId': 'ABCD'
                    }
            ),
            (
                    {
                        'DomainName': 'test.com',
                        'Region': 'us-east-1',
                        'SubjectAlternativeNames': ['hello1.test.com', 'hello3.test.com'],
                        'Tags': [{'Key': 'key', 'Value': 'value1'}],
                        'HostedZoneId': 'ABCD'
                    },
                    {
                        'DomainName': 'test.com',
                        'Region': 'us-east-1',
                        'SubjectAlternativeNames': ['hello2.test.com', 'hello1.test.com'],
                        'Tags': [{'Key': 'key', 'Value': 'value1'}],
                        'HostedZoneId': 'ABCD'
                    }
            ),
            (
                    {
                        'DomainName': 'test.com',
                        'Region': 'us-east-1',
                        'SubjectAlternativeNames': ['hello1.test.com', 'hello2.test.com', 'hello3.test.com'],
                        'Tags': [{'Key': 'key', 'Value': 'value1'}],
                        'HostedZoneId': 'ABCD'
                    },
                    {
                        'DomainName': 'test.com',
                        'Region': 'us-east-1',
                        'SubjectAlternativeNames': ['hello2.test.com', 'hello1.test.com'],
                        'Tags': [{'Key': 'key', 'Value': 'value1'}],
                        'HostedZoneId': 'ABCD'
                    }
            )
        ]:
            p1, p2 = [Box(p, camel_killer_box=True) for p in ps]
            self.assertTrue(dc.needs_new({}, p1, p2), '{} {}'.format(p1, p2))

        # 2. completeness
        for ps in [
            (
                    {
                        'DomainName': 'test.com',
                        'Region': 'us-east-1',
                        'SubjectAlternativeNames': ['hello1.test.com', 'hello2.test.com'],
                        'Tags': [{'Key': 'key', 'Value': 'value1'}],
                        'HostedZoneId': 'ABCD'
                    },
                    {
                        'DomainName': 'test.com',
                        'Region': 'us-east-1',
                        'SubjectAlternativeNames': ['hello2.test.com', 'hello1.test.com'],
                        'Tags': [{'Key': 'key', 'Value': 'value2'}],
                        'HostedZoneId': 'ABCD'
                    }
            )
        ]:
            p1, p2 = [Box(p, camel_killer_box=True) for p in ps]
            self.assertFalse(dc.needs_new({}, p1, p2), '{} {}'.format(p1, p2))

    def test_validate_properties(self):
        # 1. correctness
        for prop in [
            {
                'HostedZoneName': 'codesmith.ch',
                'DomainName': 'codesmith.ch',
            },
            {
                'HostedZoneName': 'codesmith.ch.',
                'DomainName': 'test.codesmith.ch',
                'SubjectAlternativeNames': [
                    'test2.codesmith.ch',
                    'test3.codesmith.ch',
                ],
                'WithCaaRecords': False,
                'Region': 'us-east-1',
                'Tags': {
                    'Key': 'Stan',
                    'Value': 'stan',
                }
            }
        ]:
            properties = dc.validate_properties(prop)
            self.assertIsNotNone(properties, '{}'.format(prop))
            self.assertIsNotNone(properties.hosted_zone_id, 'HostedZoneId for {}'.format(prop))

        # 2. completeness
        for prop in [
            {},
            {
                'HostedZoneName': 'codesmith.ch',
            },
            {
                'DomainName': 'codesmith.ch',
            },
            {
                'DomainName': 'error.ch',
                'HostedZoneName': 'error.ch',
            },
            {
                'DomainName': 'error.ch',
                'HostedZoneId': '???',
            },
            {
                'HostedZoneName': 'codesmith.ch.',
                'HostedZoneId': '????',
                'DomainName': 'test.codesmith.ch',
            },
            {
                'HostedZoneName': 'codesmith.ch.',
                'DomainName': 'test.error.ch',
            },
            {
                'HostedZoneName': 'codesmith.ch.',
                'DomainName': 'test.codesmith.ch',
                'SubjectAlternativeNames': [
                    'test2.codesmith.ch',
                    'test.error.ch',
                ],
            },
            {
                'HostedZoneName': 'codesmith.ch.',
                'DomainName': 'test.codesmith.ch',
                'WithCaaRecords': 'hello',
            },
        ]:
            with self.assertRaises(Exception, msg="{}".format(prop)):
                dc.validate_properties(prop)

    def test_integration(self):
        properties = self.test_properties()
        cert_proc = dc.CertificateProcessor(None, properties)
        try:
            certificate_arn = cert_proc.create_certificate()
            for sub_test in [getattr(self, m) for m in dir(self) if m.startswith('_test_')]:
                sub_test(certificate_arn)
        finally:
            cert_proc.delete_certificate()
            pass

    def test_properties(self):
        return dc.validate_properties({
            'HostedZoneName': 'codesmith.ch.',
            'DomainName': 'test-forge.codesmith.ch',
            'SubjectAlternativeNames': ['test-forge-san.codesmith.ch']
        })

    def _test_creation_change_generation_with_caa(self, certificate_arn):
        properties = self.test_properties()
        properties.with_caa_records = True
        cert_proc = dc.CertificateProcessor(certificate_arn, properties)
        changes = cert_proc.generate_create_record_set_group_changes()
        self.assertEqual(4, len(changes))
        for domain in TEST_DOMAINS:
            self.assertTrue(has_cname_record(changes, domain), 'missing cname for {}'.format(domain))
            self.assertTrue(has_caa_record(changes, domain), 'missing caa for {}'.format(domain))

    def _test_creation_change_generation_without_caa(self, certificate_arn):
        properties = self.test_properties()
        properties.with_caa_records = False
        cert_proc = dc.CertificateProcessor(certificate_arn, properties)
        changes = cert_proc.generate_create_record_set_group_changes()
        self.assertEqual(2, len(changes))
        for domain in TEST_DOMAINS:
            self.assertTrue(has_cname_record(changes, domain), 'missing cname for {}'.format(domain))
            self.assertFalse(has_caa_record(changes, domain), 'caa for {}'.format(domain))

    def _test_update_change_generation_without_caa(self, certificate_arn):
        properties = self.test_properties()
        cert_proc = dc.CertificateProcessor(certificate_arn, properties)
        changes = cert_proc.generate_create_caa_records_changes()
        self.assertEqual(2, len(changes))
        for domain in TEST_DOMAINS:
            self.assertFalse(has_cname_record(changes, domain), 'cname for {}'.format(domain))
            self.assertTrue(has_caa_record(changes, domain), 'missing caa for {}'.format(domain))

    def _test_dns_records_deletion_stardard(self, certificate_arn):
        properties = self.test_properties()
        cert_proc = dc.CertificateProcessor(certificate_arn, properties)
        try:
            cert_proc.create_record_set_group()
        except Exception as e:
            self.fail(str(e))
        else:
            cert_proc.delete_record_set_group()

    def _test_dns_records_deletion_failover(self, certificate_arn):
        properties = self.test_properties()
        cert_proc = dc.CertificateProcessor(certificate_arn, properties)
        try:
            cert_proc.create_record_set_group()
        except Exception as e:
            self.fail(str(e))
        else:
            try:
                changes = cert_proc.generate_delete_record_set_group_changes()
                cert_proc.execute_delete_changes(changes[0:1])
            except Exception as e:
                self.fail(str(e))
            else:
                cert_proc.delete_record_set_group()

    def _test_dns_records_deletion_faulty(self, certificate_arn):
        properties = self.test_properties()
        cert_proc = dc.CertificateProcessor(certificate_arn, properties)
        cert_proc.delete_record_set_group()

    def test_sns_message_id(self):
        try:
            dc.delete_sns_message_ssm_parameter(SNS_MESSAGE_TEST_EVENT)
            self.assertFalse(dc.should_skip_message('1', SNS_MESSAGE_TEST_EVENT), "should not skip message 1")
            self.assertTrue(dc.should_skip_message('1', SNS_MESSAGE_TEST_EVENT), "should skip message 1")
            self.assertFalse(dc.should_skip_message('2', SNS_MESSAGE_TEST_EVENT), "should not skip message 2")
        finally:
            dc.delete_sns_message_ssm_parameter(SNS_MESSAGE_TEST_EVENT)


def has_cname_record(changes, domain):
    for change in changes:
        record_set = change['ResourceRecordSet']
        if record_set['Type'] == 'CNAME' and record_set['Name'].endswith(domain):
            return True
    return False


def has_caa_record(changes, domain):
    for change in changes:
        record_set = change['ResourceRecordSet']
        if record_set['Type'] == 'CAA' and record_set['Name'] == domain:
            return True
    return False


if __name__ == '__main__':
    unittest.main()

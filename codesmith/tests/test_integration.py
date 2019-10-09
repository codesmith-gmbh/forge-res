import random
import string
import time
import unittest

import boto3

import codesmith.tests.cfn as tcfn

MAX_WAIT = 500


class TestS3Cleanup(unittest.TestCase):
    cf = None
    s3 = None
    stack_id = None

    @classmethod
    def setUpClass(cls):
        cls.cf = us_east_1_client('cloudformation')
        cls.s3 = us_east_1_client('s3')
        with open('templates/s3.yaml') as f:
            stack_name = cls.__name__ + random_string(15)
            stack = cls.cf.create_stack(
                StackName=stack_name,
                Parameters=[
                    {'ParameterKey': 'Version',
                     'ParameterValue': '0'}
                ],
                TemplateBody=f.read(),
                Capabilities=['CAPABILITY_AUTO_EXPAND']
            )
        cls.stack_id = stack['StackId']
        wait_for_stack_to_stabilize(cls.cf, cls.stack_id)

    @classmethod
    def tearDownClass(cls):
        cls.cf.delete_stack(StackName=cls.stack_id)
        wait_for_stack_to_stabilize(cls.cf, cls.stack_id)

    def test_prefix_cleanup(self):
        test_bucket = tcfn.find_output(TestS3Cleanup.cf, stack_id=TestS3Cleanup.stack_id, output_key='BucketName')
        self.assertIsNotNone(test_bucket)
        for i in range(2):
            TestS3Cleanup.s3.put_object(
                Bucket=test_bucket,
                Key=f'test/{i}/testfile.txt',
                Body=bytes('test!', 'utf-8')
            )
        TestS3Cleanup.cf.update_stack(
            StackName=TestS3Cleanup.stack_id,
            UsePreviousTemplate=True,
            Parameters=[
                {'ParameterKey': 'Version',
                 'ParameterValue': '1'}
            ]
        )
        wait_for_stack_to_stabilize(TestS3Cleanup.cf, TestS3Cleanup.stack_id)
        with self.assertRaises(TestS3Cleanup.s3.exceptions.NoSuchKey):
            TestS3Cleanup.s3.get_object(
                Bucket=test_bucket,
                Key='test/0/testfile.txt'
            )
        self.assertIsNotNone(TestS3Cleanup.s3.head_object(
            Bucket=test_bucket,
            Key='test/1/testfile.txt'
        ))


class TestDnsCertificate(unittest.TestCase):
    cf = None
    acm = None
    stack_id = None

    @classmethod
    def setUpClass(cls) -> None:
        cls.cf = us_east_1_client('cloudformation')
        cls.acm = us_east_1_client('acm')
        with open('templates/dnscert.yaml') as f:
            stack_name = cls.__name__ + random_string(15)
            stack = cls.cf.create_stack(
                StackName=stack_name,
                TemplateBody=f.read(),
                Capabilities=['CAPABILITY_AUTO_EXPAND']
            )
        cls.stack_id = stack['StackId']
        wait_for_stack_to_stabilize(cls.cf, cls.stack_id)

    @classmethod
    def tearDownClass(cls):
        cls.cf.delete_stack(StackName=cls.stack_id)
        wait_for_stack_to_stabilize(cls.cf, cls.stack_id)

    def test_certificate_creation(self):
        certificate_arn = tcfn.find_output(TestDnsCertificate.cf, stack_id=TestDnsCertificate.stack_id,
                                           output_key='CertificateArn')
        certificate = TestDnsCertificate.acm.describe_certificate(CertificateArn=certificate_arn)
        self.assertIsNotNone(certificate)


def us_east_1_client(service):
    client = boto3.client(service)
    if 'us-east-1' != client._client_config.region_name:
        raise RuntimeError('Tests must run in us-east-1')
    return client


def random_string(length):
    return ''.join([random.choice(string.ascii_letters) for _ in range(length)])


def wait_for_stack_to_stabilize(cf, stack_id):
    for i in range(MAX_WAIT):
        stacks = cf.describe_stacks(StackName=stack_id)
        status = stacks['Stacks'][0]['StackStatus']
        if status.endswith('COMPLETE'):
            return
        elif status.endswith('FAILED'):
            raise RuntimeError(f'Stack {stack_id} failed on status {status}')
        time.sleep(3.0)
    raise RuntimeError(f'Stack {stack_id} failed to stabilize')


if __name__ == '__main__':
    unittest.main()

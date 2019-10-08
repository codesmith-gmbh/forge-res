import unittest

import boto3
import box

import codesmith.CloudFormation.S3Cleanup.s3_cleanup as s3c
import codesmith.tests.cfn as tcfn

TEST_PREFIX = "s3cleanup"


class TestS3Cleanup(unittest.TestCase):
    cf = None
    test_bucket_name = None

    @classmethod
    def setUpClass(cls) -> None:
        cls.cf = boto3.client('cloudformation')
        if 'us-east-1' != cls.cf._client_config.region_name:
            raise RuntimeError('Tests must run in us-east-1')
        cls.test_bucket_name = tcfn.find_output(cls.cf, stack_id='ForgeTestBucket', output_key='TestBucket')

    def test_cleanup(self):
        properties = box.Box({'bucket': TestS3Cleanup.test_bucket_name, 'prefix': TEST_PREFIX})

        # 1. before the test, the prefix should be empty for the bucket
        self.assertBucketEmpty(properties)

        # 2. deleting an empty bucket must succeed
        s3c.delete_objects(properties)

        # 3. we put 2 versions of an object
        for i in range(2):
            s3c.s3.put_object(
                Bucket=properties.bucket,
                Key=properties.prefix + '/testfile.txt',
                Body=bytes(str(i), 'utf-8')
            )
        self.assertBucketLen(2, properties)

        # 4. we clean the bucket at the prefix
        s3c.delete_objects(properties)
        self.assertBucketEmpty(properties)

    def assertBucketEmpty(self, properties):
        objects = s3c.s3.list_object_versions(Bucket=properties.bucket, Prefix=properties.prefix)
        self.assertIsNone(objects.get('Versions'))

    def assertBucketLen(self, expected_len, properties):
        objects = s3c.s3.list_object_versions(Bucket=properties.bucket, Prefix=properties.prefix)
        versions = objects.get('Versions')
        self.assertIsNotNone(versions)
        self.assertEqual(expected_len, len(versions))


def find_test_bucket_name(stacks):
    outputs = stacks['Stacks'][0]['Outputs']
    for output in outputs:
        if output['OutputKey'] == 'TestBucket':
            return output['OutputValue']
    return None


if __name__ == '__main__':
    unittest.main()

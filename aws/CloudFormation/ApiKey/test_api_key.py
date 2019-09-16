import unittest
import aws.CloudFormation.ApiKey.api_key as api_key


class TestApiKey(unittest.TestCase):

    def test_properties_create_event(self):
        p = api_key.properties({
            "RequestType": "Create",
            "ResponseURL": "http://pre-signed-S3-url-for-response",
            "StackId": "arn:aws:cloudformation:eu-west-1:123456789012:stack/MyStack/guid",
            "RequestId": "unique id for this create request",
            "ResourceType": "Custom::TestResource",
            "LogicalResourceId": "MyTestResource",
            "ResourceProperties": {
                "Ordinal": "1"
            }
        })

        self.assertFalse(p is None)

    def test_properties_update_event(self):
        p = api_key.properties({
            "RequestType": "Update",
            "ResponseURL": "http://pre-signed-S3-url-for-response",
            "StackId": "arn:aws:cloudformation:eu-west-1:123456789012:stack/MyStack/guid",
            "RequestId": "unique id for this create request",
            "ResourceType": "Custom::TestResource",
            "PhysicalResourceId": "MyTestResource-Physical-Resource-Id",
            "LogicalResourceId": "MyTestResource",
            "ResourceProperties": {
                "Ordinal": "2"
            },
            "OldResourceProperties": {
                "Ordinal": "1"
            }
        })

        self.assertFalse(p is None)

    def test_delete_unexisting_key(self):
        test_key_id = 'test-forge-1'
        key_id = api_key.delete_api_key(test_key_id)
        self.assertEqual(test_key_id, key_id)

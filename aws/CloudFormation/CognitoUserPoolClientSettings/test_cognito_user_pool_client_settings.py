import unittest
import json
import aws.CloudFormation.CognitoUserPoolClientSettings.cognito_user_pool_client_settings as cupcs
import yaml


class MyTestCase(unittest.TestCase):
    def test_properties_json(self):
        with open('./testdata/cogclientset.json') as f:
            p = json.load(f)
        self.assertIsNotNone(cupcs.validate_properties(p))

    def test_properties_yaml(self):
        with open('./testdata/cogclientset.yaml') as f:
            p = yaml.load(f, Loader=yaml.SafeLoader)
        self.assertIsNotNone(cupcs.validate_properties(p))


if __name__ == '__main__':
    unittest.main()

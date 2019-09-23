import unittest

import aws.CloudFormation.CognitoUserPoolDomain.cognito_user_pool_domain as cupd


class MyTestCase(unittest.TestCase):
    def test_delete_unexisting_cognito_user_domain(self):
        p = cupd.validate_properties({
            'Domain': 'test.codesmith.ch',
            'UserPoolId': 'eu-west-1_1234567890A'
        })
        self.assertIsNone(cupd.delete_user_pool_domain(p))


if __name__ == '__main__':
    unittest.main()

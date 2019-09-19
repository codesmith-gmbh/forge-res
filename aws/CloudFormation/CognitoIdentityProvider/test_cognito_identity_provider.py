import unittest

from box import Box

import aws.CloudFormation.CognitoIdentityProvider.cognito_identity_provider as cip


class TestCognitoIdentityProvider(unittest.TestCase):
    def test_delete_unexisting_identity_provider(self):
        properties = Box({'user_pool_id': 'a_aaa_test_codesmith_forge', 'provider_name': 'Google'})
        self.assertIsNone(cip.delete_identity_provider(properties))


if __name__ == '__main__':
    unittest.main()

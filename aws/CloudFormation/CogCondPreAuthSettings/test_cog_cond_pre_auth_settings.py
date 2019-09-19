import unittest

from schema import SchemaError

import aws.CloudFormation.CogCondPreAuthSettings.cog_cond_pre_auth_settings as settings


class TestCogCondPreAuthSettings(unittest.TestCase):
    def test_validate_properties(self):
        self.assertTrue(settings.validate_properties({'UserPoolId': 'id1',
                                                      'UserPoolClientId': 'id2',
                                                      'All': 1,
                                                      'Domains': [],
                                                      'Emails': []}))

        try:
            self.assertFalse(settings.validate_properties({}))
        except SchemaError as e:
            self.assertTrue(e)

        try:
            self.assertFalse(settings.validate_properties({'UserPoolId': '',
                                                           'UserPoolClientId': 'id2'}))
        except SchemaError as e:
            self.assertTrue(e)

        try:
            self.assertFalse(settings.validate_properties({'UserPoolId': 'id1',
                                                           'UserPoolClientId': ''}))
        except SchemaError as e:
            self.assertTrue(e)


if __name__ == '__main__':
    unittest.main()

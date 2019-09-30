import unittest
import codesmith.Cognito.CogCondPreAuth.cog_cond_pre_auth as ccpa

TEST_EVENT = {
    'userPoolId': 'ashtasht',
    'callerContext': {
        'clientId': 'ashtashtas'
    },
    'request': {
        'userAttributes': {
            'email': 'stan@codesmith.ch'
        },
        'validationData': {}
    },
    'response': {},
}


class TestCogCondPreAuth(unittest.TestCase):
    def test_email_and_domain_of_user(self):
        email, domain = ccpa.email_and_domain_of_user(TEST_EVENT)
        self.assertEqual('stan@codesmith.ch', email)
        self.assertEqual('codesmith.ch', domain)


if __name__ == '__main__':
    unittest.main()

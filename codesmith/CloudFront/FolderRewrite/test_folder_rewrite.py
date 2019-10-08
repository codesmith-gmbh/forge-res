import unittest
import codesmith.CloudFront.FolderRewrite.folder_rewrite as fr

EXPECTED = {
    'uri': '/test/test2/index.html',
    'method': 'GET',
    'clientIp': '2001:cdba::3257:9652',
    'headers': {
        'user-agent': [
            {
                'key': 'User-Agent',
                'value': 'test-agent'
            }
        ],
        'host': [
            {
                'key': 'Host',
                'value': 'd123.cf.net'
            }
        ]
    }
}


class MyTestCase(unittest.TestCase):
    def setUp(self) -> None:
        self.event = {
            'Records': [
                {
                    'cf': {
                        'config': {
                            'distributionId': 'EXAMPLE'
                        },
                        'request': {
                            'method': 'GET',
                            'clientIp': '2001:cdba::3257:9652',
                            'headers': {
                                'user-agent': [
                                    {
                                        'key': 'User-Agent',
                                        'value': 'test-agent'
                                    }
                                ],
                                'host': [
                                    {
                                        'key': 'Host',
                                        'value': 'd123.cf.net'
                                    }
                                ]
                            }
                        }
                    }
                }
            ]
        }

    def set_uri(self, uri):
        self.event['Records'][0]['cf']['request']['uri'] = uri

    def test_folder_without_slash(self):
        self.set_uri('/test/test2')
        self.assertEvent()

    def test_folder_with_slash(self):
        self.set_uri('/test/test2/')
        self.assertEvent()

    def test_file(self):
        self.set_uri('/test/test2/index.html')
        self.assertEvent()

    def assertEvent(self):
        self.assertEqual(EXPECTED, fr.handler(self.event, None))


if __name__ == '__main__':
    unittest.main()

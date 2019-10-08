import json
import unittest

import codesmith.CloudFormation.ForgeResources.forge_resources as fr


class TestForgeResources(unittest.TestCase):
    def test_resources_service_tokens(self):
        response = fr.handler({
            'requestId': 'testrequest',
            'fragment': {
                'Resources': {
                    'S3Cleanup': {
                        'Type': 'Forge::S3::Cleanup',
                        'Properties': {
                            'Bucket': 'TheBucket'
                        }
                    }
                }
            }
        }, None)
        expected = {
            'requestId': 'testrequest',
            'status': 'success',
            'fragment': {
                'Resources': {
                    'S3Cleanup': {
                        'Type': 'AWS::CloudFormation::CustomResource',
                        'Properties': {
                            'ServiceToken': {'Fn::ImportValue': 'ForgeResources-S3Cleanup'},
                            'Bucket': 'TheBucket'
                        }
                    }
                }
            }
        }
        self.assertEqual(expected, response)
        self.assertIsNotNone(json.dumps(response))


if __name__ == '__main__':
    unittest.main()

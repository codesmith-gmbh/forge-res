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

    def test_redirector(self):
        response = fr.handler({
            'requestId': 'testrequest',
            'fragment': {
                'Resources': {
                    'Redirector': {
                        'Type': 'Forge::ApiGateway::Redirector',
                        'Properties': {
                            'DomainName': 'www.codesmith.ch',
                            'Location': 'https://codesmith.ch',
                            'CertificateArn': {'Ref': 'Certificate'}
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
                    'RedirectorApi': {
                        'Type': 'AWS::Serverless::Api',
                        'Properties': {
                            'DefinitionBody': {
                                'info': {
                                    'version': '1.0'
                                },
                                'definitions': {
                                    'Empty': {
                                        'type': 'object',
                                        'title': 'Empty Schema'
                                    }
                                },
                                'schemes': [
                                    'https'
                                ],
                                'swagger': '2.0',
                                'paths': {
                                    '/': {
                                        'get': {
                                            'x-amazon-apigateway-integration': {
                                                'type': 'mock',
                                                'passthroughBehavior': 'when_no_match',
                                                'requestTemplates': {
                                                    'application/json': '{"statusCode": 301}'
                                                },
                                                'responses': {
                                                    '301': {
                                                        'statusCode': '301',
                                                        'responseParameters': {
                                                            'method.response.header.Location': {
                                                                'Fn::Sub': [
                                                                    "'${Location}'",
                                                                    {
                                                                        'Location': 'https://codesmith.ch'
                                                                    }
                                                                ]
                                                            }
                                                        }
                                                    }
                                                }
                                            },
                                            'consumes': [
                                                'application/json'
                                            ],
                                            'responses': {
                                                '301': {
                                                    'headers': {
                                                        'Location': {
                                                            'type': 'string'
                                                        }
                                                    },
                                                    'description': '301 response'
                                                }
                                            }
                                        }
                                    }
                                }
                            },
                            'Name': {
                                'Fn::Sub': '${AWS::StackName}-RedirectorApi'
                            },
                            'StageName': 'redirector'
                        }
                    },
                    'RedirectorBasePathMapping': {
                        'Type': 'AWS::ApiGateway::BasePathMapping',
                        'Properties': {
                            'DomainName': {
                                'Ref': 'RedirectorDomainName'
                            },
                            'RestApiId': {
                                'Ref': 'RedirectorApi'
                            },
                            'Stage': 'redirector'
                        }
                    },
                    'RedirectorDomainName': {
                        'Type': 'AWS::ApiGateway::DomainName',
                        'Properties': {
                            'CertificateArn': {
                                'Ref': 'Certificate'
                            },
                            'EndpointConfiguration': {
                                'Types': [
                                    'EDGE'
                                ]
                            },
                            'DomainName': 'www.codesmith.ch'
                        }
                    }
                }
            }
        }
        self.assertEqual(expected, response)
        self.assertIsNotNone(json.dumps(response))

    def test_normal_resource(self):
        response = fr.handler({
            'requestId': 'testrequest',
            'fragment': {
                'Resources': {
                    'Bucket': {
                        'Type': 'AWS::S3::Bucket',
                        'Properties': {'VersioningConfiguration': {'Status': 'Enabled'}}
                    }
                }
            }
        }, None)
        expected = {
            'requestId': 'testrequest',
            'status': 'success',
            'fragment': {
                'Resources': {
                    'Bucket': {
                        'Type': 'AWS::S3::Bucket',
                        'Properties': {'VersioningConfiguration': {'Status': 'Enabled'}}
                    }
                }
            }
        }
        self.assertEqual(expected, response)
        self.assertIsNotNone(json.dumps(response))


if __name__ == '__main__':
    unittest.main()

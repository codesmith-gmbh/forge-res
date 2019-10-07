import structlog
from schema import Schema

from codesmith.common.schema import box, non_empty_string

log = structlog.get_logger()

SERVICE_TOKENS = {
    'Forge::ApiGateway::ApiKey': 'ForgeResources-ApiKey',
    'Forge::Cognito::CondPreAuthSettings': 'ForgeResources-CogCondPreAuthSettings',
    'Forge::Cognito::IdentityProvider': 'ForgeResources-CognitoIdentityProvider',
    'Forge::Cognito::UserPoolClientSettings': 'ForgeResources-CognitoUserPoolClientSettings',
    'Forge::Cognito::UserPoolDomain': 'ForgeResources-CognitoUserPoolDomain',
    'Forge::CertificateManager::DnsCertificate': 'ForgeResources-DnsCertificate',
    'Forge::ECR::Cleanup': 'ForgeResources-EcrCleanup',
    'Forge::ElasticLoadBalancingV2::ListenerRuleSwapper': 'ForgeResources-ListenerRuleSwapper',
    'Forge::RDS::PostgresDatabase': 'ForgeResources-PostgresDatabase',
    'Forge::RDS::DbInstanceResourceId': 'ForgeResources-DbInstanceResourceId',
    'Forge::S3::Cleanup': 'ForgeResources-S3Cleanup',
    'Forge::Utils::Sequence': 'ForgeResources-Sequence',
    'Forge::Utils::SequenceValue': 'ForgeResources-SequenceValue',
}


def handler(event, _):
    log.msg('', event=event)
    fragment = event['fragment']
    transform(fragment)
    return {
        'requestId': event['requestId'],
        'status': 'success',
        'fragment': fragment
    }


def transform(fragment):
    resources = fragment['Resources']
    for (name, val) in resources.items():
        type = val['Type']
        service_token = SERVICE_TOKENS.get(type)
        if service_token:
            val['Type'] = 'AWS::CloudFormation::CustomResource'
            val['Properties']['ServiceToken'] = {'Fn::ImportValue', service_token}
        elif type == 'Forge::ApiGateway::Redirector':
            del resources[name]
            redirector_properties = validate_properties(val['Properties'])
            for generator in [
                api_gateway_redictor_api_resource,
                domain_name_redirector_api_resource,
                base_path_mapping_redirector_api_resource
            ]:
                (resource_name, resource_val) = generator(name, redirector_properties)
                resources[resource_name] = resource_val
        else:
            pass


REDIRECTOR_PROPERTIES_SCHEMA = Schema({
    'CertificateArn': non_empty_string,
    'DomainName': non_empty_string,
    'Location': non_empty_string,
})

STAGE_NAME = 'redirector'


def validate_properties(properties):
    return box(properties, schema=REDIRECTOR_PROPERTIES_SCHEMA)


def api_gateway_redictor_api_resource(redirector_name, properties):
    name = api_name(redirector_name)
    resource = {
        'Type': 'AWS::Serverless::Api',
        'Properties': {
            'Name': {'Fn::Sub', '${AWS::StackName}-' + name},
            'StageName': STAGE_NAME,
            'DefinitionBody': {
                'swagger': '2.0',
                'info': {'version': '1.0'},
                'schemes': ['https'],
                'path': {
                    '/': {
                        'get': {
                            'consumes': ['application/json'],
                            'responses': {
                                '301': {
                                    'description': '301 response',
                                    'headers': {
                                        'Location': {'type': 'string'}
                                    }
                                }
                            },
                            'x-amazon-apigateway-integration': {
                                'responses': {
                                    '301': {
                                        'statusCode': '301',
                                        'responseParameters': {
                                            'method.response.header.Location': {
                                                'Fn::Sub': [
                                                    "'${Location}'",
                                                    {'Location': properties.location}
                                                ]
                                            }
                                        }
                                    }
                                },
                                'requestTemplates': {
                                    'application/json': '{"statusCode": 301}'
                                },
                                'passthroughBehavior': 'when_no_match',
                                'type': 'mock'
                            }
                        }
                    }
                },
                'definitions': {
                    'Empty': {
                        'type': 'object',
                        'title': 'Empty Schema'
                    }
                }
            }
        }
    }
    return name, resource


def domain_name_redirector_api_resource(redirector_name, properties):
    resource = {
        'Type': 'AWS::ApiGateway::DomainName',
        'Properties': {
            'CertificateArn': properties.certificate_arn,
            'DomainName': properties.domain_name,
            'EndpointConfiguration': {
                'Types': ['EDGE']
            }
        }
    }
    return domain_name(redirector_name), resource


def base_path_mapping_redirector_api_resource(redirector_name, properties):
    resource = {
        'Type': 'AWS::ApiGateway::DomainName',
        'Properties': {
            'DomainName': {'Ref': domain_name(redirector_name)},
            'RestApiId': {'Ref': api_name(redirector_name)},
            'Stage': STAGE_NAME
        }
    }
    return base_path_mapping_name(redirector_name), resource


def api_name(redirector_name):
    return redirector_name + 'Api'


def domain_name(redirector_name):
    return redirector_name + 'DomainName'


def base_path_mapping_name(redirector_name):
    return redirector_name + 'BasePathMapping'

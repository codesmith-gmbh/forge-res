import logging

import boto3
from box import Box
from crhelper import CfnResource
from schema import And, Optional, Schema

from aws.common.cfn import resource_properties, physical_resource_id
from aws.common.schema import not_empty

helper = CfnResource()
logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

idp = boto3.client('cognito-idp')

properties_schema = Schema({
    'UserPoolId': And(str, not_empty, error='not empty string for UserPoolId'),
    'Domain': And(str, not_empty, error='not empty string for UserPoolId'),
    Optional('CustomDomainConfig', {}): {
        'CertificateArn': And(str, not_empty, error='not empty string for CertificateArn')}
})


def validate_properties(properties):
    return Box(properties_schema.validate(properties), camel_killer_box=True)


@helper.create
@helper.update
def create(event, _):
    properties = validate_properties(resource_properties(event))
    certificate_arn = properties.custom_domain_config.certificate_arn
    if certificate_arn is None:
        out = idp.create_user_pool_domain(
            Domain=properties.domain,
            UserPoolId=properties.user_pool_id
        )
    else:
        out = idp.create_user_pool_domain(
            Domain=properties.domain,
            UserPoolId=properties.user_pool_id,
            CustomDomainConfig=properties.custom_domain_config
        )
    cloudfront_domain = out.get('CloudFrontDomain')
    if cloudfront_domain is None:
        cloudfront_domain = ''
        domain = properties.domain + '.auth.' + idp._client_config.region_name + '.amazoncognito.com'
    else:
        domain = properties.domain

    helper.Data.update({
        'UserPoolId': properties.user_pool_id,
        'CloudFrontDomain': cloudfront_domain,
        'Domain': domain
    })

    return properties.domain


@helper.delete
def delete(event, _):
    properties = validate_properties(resource_properties(event))
    delete_user_pool_domain(properties)
    return physical_resource_id(event)


def delete_user_pool_domain(properties):
    try:
        idp.delete_user_pool_domain(
            Domain=properties.domain,
            UserPoolId=properties.user_pool_id
        )
    except idp.exceptions.InvalidParameterException:
        pass


def handler(event, context):
    logger.info('event: %s', event)
    helper(event, context)

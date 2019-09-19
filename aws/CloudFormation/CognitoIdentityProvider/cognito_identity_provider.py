import logging

import boto3
from box import Box
from crhelper import CfnResource
from schema import And, Schema

from aws.common.cfn import resource_properties, old_resource_properties, physical_resource_id
from aws.common.schema import not_empty

helper = CfnResource()
logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

ssm = boto3.client('ssm')
idp = boto3.client('cognito-idp')

properties_schema = Schema({
    'UserPoolId': And(str, not_empty, error='not empty string for UserPoolId'),
    'ProviderName': And(str, not_empty, error='not empty string for ProviderName'),
    'ClientIdParameter': And(str, not_empty, error='not empty string for ClientIdParameter'),
    'ClientSecretParameter': And(str, not_empty, error='not empty string for ClientSecretParameter'),
    'ProviderType': And(str, not_empty, error='not empty string for ProviderType'),
    'AuthorizeScopes': [str],
    'AttributeMapping': {str: str}
})


def validate_properties(properties):
    return Box(properties_schema.validate(properties), camel_killer_box=True)


def derive_details(properties):
    details = {}
    if properties.provider_type == 'Google':
        details["authorize_url"] = "https://accounts.google.com/o/oauth2/v2/auth"
        details["authorize_scopes"] = " ".join(properties.authorize_scopes)
        details["attributes_url_add_attributes"] = "true"
        details["token_url"] = "https://www.googleapis.com/oauth2/v4/token"
        details["attributes_url"] = "https://people.googleapis.com/v1/people/me?personFields="
        details["oidc_issuer"] = "https://accounts.google.com"
        details["token_request_method"] = "POST"
    else:
        raise RuntimeError(f'Unknown provider {properties.provider_type}')
    return details


def read_parameter(parameter_name):
    parameter = ssm.get_parameter(
        Name=parameter_name,
        WithDecryption=True
    )
    return parameter['Parameter']['Value']


@helper.create
def create(event, _):
    properties = validate_properties(resource_properties(event))
    return create_identity_provider(properties)


def create_identity_provider(properties):
    details = derive_details(properties)
    user_pool_id = properties.user_pool_id
    provider_name = properties.provider_name
    idp.create_identity_provider(
        UserPoolId=user_pool_id,
        ProviderName=provider_name,
        ProviderType=properties.provider_type,
        ProviderDetails=details,
        AttributeMapping=properties.attribute_mapping
    )
    helper.Data.update({'UserPoolId': user_pool_id, 'ProviderName': provider_name})
    return f'{user_pool_id}/{provider_name}'


@helper.update
def update(event, _):
    new_properties = validate_properties(resource_properties(event))
    old_properties = validate_properties(old_resource_properties(event))
    if is_new_resource_inferred(new_properties, old_properties):
        return create_identity_provider(new_properties)
    update_identity_provider(new_properties)
    return physical_resource_id(event)


def is_new_resource_inferred(new_properties, old_properties):
    for attr in ['user_pool_id', 'provider_name']:
        if new_properties[attr] != old_properties[attr]:
            return True
    return False


def update_identity_provider(properties):
    details = derive_details(properties)
    idp.update_identity_provider(
        UserPoolId=properties.user_pool_id,
        ProviderName=properties.provider_name,
        ProviderDetails=details,
        AttributeMapping=properties.attribute_mapping
    )


@helper.delete
def delete(event, _):
    properties = validate_properties(resource_properties(event))
    delete_identity_provider(properties)
    return physical_resource_id(event)


def delete_identity_provider(properties):
    try:
        idp.delete_identity_provider(
            UserPoolId=properties.user_pool_id,
            ProviderName=properties.provider_name,
        )
    except idp.exceptions.ResourceNotFoundException:
        pass


def handler(event, context):
    logger.info('event: %s', event)
    helper(event, context)

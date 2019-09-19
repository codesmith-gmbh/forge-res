import logging

import boto3
from box import Box
from crhelper import CfnResource
from schema import And, Optional, Schema

from aws.common.cfn import resource_properties
from aws.common.schema import not_empty

helper = CfnResource()
logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

idp = boto3.client('cognito-idp')

properties_schema = Schema({
    'UserPoolId': And(str, not_empty, error='not empty string for UserPoolId'),
    'UserPoolClientId': And(str, not_empty, error='not empty string for UserPoolId'),

    Optional('AllowedOAuthFlowsUserPoolClient', default=False): bool,
    Optional('AllowedOAuthFlows', default=[]): [str],
    Optional('AllowedOAuthScopes', default=[]): [str],
    Optional('CallbackURLs', default=[]): [str],
    Optional('LogoutURLs', default=[]): [str],
    Optional('SupportedIdentityProviders', default=[]): [str]
})


def validate_properties(properties):
    p = Box(properties_schema.validate(properties), camel_killer_box=True)
    return p


@helper.create
@helper.update
def create(event, _):
    properties = validate_properties(resource_properties(event))
    return update_user_pool_client(properties)


def update_user_pool_client(properties):
    default_redirect_uri = ""
    if len(properties.CallbackURLs) == 0:
        default_redirect_uri = properties.CallbackURLs[0]
    client_id = properties.user_pool_client_id
    idp.update_user_pool_client(
        UserPoolId=properties.user_pool_id,
        ClientId=client_id,
        DefaultRedirectURI=default_redirect_uri,
        CallbackURLs=properties.CallbackURLs,
        LogoutURLs=properties.LogoutURLs,
        AllowedOAuthFlows=properties.AllowedOAuthFlows,
        AllowedOAuthFlowsUserPoolClient=properties.AllowedOAuthFlowsUserPoolClient,
        AllowedOAuthScopes=properties.AllowedOAuthScopes,
        SupportedIdentityProviders=properties.supported_identity_providers,
    )
    return client_id


@helper.delete
def delete(event, _):
    properties = validate_properties(resource_properties(event))
    properties.CallbackURLs = []
    properties.LogoutURLs = []
    properties.AllowedOAuthFlows = []
    properties.AllowedOAuthFlowsUserPoolClient = False
    properties.AllowedOAuthScopes = []
    properties.SupportedIdentityProviders = []

    return update_user_pool_client(properties)

import json
import logging

import boto3
from box import Box
from crhelper import CfnResource
from schema import And, Optional, Schema

import codesmith.common.naming as naming
from codesmith.common.cfn import resource_properties
from codesmith.common.schema import not_empty
from codesmith.common.ssm import put_string_parameter, silent_delete_parameter_from_event

helper = CfnResource()
logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

properties_schema = Schema({
    'UserPoolId': And(str, not_empty, error='not empty string for UserPoolId'),
    'UserPoolClientId': And(str, not_empty, error='not empty string for UserPoolClientId'),
    Optional('All', default=False): bool,
    Optional('Domains', default=[]): [str],
    Optional('Emails', default=[]): [str]
})

ssm = boto3.client('ssm')


def validate_properties(props):
    return Box(properties_schema.validate(props), camel_killer_box=True)


@helper.create
@helper.update
def create(event, _):
    p = validate_properties(resource_properties(event))
    parameter_name = naming.cog_cond_pre_auth_parameter_name(p.user_pool_id, p.user_pool_client_id)
    parameter_value = json.dumps({'All': p.all, 'Domains': p.domains, 'Emails': p.emails})
    return put_string_parameter(ssm, parameter_name,
                                value=parameter_value,
                                description='Forge Cognito Pre Auth Settings Parameter')


@helper.delete
def delete(event, _):
    return silent_delete_parameter_from_event(ssm, event)


def handler(event, context):
    logger.info('event: %s', event)
    helper(event, context)

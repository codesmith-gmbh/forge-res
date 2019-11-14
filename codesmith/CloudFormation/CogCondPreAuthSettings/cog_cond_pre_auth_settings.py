import json
import logging

import boto3
from box import Box
from crhelper import CfnResource
from schema import Optional

import codesmith.common.naming as naming
from codesmith.common.cfn import resource_properties
from codesmith.common.schema import encoded_bool, non_empty_string, tolerant_schema
from codesmith.common.ssm import put_string_parameter, silent_delete_parameter_from_event

helper = CfnResource()
logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

properties_schema = tolerant_schema({
    'UserPoolId': non_empty_string,
    'UserPoolClientId': non_empty_string,
    Optional('All', default=False): encoded_bool,
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
    put_string_parameter(ssm, parameter_name,
                         value=parameter_value,
                         description='Forge Cognito Pre Auth Settings Parameter')
    return parameter_name


@helper.delete
def delete(event, _):
    return silent_delete_parameter_from_event(ssm, event)


def handler(event, context):
    logger.info('event: %s', event)
    helper(event, context)

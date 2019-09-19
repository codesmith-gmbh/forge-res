import json
import logging

import boto3
from box import Box
from crhelper import CfnResource
from schema import And, Optional, Schema

import aws.common.naming as naming
from aws.common.cfn import physical_resource_id, resource_properties
from aws.common.schema import not_empty

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
    parameter_value = json.dumps({'All': p.all, 'Domains': p.domains, 'Emails': p.emails}),
    try:
        ssm.put_parameter(
            Name=parameter_name,
            Value=parameter_value,
            Overwrite=True,
            Type='String',
            Tier='Standard'
        )
    except ssm.exceptions.ClientError as e:
        raise RuntimeError(f'Cannot create parameter with name {parameter_name}') from e
    return parameter_name


@helper.delete
def delete(event, _):
    parameter_name = physical_resource_id(event)
    try:
        ssm.delete_parameter(Name=parameter_name)
    except ssm.exceptions.ParameterNotFound:
        pass
    return parameter_name


def handler(event, context):
    logger.info('event: %s', event)
    helper(event, context)

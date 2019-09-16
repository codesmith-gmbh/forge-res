import boto3
from botocore.exceptions import ClientError
from crhelper import CfnResource
import logging
from typing import NamedTuple

helper = CfnResource()
logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

apg = boto3.client('apigateway')
cf = boto3.client('cloudformation')


class Properties(NamedTuple):
    physical_resource_id: str
    logical_resource_id: str
    stack_id: str
    ordinal: str


def properties(event):
    try:
        p = Properties(
            physical_resource_id=event.get('PhysicalResourceId'),
            logical_resource_id=event['LogicalResourceId'],
            stack_id=event['StackId'],
            ordinal=event['ResourceProperties']['Ordinal']
        )
    except KeyError as e:
        raise ValueError('cloudformation event not valid') from e

    if not p.ordinal:
        raise ValueError('Ordinal is obligatory')

    return p


@helper.create
@helper.update
def create_update(event, _):
    p = properties(event)
    stack_id = p.stack_id
    try:
        stack = cf.describe_stacks(StackName=stack_id)
    except ClientError as e:
        raise RuntimeError(f'Cannot retrieve the stack name for {stack_id}') from e

    stack_name = stack['Stacks'][0]['StackName']
    logical_resource_id = p.logical_resource_id
    ordinal = p.ordinal
    key_name = '-'.join([stack_name, logical_resource_id, ordinal])

    try:
        key = apg.create_api_key(
            name=key_name,
            enabled=True,
        )
    except ClientError as e:
        raise RuntimeError(f'Cannot create the Api Key with name {key_name}') from e

    helper.Data.update({'Secret': key['value']})
    return key['id']


@helper.delete
def delete(event, _):
    p = properties(event)
    key_id = p.physical_resource_id
    return delete_api_key(key_id)


def delete_api_key(key_id):
    try:
        apg.delete_api_key(
            apiKey=key_id
        )
    except apg.exceptions.NotFoundException:
        pass
    except Exception as e:
        raise e

    return key_id


def handler(event, context):
    logger.info('event: %s', event)
    helper(event, context)

import logging

import boto3
from box import Box
from crhelper import CfnResource
from schema import And, Schema

from aws.common.cfn import resource_properties, physical_resource_id
from aws.common.schema import not_empty

helper = CfnResource()
logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

rds = boto3.client('rds')

properties_schema = Schema({
    'DbInstanceIdentifier': And(str, not_empty, 'not empty string for DbInstanceIdentifier')
})


def validate_properties(properties):
    return Box(properties_schema.validate(properties), camel_killer_box=True)


@helper.create
@helper.update
def create(event, _):
    properties = validate_properties(resource_properties(event))
    db_instances = rds.describe_db_instances(
        DBInstanceIdentifier=properties.db_instance_identifier
    )
    return db_instances['DBInstances'][0]['DbiResourceId']


@helper.delete
def delete(event, _):
    return physical_resource_id(event)


def handler(event, context):
    logger.info('event: %s', event)
    helper(event, context)

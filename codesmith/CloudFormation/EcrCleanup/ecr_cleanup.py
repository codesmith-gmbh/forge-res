import logging

import boto3
from box import Box
from crhelper import CfnResource
from schema import And, Schema

import codesmith.common.cfn as cfn
from codesmith.common.cfn import logical_resource_id, resource_properties
from codesmith.common.schema import not_empty

helper = CfnResource()
logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

ecr = boto3.client('ecr')
cf = boto3.client('cloudformation')

properties_schema = Schema({
    'Repository': And(str, not_empty, 'not empty string for Repository')
})


def validate_properties(properties):
    return Box(properties_schema.validate(properties), camel_killer_box=True)


@helper.create
@helper.update
def create(event, _):
    properties = validate_properties(resource_properties(event))
    return physical_resource_id(event, properties)


def physical_resource_id(event, properties):
    return '{0}:{1}'.format(logical_resource_id(event), properties.repository)


@helper.delete
def delete(event, _):
    properties = validate_properties(resource_properties(event))
    if has_valid_physical_resource_id(event, properties) and cfn.is_stack_delete_in_progress(cf, event):
        delete_all_images(properties)
    return cfn.physical_resource_id(event)


def has_valid_physical_resource_id(event, properties):
    return cfn.physical_resource_id(event) == physical_resource_id(event, properties)


def delete_all_images(properties):
    images = ecr.list_images(repositoryName=properties.repository)
    while True:
        image_ids = images['imageIds']
        next_token = images.get('nextToken')
        if len(image_ids) > 0:
            ecr.batch_delete_image(
                repositoryName=properties.repository,
                imageIds=image_ids
            )
        if next_token is None:
            return
        else:
            images = ecr.list_images(
                repositoryName=properties.repository,
                nextToken=next_token
            )


def handler(event, context):
    logger.info('event: %s', event)
    helper(event, context)

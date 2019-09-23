import logging

import boto3
from box import Box
from crhelper import CfnResource
from schema import And, Optional, Schema

import aws.common.cfn as cfn
from aws.common.cfn import logical_resource_id, resource_properties
from aws.common.schema import not_empty

helper = CfnResource()
logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

s3 = boto3.client('s3')
cf = boto3.client('cloudformation')

properties_schema = Schema({
    'Bucket': And(str, not_empty, 'not empty string for Bucket'),

    Optional('Prefix', default=''): str,
    Optional('ActiveOnlyOnStackDeletion', default=True): bool
})


def validate_properties(properties):
    return Box(properties_schema.validate(properties), camel_killer_box=True)


@helper.create
@helper.update
def create(event, _):
    properties = validate_properties(resource_properties(event))
    return physical_resource_id(event, properties)


def physical_resource_id(event, properties):
    return '{0}:{1}:{2}'.format(logical_resource_id(event), properties.bucket, properties.prefix)


@helper.delete
def delete(event, _):
    properties = validate_properties(resource_properties(event))
    if has_valid_physical_resource_id(event, properties) and should_delete(event, properties):
        delete_objects(properties)
    return cfn.physical_resource_id(event)


def has_valid_physical_resource_id(event, properties):
    return cfn.physical_resource_id(event) == physical_resource_id(event, properties)


def should_delete(event, properties):
    return not properties.active_only_on_stack_deletion or cfn.is_stack_delete_in_progress(cf, event)


def delete_objects(properties):
    out = s3.list_object_versions(
        Bucket=properties.bucket,
        Prefix=properties.prefix
    )

    while True:
        versions = out['Versions']
        if len(versions) > 0:
            objects = [{'Key': v['Key'],
                        'VersionId': v['VersionId']} for v in versions]
            s3.delete_objects(
                Bucket=properties.bucket,
                Delete={
                    'Objects': objects,
                    'Quiet': True
                }
            )

        is_truncated = out['IsTruncated']
        if is_truncated:
            out = s3.list_object_versions(
                Bucket=properties.bucket,
                Prefix=properties.prefix,
                KeyMarker=out['NextKeyMarker'],
                VersionIdMarket=out['NextVersionIdMarker']
            )
        else:
            return


def handler(event, context):
    logger.info('event: %s', event)
    helper(event, context)

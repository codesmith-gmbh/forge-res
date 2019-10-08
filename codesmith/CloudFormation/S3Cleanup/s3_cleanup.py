import logging

import boto3
from box import Box
from crhelper import CfnResource
from schema import Optional

import codesmith.common.cfn as cfn
from codesmith.common.cfn import logical_resource_id, resource_properties
from codesmith.common.schema import encoded_bool, non_empty_string, tolerant_schema

helper = CfnResource()
logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

s3 = boto3.client('s3')
cf = boto3.client('cloudformation')

properties_schema = tolerant_schema({
    'Bucket': non_empty_string,

    Optional('Prefix', default=''): str,
    Optional('ActiveOnlyOnStackDeletion', default=True): encoded_bool
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
        versions = out.get('Versions')
        if versions:
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

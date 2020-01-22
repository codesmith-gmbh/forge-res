import traceback

import boto3
import structlog
from crhelper import CfnResource

import codesmith.common.cfn as cfn
import codesmith.common.naming as naming
import codesmith.common.ssm as cssm
from codesmith.common.cfn import resource_properties, old_resource_properties
from codesmith.common.schema import box, non_empty_string, tolerant_schema, encoded_int

log = structlog.get_logger()
helper = CfnResource()
cf = boto3.client('cloudformation')
s3 = boto3.client('s3')
ssm = boto3.client('ssm')

#
# Property validation
#

properties_schema = tolerant_schema({
    'Bucket': non_empty_string,
    'CurrentReleasePrefix': non_empty_string,
    'ReleaseCountNumber': encoded_int,
})


def validate_properties(properties):
    return box(properties, schema=properties_schema)


@helper.create
def create(event, _):
    properties = validate_properties(resource_properties(event))
    parameter_name = naming.s3_release_cleanup_parameter_name(
        stack_arn=cfn.stack_id(event),
        logical_resource_id=cfn.logical_resource_id(event)
    )
    put_json_parameter(parameter_name, [properties.current_release_prefix])
    return parameter_name


def put_json_parameter(parameter_name, releases):
    cssm.put_json_parameter(ssm, parameter_name,
                            value=releases,
                            description='Durable list of past and current releases for S3ReleaseCleanup')


@helper.delete
def delete(event, _):
    return cssm.silent_delete_parameter_from_event(ssm, event)


@helper.update
def update(event, _):
    old_properties = validate_properties(old_resource_properties(event))
    properties = validate_properties(resource_properties(event))
    parameter_name = cfn.physical_resource_id(event)
    releases = cssm.fetch_json_parameter(ssm, parameter_name)
    current_release_prefix = properties.current_release_prefix
    if len(releases) > 1 \
            and releases[0] == old_properties.current_release_prefix \
            and releases[1] == current_release_prefix \
            and cfn.is_stack_rollback_complete_cleanup_in_progress(cf, event):
        # in the case of a stack rollback and swap of the 2 first releases, we just assume
        # that it is update to rollback the resource itself.
        releases.pop(0)
    else:
        # 1. we need to put the release into the list; if the release is already inside, we remove it. in any case,
        # we put the release in front of the list.
        try:
            releases.remove(current_release_prefix)
        except ValueError:
            pass
        releases.insert(0, current_release_prefix)
        # 2. we cleanup old releases if we have gone over the parameterised count
        while len(releases) > properties.release_count_number:
            old_release_prefix = releases.pop()
            delete_objects(properties.bucket, old_release_prefix)
    put_json_parameter(parameter_name, releases)
    return parameter_name


def delete_objects(bucket, prefix):
    try:
        out = s3.list_object_versions(
            Bucket=bucket,
            Prefix=prefix
        )

        while True:
            versions = out.get('Versions')
            if versions:
                objects = [{'Key': v['Key'],
                            'VersionId': v['VersionId']} for v in versions]
                s3.delete_objects(
                    Bucket=bucket,
                    Delete={
                        'Objects': objects,
                        'Quiet': True
                    }
                )

            is_truncated = out['IsTruncated']
            if is_truncated:
                out = s3.list_object_versions(
                    Bucket=bucket,
                    Prefix=prefix,
                    KeyMarker=out['NextKeyMarker'],
                    VersionIdMarket=out['NextVersionIdMarker']
                )
            else:
                return
    except Exception as e:
        print(traceback.format_exc())
        log.msg('could not properly clean the release', bucket=bucket, prefix=prefix, e=e)


#
# Handler
#

def handler(event, context):
    log.info('event', cf_event=event)
    helper(event, context)

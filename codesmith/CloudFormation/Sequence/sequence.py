import logging

import boto3
from box import Box
from crhelper import CfnResource
from schema import Optional

from codesmith.common import naming
from codesmith.common.calc import calculator, SSM_PARAMETER_DESCRIPTION
from codesmith.common.cfn import resource_properties
from codesmith.common.schema import non_empty_string, tolerant_schema
from codesmith.common.ssm import silent_delete_parameter_from_event, put_string_parameter

helper = CfnResource()
logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

ssm = boto3.client('ssm')
expression_parser = calculator(1)

properties_schema = tolerant_schema({
    'SequenceName': non_empty_string,
    Optional('Expression'): non_empty_string
})


def validate_properties(properties):
    p = properties_schema.validate(properties)
    if 'expression' not in p:
        p['expression'] = 'x'
    expression_parser.parse(p['expression'])
    return Box(p, camel_killer_box=True)


@helper.create
@helper.update
def create(event, _):
    properties = validate_properties(resource_properties(event))
    return put_sequence_parameter(properties)


def put_sequence_parameter(properties):
    parameter_name = naming.sequence_parameter_name(properties.sequence_name)
    put_string_parameter(ssm, parameter_name,
                         value=properties.expression,
                         description=SSM_PARAMETER_DESCRIPTION)
    return parameter_name


@helper.delete
def delete(event, _):
    return silent_delete_parameter_from_event(ssm, event)


def handler(event, context):
    logger.info('event: %s', event)
    helper(event, context)

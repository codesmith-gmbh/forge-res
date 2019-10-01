import logging

import boto3
from crhelper import CfnResource
from schema import Schema

import codesmith.common.cfn as cfn
from codesmith.common.cfn import logical_resource_id, resource_properties
from codesmith.common.schema import box, non_empty_string

helper = CfnResource()
logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

elb = boto3.client('elbv2')
cf = boto3.client('cloudformation')

properties_schema = Schema({
    'ListenerArn': non_empty_string(field_name='ListenerArn'),
    'Rule1Arn': non_empty_string(field_name='Rule1Arn'),
    'Rule2Arn': non_empty_string(field_name='Rule2Arn'),
    'Trigger': non_empty_string(field_name='Trigger')
})


def validate_properties(properties):
    return box(properties, schema=properties_schema)


@helper.create
def create(event, _):
    properties = validate_properties(resource_properties(event))
    return physical_resource_id(event, properties)


def physical_resource_id(event, properties):
    return '{0}:{1}'.format(logical_resource_id(event), properties.trigger)


@helper.update
def update(event, _):
    properties = validate_properties(resource_properties(event))
    swap_rules(properties)
    return physical_resource_id(event, properties)


def swap_rules(properties):
    rule1_arn = properties.rule1_arn
    rule2_arn = properties.rule2_arn
    rules = elb.describe_rules(
        ListenerArn=properties.listener_arn,
        RuleArns=[rule1_arn, rule2_arn]
    )
    priorities = rule_priorities(rules)
    elb.set_rule_priorities(
        RulePriorities=[
            {'RuleArn': rule1_arn, 'Priority': priorities[rule2_arn]},
            {'RuleArn': rule2_arn, 'Priority': priorities[rule1_arn]}
        ]
    )


def rule_priorities(rules):
    return {rule['RuleArn']: int(rule['Priority']) for rule in rules['Rules']}


@helper.delete
def delete(event, _):
    properties = validate_properties(resource_properties(event))
    if cfn.is_being_replaced(cf, event):
        swap_rules(properties)
    return cfn.physical_resource_id(event)


def handler(event, context):
    logger.info('event: %s', event)
    helper(event, context)

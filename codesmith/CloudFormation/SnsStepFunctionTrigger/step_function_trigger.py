import os

import boto3
import json
import structlog
import traceback

import codesmith.common.cfn as cfn

log = structlog.get_logger()
step = boto3.client('stepfunctions')

STATE_MACHINE_ARN = os.environ.get('STATE_MACHINE_ARN')


def handler(event, ctx):
    for record in event['Records']:
        process_record(record)


def process_record(record):
    sns = record['Sns']
    message_id = sns['MessageId']
    message = sns['Message']
    event = json.loads(message)

    try:
        start_state_machine(message_id, message)
    except step.exceptions.ExecutionAlreadyExists:
        log.msg('Execution already started, skipping', cfn_event=event, state_machine_arn=STATE_MACHINE_ARN)
    except Exception as e:
        log.msg('Error while starting the state machine', cfn_event=event, state_machine_arn=STATE_MACHINE_ARN, error=e)
        print(traceback.format_exc())
        cfn.send_failed(event, str(e))


def start_state_machine(execution_name, execution_input):
    execution = step.start_execution(
        input=execution_input,
        name=execution_name,
        stateMachineArn=STATE_MACHINE_ARN
    )
    log.msg('execution started', execution=execution)

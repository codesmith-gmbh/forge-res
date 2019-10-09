from crhelper.utils import _send_response
from crhelper.resource_helper import SUCCESS, FAILED


#
# 1. extract data
#

def stack_id(event):
    return event.get('StackId')


def logical_resource_id(event):
    return event.get('LogicalResourceId')


def physical_resource_id(event):
    return event.get('PhysicalResourceId')


def resource_properties(event):
    return event.get('ResourceProperties')


def old_resource_properties(event):
    return event.get('OldResourceProperties')


def is_stack_delete_in_progress(cf, event):
    stacks = cf.describe_stacks(
        StackName=stack_id(event),
    )
    stack_status = stacks['Stacks'][0]['StackStatus']
    return stack_status == 'DELETE_IN_PROGRESS'


#
# 2. Send success/failure to the presigned-url
#

def send_success(event):
    send_completion(event, SUCCESS)


def send_failed(event, reason):
    send_completion(event, FAILED, reason=reason)


def send_completion(event, status, *, reason='', data=None):
    response_url = event['ResponseURL']
    response_body = {
        'Status': status,
        'PhysicalResourceId': physical_resource_id(event) or 'error',
        'StackId': stack_id(event),
        'RequestId': event['RequestId'],
        'LogicalResourceId': logical_resource_id(event),
        'Reason': reason,
        'Data': data if data else {},
    }
    _send_response(response_url, response_body)


#
# 3. check if a deleted resource is being replaced.
#

def is_being_replaced(cf, event):
    lid = logical_resource_id(event)
    res = cf.describe_stack_resource(
        StackName=stack_id(event),
        LogicalResourceId=lid
    )
    return physical_resource_id(event) != res['StackResourceDetail']['PhysicalResourceId']

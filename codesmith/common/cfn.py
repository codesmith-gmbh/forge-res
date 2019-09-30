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

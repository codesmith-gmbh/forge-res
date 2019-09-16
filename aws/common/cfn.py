def physical_resource_id(event):
    return event.get('PhysicalResourceId')


def resource_properties(event):
    return event.get('ResourceProperties')

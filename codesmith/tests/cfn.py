def find_output(cf, *, stack_id, output_key):
    stacks = cf.describe_stacks(StackName=stack_id)
    outputs = stacks['Stacks'][0]['Outputs']
    for output in outputs:
        if output['OutputKey'] == output_key:
            return output['OutputValue']
    return None

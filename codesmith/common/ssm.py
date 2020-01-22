import json

from codesmith.common import cfn


def put_string_parameter(ssm, parameter_name, *, value, description):
    try:
        return ssm.put_parameter(
            Name=parameter_name,
            Description=description,
            Value=value,
            Overwrite=True,
            Type='String',
            Tier='Standard'
        )
    except ssm.exceptions.ClientError as e:
        raise RuntimeError(f'Cannot put parameter with name {parameter_name}') from e


def fetch_string_parameter(ssm, parameter_name):
    parameter = ssm.get_parameter(
        Name=parameter_name,
        WithDecryption=True
    )
    return parameter['Parameter']['Value']


def put_json_parameter(ssm, parameter_name, *, value, description):
    return put_string_parameter(ssm, parameter_name,
                                value=json.dumps(value),
                                description=description)


def fetch_json_parameter(ssm, parameter_name):
    return json.loads(fetch_string_parameter(ssm, parameter_name))


def silent_delete_parameter(ssm, parameter_name):
    try:
        ssm.delete_parameter(Name=parameter_name)
    except ssm.exceptions.ParameterNotFound:
        pass
    except ssm.exceptions.ClientError:
        pass
    return parameter_name


def silent_delete_parameter_from_event(ssm, event):
    parameter_name = cfn.physical_resource_id(event)
    return silent_delete_parameter(ssm, parameter_name)

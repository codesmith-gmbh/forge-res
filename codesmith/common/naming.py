# SSM Parameters
#
# Some of the custom CloudFormation resources create by the forge are implemented
# as SSM parameters in the parameter store.
SSM_PARAMETER_PREFIX = '/codesmith-forge'


# CogCondPreAuth naming
def cog_cond_pre_auth_parameter_name(user_pool_id: str, user_pool_client_id: str) -> str:
    return '{0}/CogCondPreAuth/{1}/{2}'.format(SSM_PARAMETER_PREFIX, user_pool_id, user_pool_client_id)


# Sequence naming
def sequence_parameter_name(sequence_name: str) -> str:
    return '{0}/Sequence{1}'.format(SSM_PARAMETER_PREFIX, sequence_name)

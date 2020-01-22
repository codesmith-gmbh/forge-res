import re

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


# Dns Certificate SNS Message memory
def dns_certificate_sns_message_id_parameter_name(stack_arn, logical_resource_id):
    stack_id = extract_stack_id(stack_arn)
    return '{0}/DnsCertificateSnsMessageId/{1}/{2}'.format(SSM_PARAMETER_PREFIX, stack_id, logical_resource_id)


# S3ReleaseCleanup store
def s3_release_cleanup_parameter_name(stack_arn, logical_resource_id):
    stack_id = extract_stack_id(stack_arn)
    return '{0}/S3ReleaseCleanup/{1}/{2}'.format(SSM_PARAMETER_PREFIX, stack_id, logical_resource_id)


STACK_ID_REGEX = re.compile('^arn:.*:cloudformation:.*:.*:stack/(.*)')


def extract_stack_id(stack_arn):
    return STACK_ID_REGEX.match(stack_arn).group(1)

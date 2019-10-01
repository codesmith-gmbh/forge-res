import re
import codesmith.common.cfn as cfn

REGION_EXTRACTOR = re.compile('arn:aws:(?:.*?):(.*?):')


def extract_region(arn):
    return REGION_EXTRACTOR.match(arn).group(1)


def is_same_region(event, region1, region2):
    # 1. if the new region is the same as the old one, they are the same.
    if region1 == region2:
        return True

    # 2. else, if both are defined, they are not the same.
    if region1 and region2:
        return False

    # 3. else, we have a complicate case where either the old or the new region are implicit from the
    # region of the cloudformation stack.
    sdk_region = extract_region(cfn.stack_id(event))
    return sdk_region == region1 or sdk_region == region2


CERTIFICATE_ARN_REGEX = re.compile('^arn:aws.*:acm:.*certificate/.*')


def is_certificate_arn(arn):
    return bool(CERTIFICATE_ARN_REGEX.match(arn))

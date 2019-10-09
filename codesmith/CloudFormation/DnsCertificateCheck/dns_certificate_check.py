import re

import boto3
import structlog

import codesmith.common.cfn as cfn

MAX_ROUND_COUNT = 60

log = structlog.get_logger()


def handler(event, _):
    log.msg('', sf_event=event)
    round_f = event.get("Round")
    round_index = int(round_f) if round_f else 0
    check = check_certificate(event, round_index)
    event["IsCertificateIssued"] = check
    event["Round"] = round_index + 1
    return event


def check_certificate(event, round_index):
    try:
        if event['RequestType'] == 'Delete':
            cfn.send_success(event)
            return True
        certificate_arn = cfn.physical_resource_id(event)
        if round_index >= MAX_ROUND_COUNT:
            cfn.send_failed(event, "certificate {} did not stablise".format(certificate_arn))
            return True
        acm = acm_service(certificate_arn)
        certificate = acm.describe_certificate(CertificateArn=certificate_arn)
        certificate_status = certificate['Certificate']['Status']
        if certificate_status == 'ISSUED':
            cfn.send_success(event)
            return True
        elif certificate_status == 'PENDING_VALIDATION':
            return False
        else:
            cfn.send_failed(event,
                            'the certificate {} is in invalid status {}.'.format(certificate_arn, certificate_status))
            return True
    except Exception as e:
        cfn.send_failed(event, "exception during checking: {}".format(str(e)))
        return True


CERTIFICATE_REGION_REGEX = re.compile('^arn:aws.*:acm:(.+?):')


def certificate_region(certificate_arn):
    return CERTIFICATE_REGION_REGEX.match(certificate_arn).group(1)


def acm_service(certificate_arn):
    region = certificate_region(certificate_arn)
    return boto3.client('acm', region_name=region)

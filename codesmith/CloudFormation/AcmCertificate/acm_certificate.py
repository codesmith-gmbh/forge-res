import time
import traceback

import boto3
import structlog
from schema import Optional

import codesmith.common.cfn as cfn
import codesmith.common.common as c
from codesmith.common.cfn import resource_properties
from codesmith.common.schema import box, non_empty_string, tolerant_schema

MAX_ROUND_COUNT = 60

log = structlog.get_logger()

#
# 1. Property validation
#

properties_schema = tolerant_schema({
    'DomainName': non_empty_string,
    Optional('Region'): non_empty_string,
    Optional('SubjectAlternativeNames', default=[]): [str],
    Optional('Tags', default=[]): {'Key': non_empty_string,
                                   'Value': non_empty_string}
})


def validate_properties(properties):
    p = box(properties, schema=properties_schema)
    return p


#
# 2. Process event
#

def handler(event, _):
    log.msg('', sf_event=event)
    round_f = event.get("Round")
    round_index = int(round_f) if round_f else 0
    if round_index >= MAX_ROUND_COUNT:
        cfn.send_failed(event, "process with event {} did not stabilise".format(event))
        is_done = True
    else:
        is_done = process_event(event)
    event["IsDone"] = is_done
    event["Round"] = round_index + 1
    return event


def process_event(event):
    event_type = event['RequestType']
    try:
        if event_type == 'Create':
            try:
                create_certificate(event)
                return True
            except CertificateError as e:
                event['PhysicalResourceId'] = e.certificate_arn
                raise e
        elif event_type == 'Update':
            update_certificate(event)
            return True
        elif event_type == 'Delete':
            return delete_certificate(event)
        else:
            raise ValueError('Unknown event type: {}'.format(event_type))
    except Exception as e:
        log.msg('Error while processing the event', cfn_event=event, error=e)
        print(traceback.format_exc())
        cfn.send_failed(event, str(e))
        return True


#
# 3.1 Delete the certificate
#
def delete_certificate(event):
    certificate_arn = cfn.physical_resource_id(event)
    log.msg('deleting certificate', certificate_arn=certificate_arn)
    # We delete only if the certificate has been properly created before.
    if c.is_certificate_arn(certificate_arn):
        properties = validate_properties(resource_properties(event))
        cert_proc = CertificateProcessor(certificate_arn, properties)
        try:
            cert_proc.delete_certificate()
            is_deleted = True
        except cert_proc.acm.exceptions.ResourceInUseException:
            log.msg('Certificate in use; waiting and retrying', certificate_arn=certificate_arn)
            is_deleted = False
    else:
        is_deleted = True
    if is_deleted:
        cfn.send_success(event)
    return is_deleted


#
# 3.2 Create the certificate
#

class CertificateError(RuntimeError):
    def __init__(self, certificate_arn):
        self.certificate_arn = certificate_arn


def create_certificate(event):
    properties = validate_properties(resource_properties(event))
    cert_proc = CertificateProcessor(None, properties)
    event['PhysicalResourceId'] = cert_proc.create_certificate()
    cert_proc.describe_certificate()
    cfn.send_success(event)


#
# 3.3 Update the certificate
#
def update_certificate(event):
    new_properties = validate_properties(resource_properties(event))
    old_properties = validate_properties(cfn.old_resource_properties(event))
    certificate_arn = cfn.physical_resource_id(event)
    if needs_new(event, old_properties, new_properties):
        log.msg('new certificate needed',
                stack_arn=cfn.stack_id(event),
                logical_resource_id=cfn.logical_resource_id(event))
        create_certificate(event)
    else:
        cert_proc = CertificateProcessor(certificate_arn, new_properties)
        if safe_set(old_properties.tags) != safe_set(new_properties.tags):
            cert_proc.update_tags()
        cfn.send_success(event)


def needs_new(event, old, new):
    return (
            old.domain_name != new.domain_name or
            not c.is_same_region(event, old.region, new.region) or
            safe_set(new.subject_alternative_names) != safe_set(old.subject_alternative_names) or
            old.hosted_zone_id != new.hosted_zone_id
    )


#
# 3.4  Certificate processor
#
def safe_set(coll):
    return set(coll if coll else [])


class CertificateProcessor:
    def __init__(self, certificate_arn, properties):
        self.certificate_arn = certificate_arn
        self.properties = properties
        self.acm = self.acm_service()

    def create_certificate(self):
        args = {
            'DomainName': self.properties['DomainName'],
            'ValidationMethod': 'DNS',
            'Options':
                {
                    'CertificateTransparencyLoggingPreference': 'ENABLED'
                }
        }
        if self.properties.subject_alternative_names:
            args['SubjectAlternativeNames'] = self.properties.subject_alternative_names
        certificate = self.acm.request_certificate(**args)
        self.certificate_arn = certificate['CertificateArn']
        if self.properties.tags:
            self.acm.add_tags_to_certificate(
                CertificateArn=self.certificate_arn,
                Tags=self.properties.tags
            )
        return self.certificate_arn

    # Waiting for the data for the CNAME records requires a loop and waiting
    # since those are created by AWS asynchronously and added to the
    # certificate information only when they have been properly created. We
    # wait at most 3 minutes with 3 seconds interval.
    def describe_certificate(self):
        for i in range(0, 60):
            out = self.acm.describe_certificate(CertificateArn=self.certificate_arn)
            certificate = out['Certificate']
            log.msg("describe certificate", i=i, certificate=certificate)
            options = certificate.get('DomainValidationOptions')
            if options:
                if len(options) == len(self.properties.subject_alternative_names) + 1:
                    options_without_resource_record = [r for r in options if r.get('ResourceRecord') is None]
                    if not options_without_resource_record:
                        return certificate
            log.msg('DomainValidationOptions for certificate not complete', certificate=certificate)
            time.sleep(3.0)
        raise RuntimeError("no DNS entries for certificate {}".format(self.certificate_arn))

    def delete_certificate(self):
        try:
            self.acm.delete_certificate(CertificateArn=self.certificate_arn)
        except self.acm.exceptions.ResourceNotFoundException:
            log.msg('certificate does not exists', certificate_arn=self.certificate_arn)

    def update_tags(self):
        tags = self.acm.list_tags_for_certificate(CertificateArn=self.certificate_arn)
        if tags['Tags']:
            self.acm.remove_tags_from_certificate(
                CertificateArn=self.certificate_arn,
                Tags=tags['Tags']
            )
        if self.properties.tags:
            self.acm.add_tags_to_certificate(
                CertificateArn=self.certificate_arn,
                Tags=self.properties.tags
            )

    def acm_service(self):
        manual_region = self.properties.region
        if manual_region:
            return boto3.client('acm', region_name=manual_region)
        else:
            return boto3.client('acm')

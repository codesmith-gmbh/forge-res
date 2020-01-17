import json
import os
import time
import traceback

import boto3
import structlog
from schema import Optional

import codesmith.common.cfn as cfn
import codesmith.common.common as c
import codesmith.common.naming as naming
import codesmith.common.ssm as cssm
from codesmith.common.cfn import resource_properties
from codesmith.common.schema import box, encoded_bool, non_empty_string, tolerant_schema

STATE_MACHINE_ARN = os.environ.get('STATE_MACHINE_ARN')

log = structlog.get_logger()
cf = boto3.client('cloudformation')
r53 = boto3.client('route53')
ssm = boto3.client('ssm')
step = boto3.client('stepfunctions')

#
# 1. Property validation
#

properties_schema = tolerant_schema({
    'DomainName': non_empty_string,
    Optional('Region'): non_empty_string,
    Optional('SubjectAlternativeNames', default=[]): [str],
    Optional('Tags', default=[]): {'Key': non_empty_string,
                                   'Value': non_empty_string},
    Optional('HostedZoneName'): non_empty_string,
    Optional('HostedZoneId'): non_empty_string,
    Optional('WithCaaRecords', default=True): encoded_bool
})


def validate_properties(properties):
    p = box(properties, schema=properties_schema)
    hosted_zone_name = p.get('HostedZoneName')
    hosted_zone_id = p.get('HostedZoneId')
    if (not hosted_zone_name) and (not hosted_zone_id):
        raise ValueError('one of HostedZoneName or HostedZoneId must be defined')
    if hosted_zone_name and hosted_zone_id:
        raise ValueError('only of HostedZoneName or HostedZoneId may be defined')
    p = complete_hosted_zone_data(p)
    check_domains_in_hosted_zone(p)
    return p


def complete_hosted_zone_data(properties):
    hosted_zone_name = properties.get('HostedZoneName')
    if hosted_zone_name:
        hosted_zone = r53.list_hosted_zones_by_name(
            DNSName=hosted_zone_name
        )
        properties.hosted_zone_id = hosted_zone['HostedZones'][0]['Id']
    else:
        hosted_zone = r53.get_hosted_zone(
            Id=properties.hosted_zone_id
        )
        properties.hosted_zone_name = hosted_zone['HostedZone']['Name']
    return properties


def check_domains_in_hosted_zone(properties):
    hosted_zone_name = properties.hosted_zone_name
    check_subdomain(properties.domain_name, hosted_zone_name)
    for domain in properties.subject_alternative_names:
        check_subdomain(domain, hosted_zone_name)


def check_subdomain(subdomain, domain):
    if not is_subdomain(subdomain, domain):
        raise ValueError("{} not subdomain of {}".format(subdomain, domain))


def is_subdomain(subdomain, domain):
    return normalize_domain(subdomain).endswith(normalize_domain(domain))


def normalize_domain(domain_name):
    if domain_name.endswith('.'):
        return domain_name
    else:
        return domain_name + '.'


def safe_set(coll):
    return set(coll if coll else [])


#
# 2. Decode and process SNS events together with the main handler
#

def handler(event, ctx):
    for record in event['Records']:
        process_record(record)


def process_record(record):
    sns = record['Sns']
    message_id = sns['MessageId']
    event = json.loads(sns['Message'])
    event_type = event['RequestType']

    try:
        if event_type == 'Create':
            try:
                create_certificate(message_id, event)
            except CertificateError as e:
                event['PhysicalResourceId'] = e.certificate_arn
                raise e
        elif event_type == 'Update':
            update_certificate(message_id, event)
        elif event_type == 'Delete':
            delete_certificate(event)
        else:
            raise ValueError('Unknown event type: {}'.format(event_type))
    except Exception as e:
        log.msg('Error while processing the event', cfn_event=event, error=e)
        print(traceback.format_exc())
        cfn.send_failed(event, str(e))


#
# 3.1 Delete the certificate
#
def delete_certificate(event):
    certificate_arn = cfn.physical_resource_id(event)
    log.msg('deleting certificate', certificate_arn=certificate_arn)
    # We delete only if the certificate has been properly created before.
    if c.is_certificate_arn(certificate_arn):
        properties = validate_properties(resource_properties(event))
        cert_proc = CertificateProcessor(certificate_arn=certificate_arn, properties=properties)
        if not cfn.is_being_replaced(cf, event):
            # This resource may create DNS Records that should be deleted as well. Also there is an SNS parameter
            # to ensure that the resource is created only once (to handle possible SNS delivery duplication)
            # these are deleted only if the resource is being deleted (the update function takes care of the DNS
            # records if the resource is being replaced).
            cert_proc.delete_record_set_group()
            delete_sns_message_ssm_parameter(event)
        cert_proc.delete_certificate()
    cfn.send_success(event)


#
# 3.2 Create the certificate
#

class CertificateError(RuntimeError):
    def __init__(self, certificate_arn):
        self.certificate_arn = certificate_arn


def create_certificate(sns_message_id, event):
    if should_skip_message(sns_message_id, event):
        return
    properties = validate_properties(resource_properties(event))
    cert_proc = CertificateProcessor(None, properties)
    certificate_arn = cert_proc.create_certificate()
    try:
        cert_proc.create_record_set_group()
        start_wait_state_machine(certificate_arn, sns_message_id, event)
    except Exception as e:
        raise CertificateError(certificate_arn) from e


#
# 3.3 Update the certificate
#
def update_certificate(sns_message_id, event):
    if should_skip_message(sns_message_id, event):
        return
    new_properties = validate_properties(resource_properties(event))
    old_properties = validate_properties(cfn.old_resource_properties(event))
    certificate_arn = cfn.physical_resource_id(event)
    if needs_new(event, old_properties, new_properties):
        log.msg('new certificate needed',
                stack_arn=cfn.stack_id(event),
                logical_resource_id=cfn.logical_resource_id(event))
        log.msg('delete old dns record', old_properties=old_properties)
        cert_proc = CertificateProcessor(certificate_arn, old_properties)
        cert_proc.delete_record_set_group()
        create_certificate(sns_message_id, event)
    else:
        cert_proc = CertificateProcessor(certificate_arn, new_properties)
        if safe_set(old_properties.tags) != safe_set(new_properties.tags):
            cert_proc.update_tags()
        if old_properties.with_caa != new_properties.with_caa:
            if new_properties.with_caa:
                cert_proc.create_caa_records()
            else:
                cert_proc.delete_caa_records()
        cfn.send_success(event)


def needs_new(event, old, new):
    return (
            old.domain_name != new.domain_name or
            not c.is_same_region(event, old.region, new.region) or
            safe_set(new.subject_alternative_names) != safe_set(old.subject_alternative_names) or
            old.hosted_zone_id != new.hosted_zone_id
    )


#
# 4. DNS Records manipulation
#

class GenerationSpec:
    def __init__(self, with_cname, with_caa):
        self.with_cname = with_cname
        self.with_caa = with_caa


CAA_GENERATION_SPEC = GenerationSpec(with_cname=False, with_caa=True)


def generation_spec(properties):
    return GenerationSpec(with_cname=True, with_caa=properties.with_caa_records)


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

    COMMENT = "by Codesmith Forge DnsCertificateRecordSetGroup custom resource"

    def create_record_set_group(self):
        log.msg('Creating DNS records for the Certificate', CertificateArn=self.certificate_arn)
        changes = self.generate_create_record_set_group_changes()
        self.execute_dns_records_batch(changes)

    def generate_create_record_set_group_changes(self):
        return self.generate_record_set_group_changes('CREATE', generation_spec(self.properties))

    def delete_record_set_group(self):
        changes = self.generate_delete_record_set_group_changes()
        self.execute_delete_batch(changes)

    def generate_delete_record_set_group_changes(self):
        return self.generate_record_set_group_changes('DELETE', generation_spec(self.properties))

    def create_caa_records(self):
        changes = self.generate_create_caa_records_changes()
        self.execute_dns_records_batch(changes)

    def generate_create_caa_records_changes(self):
        return self.generate_record_set_group_changes('CREATE', CAA_GENERATION_SPEC)

    def delete_caa_records(self):
        changes = self.generate_delete_caa_records_changes()
        self.execute_delete_batch(changes)

    def generate_delete_caa_records_changes(self):
        return self.generate_record_set_group_changes('DELETE', CAA_GENERATION_SPEC)

    def generate_record_set_group_changes(self, change_action, spec):
        certificate = self.describe_certificate()
        changes = []
        for option in certificate['DomainValidationOptions']:
            if spec.with_cname:
                changes.append(cname_record_change(option, change_action))
            if spec.with_caa:
                changes.append(caa_record_change(option, change_action))
        return changes

    def execute_dns_records_batch(self, changes):
        change_info = r53.change_resource_record_sets(
            HostedZoneId=self.properties.hosted_zone_id,
            ChangeBatch={
                'Comment': self.COMMENT,
                'Changes': changes
            }
        )
        self.wait_for_dns_records_batch(change_info['ChangeInfo'])

    @staticmethod
    def wait_for_dns_records_batch(change_info):
        change_id = change_info['Id']
        change_status = change_info['Status']
        for i in range(0, 60):
            if change_status == 'INSYNC':
                return
            time.sleep(3.0)
            change_info = r53.get_change(Id=change_id)['ChangeInfo']
            change_status = change_info['Status']
        raise RuntimeError('change {} did not sync in time'.format(change_id))

    def acm_service(self):
        manual_region = self.properties.region
        if manual_region:
            return boto3.client('acm', region_name=manual_region)
        else:
            return boto3.client('acm')

    def execute_delete_batch(self, changes):
        try:
            self.execute_dns_records_batch(changes)
        except Exception as e:
            log.msg("could not delete the records in batch, deleting one by one", error=str(e))
            self.execute_delete_changes(changes)

    def execute_delete_changes(self, changes):
        for change in changes:
            try:
                change_info = r53.change_resource_record_sets(
                    HostedZoneId=self.properties.hosted_zone_id,
                    ChangeBatch={
                        'Comment': self.COMMENT,
                        'Changes': [change]
                    }
                )['ChangeInfo']
                self.wait_for_dns_records_batch(change_info)
            except r53.exceptions.InvalidChangeBatch as e:
                if e.response['Error']['Message'].startswith('[Tried to delete resource record set'):
                    log.msg("non existant record, skipping", change=change)
                else:
                    raise e


def cname_record_change(option, change_action):
    resource_record = option['ResourceRecord']
    return {'Action': change_action,
            'ResourceRecordSet': {
                'Name': resource_record['Name'],
                'ResourceRecords': [{'Value': resource_record['Value']}],
                'Type': resource_record['Type'],
                'TTL': 300
            }}


def caa_record_change(option, change_action):
    caa_name = option['DomainName'] + "."
    return {'Action': change_action,
            'ResourceRecordSet': {
                'Name': caa_name,
                'ResourceRecords': [{'Value': '0 issue "amazon.com"'}],
                'Type': 'CAA',
                'TTL': 300
            }}


#
# 5. SSM Parameter manipulation
#

SSM_PARAMETER_DESCRIPTION = 'Codesmith Forge: SNS skip detection for DnsCertificate'


# Important: the following code works only because the ReservedConcurrentExecutions of the the DnsCertificate lamdba
# function it set to 1 and so all SNS events are serialized.
def should_skip_message(sns_message_id, event):
    stack_arn = cfn.stack_id(event)
    logical_resource_id = cfn.logical_resource_id(event)
    log.msg('checking sns message skipping',
            stack_arn=stack_arn,
            logical_resource_id=logical_resource_id,
            sns_message_id=sns_message_id)
    parameter_name = naming.dns_certificate_sns_message_id_parameter_name(stack_arn, logical_resource_id)
    last_message_id = None
    try:
        last_message_id = cssm.fetch_string_parameter(ssm, parameter_name)
    except ssm.exceptions.ParameterNotFound:
        pass
    except Exception as e:
        raise e
    if last_message_id == sns_message_id:
        log.msg('sns message already processed', sns_message_id=sns_message_id)
        return True
    else:
        cssm.put_string_parameter(ssm, parameter_name, value=sns_message_id, description=SSM_PARAMETER_DESCRIPTION)
        return False


def delete_sns_message_ssm_parameter(event):
    stack_arn = cfn.stack_id(event)
    logical_resource_id = cfn.logical_resource_id(event)
    parameter_name = naming.dns_certificate_sns_message_id_parameter_name(stack_arn, logical_resource_id)
    try:
        ssm.delete_parameter(Name=parameter_name)
    except ssm.exceptions.ParameterNotFound:
        log.msg('parameter does not exists, skipping', parameter_name=parameter_name)
    except Exception as e:
        raise e


#
# 6. Wait state machine
#
def start_wait_state_machine(certificate_arn, sns_message_id, event):
    event['PhysicalResourceId'] = certificate_arn
    event['ResourceProperties']['CertificateArn'] = certificate_arn
    event_text = json.dumps(event)
    execution = step.start_execution(
        input=event_text,
        name=sns_message_id,
        stateMachineArn=STATE_MACHINE_ARN
    )
    log.msg('execution started', execution=execution)

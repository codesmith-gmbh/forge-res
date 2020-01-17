import time

import boto3
import structlog
from crhelper import CfnResource
from schema import Optional

import codesmith.common.cfn as cfn
from codesmith.common.cfn import resource_properties, old_resource_properties
from codesmith.common.common import extract_region
from codesmith.common.schema import box, encoded_bool, non_empty_string, tolerant_schema

log = structlog.get_logger()
helper = CfnResource()
r53 = boto3.client('route53')

#
# Property validation
#

properties_schema = tolerant_schema({
    'CertificateArn': non_empty_string,
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


#
# Certificate Processor
#

class GenerationSpec:
    def __init__(self, with_cname, with_caa):
        self.with_cname = with_cname
        self.with_caa = with_caa


CAA_GENERATION_SPEC = GenerationSpec(with_cname=False, with_caa=True)


def generation_spec(properties):
    return GenerationSpec(with_cname=True, with_caa=properties.with_caa_records)


class CertificateProcessor:
    def __init__(self, properties):
        self.properties = properties
        self.acm = self.acm_service()
        self.certificate = self.describe_certificate()
        self.check_domains_in_hosted_zone()

    def check_domains_in_hosted_zone(self):
        hosted_zone_name = self.properties.hosted_zone_name
        check_subdomain(self.certificate['DomainName'], hosted_zone_name)
        for domain in self.certificate['SubjectAlternativeNames']:
            check_subdomain(domain, hosted_zone_name)

    @property
    def certificate_arn(self):
        return self.properties.certificate_arn

    def describe_certificate(self):
        out = self.acm.describe_certificate(CertificateArn=self.certificate_arn)
        return out['Certificate']

    COMMENT = "by Codesmith Forge DnsCertificateRecordSetGroup custom resource"

    def create_record_set_group(self):
        log.msg('Creating DNS records for the Certificate', CertificateArn=self.certificate_arn)
        changes = self.generate_create_record_set_group_changes()
        self.execute_dns_records_batch(changes)

    def generate_create_record_set_group_changes(self):
        return self.generate_record_set_group_changes('UPSERT', generation_spec(self.properties))

    def delete_record_set_group(self):
        changes = self.generate_delete_record_set_group_changes()
        self.execute_delete_batch(changes)

    def generate_delete_record_set_group_changes(self):
        return self.generate_record_set_group_changes('DELETE', generation_spec(self.properties))

    def generate_record_set_group_changes(self, change_action, spec):
        changes = []
        for option in self.certificate['DomainValidationOptions']:
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
        manual_region = extract_region(self.certificate_arn)
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


# Since we can always update the Route53 Records, it is never necessary to create a new resource while updating
# and the physical id for the resource is irrelevant; we simply use the logical resource id.
def physical_resource_id(event):
    return cfn.logical_resource_id(event)


#
# Create
#


def create_from_event(event, properties_fn):
    properties = validate_properties(properties_fn(event))
    cert_proc = CertificateProcessor(properties)
    cert_proc.create_record_set_group()


@helper.create
def create(event, _):
    create_from_event(event, resource_properties)
    return physical_resource_id(event)


#
# Delete
#

def delete_from_event(event, properties_fn):
    try:
        properties = validate_properties(properties_fn(event))
    except Exception as e:
        log.msg("invalid properties, skipping deletion", e=e, cfn_event=event)
        return
    cert_proc = CertificateProcessor(properties)
    cert_proc.delete_record_set_group()


@helper.delete
def delete(event, _):
    delete_from_event(event, resource_properties)
    return physical_resource_id(event)


#
# Update -> Delete and Create
#
@helper.update
def update(event, _):
    delete_from_event(event, old_resource_properties)
    create_from_event(event, resource_properties)
    return physical_resource_id(event)


#
# Handler
#

def handler(event, context):
    log.info('event', cf_event=event)
    helper(event, context)

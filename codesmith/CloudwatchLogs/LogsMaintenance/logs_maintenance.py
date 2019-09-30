import json
import logging
import os

import boto3

SNS_ALERT_TOPIC_ARN = os.environ.get('SNS_ALERT_TOPIC_ARN')
DEFAULT_RETENTION_IN_DAYS = 90

logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

sns = boto3.client('sns')
ec2 = boto3.client('ec2')


def handler(event, _):
    logger.info('event %s', event)
    regions = fetch_regions()
    problems = []

    for region in regions:
        logger.info("Region maintenance: %s", region)
        problems.extend(check_log_groups_expiration(region))

    if len(problems) > 0:
        alert(problems)


def fetch_regions():
    regions = ec2.describe_regions()
    return [region['RegionName'] for region in regions['Regions']]


def check_log_groups_expiration(region):
    problems = []
    logs = boto3.client('logs', region_name=region)
    try:
        grps = logs.describe_log_groups()
    except logs.exceptions.ClientError:
        return problems
    while True:
        for grp in grps['logGroups']:
            retention_in_days = grp.get('retentionInDays')
            if retention_in_days is None or retention_in_days != DEFAULT_RETENTION_IN_DAYS:
                log_group_name = grp['logGroupName']
                try:
                    logs.put_retention_policy(
                        logGroupName=log_group_name,
                        retentionInDays=DEFAULT_RETENTION_IN_DAYS
                    )
                except logs.exceptions.ClientError as e:
                    problems.append({'logGroupName': log_group_name, 'error': str(e)})

        next_token = grps.get('nextToken')
        if next_token is None:
            break
        else:
            try:
                grps = logs.describe_log_groups(nextToken=next_token)
            except logs.exceptions.ClientError:
                return problems
    return problems


def alert(problems):
    subject = "CloudwatchLogs expiration checker and gc has {} problems".format(len(problems))
    msg_text = json.dumps(problems)
    sns.publish(
        TargetArn=SNS_ALERT_TOPIC_ARN,
        Subject=subject,
        Message=msg_text
    )

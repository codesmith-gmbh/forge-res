import json
import logging

import boto3
from box import Box
from schema import Optional, Schema

import codesmith.common.naming as naming
from codesmith.common.ssm import fetch_string_parameter

logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

ssm = boto3.client('ssm')

settings_schema = Schema({
    Optional('All', default=False): bool,
    Optional('Domains', default=[]): [str],
    Optional('Emails', default=[]): [str]
})


def process_event(event, _):
    # 1. we fetch the Settings for the given user pool
    user_pool_id = event['userPoolId']
    client_id = event['callerContext']['clientId']
    settings = fetch_settings(user_pool_id, client_id)

    # 2. we get the email and the domain name from the event.
    email, domain = email_and_domain_of_user(event)

    # 3. To a accept an authentication request, one of the following condition must be true:
    #    a. The `All` flag is set to true
    #    b. The domain name of the user is contained in the list of whitelisted domain names.
    #    c. The email of the user is contained in the list of whitelisted domain names.
    if not (settings.all or domain in settings.domain or email in settings.email):
        raise ValueError('')

    return event


def fetch_settings(user_pool_id, client_id):
    parameter_name = naming.cog_cond_pre_auth_parameter_name(user_pool_id, client_id)
    settings_text = fetch_string_parameter(ssm, parameter_name=parameter_name)
    p = json.loads(settings_text)
    return Box(settings_schema.validate(p), camel_killer_box=True)


def email_and_domain_of_user(event):
    email = event['request']['userAttributes']['email']
    _, domain = email.split("@", 1)
    return email, domain

import io
import json
import logging
import os
import zipfile

import boto3
from box import Box
from schema import And, Optional, Schema

from codesmith.common.schema import not_empty

logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)

s3 = boto3.client('s3')

properties_schema = Schema({
    'Pipeline': And(str, not_empty, error='not empty string for Pipeline'),
    Optional('OnTag', default=False): bool,
    Optional('OnCommit', default=False): bool
})


def load_properties_from_json(json_text):
    p = json.loads(json_text)
    return Box(p, camel_killer_box=True)


EVENTS_BUCKET_NAME = 'EVENTS_BUCKET_NAME'
TAG_REF_PREFIX = 'refs/tags/'
BUILD_SPEC = '''version: 0.2

phases:
  build:
    commands:
      - bash checkout.sh
artifacts:
  type: zip
  files:
    - "**/*"
  base-directory: repo
'''

CHECKOUT_SH = '''#!/bin/bash

git config --global credential.helper '!aws codecommit credential-helper $@'
git config --global credential.UseHttpPath true

git clone --shallow-submodules https://git-codecommit.{}.amazonaws.com/v1/repos/{} repo
cd repo
{}
cd
'''


def process_event(event, _):
    logger.info('received CodeCommit event', event)
    for commit in event['Records']:
        t = derive_trigger(commit)
        t.trigger()
    return event


def derive_trigger(commit):
    custom_data = commit['customData']
    properties = load_properties_from_json(custom_data)
    aws_region = commit['awsRegion']
    repository = extract_repository_name(commit['eventSourceARN'])
    reference = commit['codecommit']['references'][0]
    logger.info('repository data: %s %s %s', repository, reference, aws_region)
    if is_commit(reference) and properties.on_commit:
        checkout_command = 'git checkout ' + reference['commit']
    elif is_tag(reference) and properties.on_tag:
        checkout_command = 'git checkout ' + extract_tag(reference)
    else:
        return NOPTrigger()

    return PipelineTrigger(
        aws_region=aws_region,
        repository=repository,
        pipeline=properties.pipeline,
        checkout_command=checkout_command
    )


def extract_repository_name(event_source_arn):
    return event_source_arn[event_source_arn.rfind(':') + 1:]


def is_commit(reference):
    return reference['ref'].startswith('refs/heads/')


def is_tag(reference):
    return reference['ref'].startswith(TAG_REF_PREFIX)


def extract_tag(reference):
    return reference['ref'][len(TAG_REF_PREFIX):]


class PipelineTrigger:
    s3_bucket = os.environ.get(EVENTS_BUCKET_NAME)

    def __init__(self, *, aws_region, repository, pipeline, checkout_command):
        self.aws_region = aws_region
        self.repository = repository
        self.pipeline = pipeline
        self.checkout_command = checkout_command

    def trigger(self):
        s3.put_object(
            Key=self.pipeline + '/trigger.zip',
            Bucket=PipelineTrigger.s3_bucket,
            Body=self.generate_zip_file()
        )

    def generate_files(self):
        return {
            'buildspec.yaml': BUILD_SPEC,
            'chechkout.sh': CHECKOUT_SH.format(self.aws_region, self.repository, self.checkout_command)
        }

    def generate_zip_file(self):
        buf = io.BytesIO()
        with zipfile.ZipFile(buf, 'a') as zf:
            for file, content in self.generate_files().items():
                zf.writestr(file, content)
        return buf.getbuffer()


class NOPTrigger:
    def trigger(self):
        logger.info('no trigger')

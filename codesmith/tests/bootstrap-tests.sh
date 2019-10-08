#!/usr/bin/env bash

set -eu -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null && pwd)"
AWS_REGION=us-east-1

echo "Bootstrap tests in region: ${AWS_REGION}"

aws --region ${AWS_REGION} cloudformation deploy \
  --template-file templates/ForgeTestBucket.yaml \
  --stack-name ForgeTestBucket \
  --no-fail-on-empty-changeset

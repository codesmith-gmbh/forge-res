#!/usr/bin/env bash

set -eo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"
VERSION=$(git describe --match "v*" --dirty=--DIRTY-- --always | sed 's:^v\(.*\)$:\1:')

echo "Bootstrap in region: ${DEFAULT_AWS_REGION}"
echo "Version: ${VERSION}"
echo "ForgeDomainName: ${FORGE_DOMAIN_NAME}"

CLOUDFORMATION_ROLE_ARN=$(aws cloudformation describe-stacks --stack-name ForgeBootstrap | jq -r '.Stacks[0].Outputs | map(select(.OutputKey=="BootstrapCloudFormationRoleArn"))[0].OutputValue')

cd ${SCRIPT_DIR}

if [[ "${DEFAULT_AWS_REGION}" != "us-east-1" ]]; then
    echo "Deploying the stack ForgeIam on the us-east-1 region"

    aws --region us-east-1 cloudformation deploy \
        --template-file templates/ForgeIam.yaml \
        --stack-name ForgeIam \
        --capabilities CAPABILITY_NAMED_IAM \
        --role-arn ${CLOUDFORMATION_ROLE_ARN} \
        --no-fail-on-empty-changeset
fi

echo "Deploying the stack ForgeIam on the ${DEFAULT_AWS_REGION} region"

aws --region ${DEFAULT_AWS_REGION} cloudformation deploy \
    --template-file templates/ForgeIam.yaml \
    --stack-name ForgeIam \
    --capabilities CAPABILITY_NAMED_IAM \
    --role-arn ${CLOUDFORMATION_ROLE_ARN} \
    --no-fail-on-empty-changeset

echo "Deploying the stack ForgeBuckets on the ${DEFAULT_AWS_REGION} region"

aws --region ${DEFAULT_AWS_REGION} cloudformation deploy \
    --template-file templates/ForgeBuckets.yaml \
    --stack-name ForgeBuckets \
    --capabilities CAPABILITY_NAMED_IAM \
    --role-arn ${CLOUDFORMATION_ROLE_ARN} \
    --no-fail-on-empty-changeset \
    --parameter-overrides \
        ForgeDomainName=${FORGE_DOMAIN_NAME}

cd ${SCRIPT_DIR}/..

goreleaser --rm-dist --snapshot

S3_BUCKET=$(aws cloudformation describe-stacks --stack-name ForgeBuckets | jq -r '.Stacks[0].Outputs | map(select(.OutputKey=="ArtifactsBucketName"))[0].OutputValue')

aws cloudformation package \
    --template-file=${SCRIPT_DIR}/templates/ForgeResources.yaml \
    --s3-bucket=${S3_BUCKET} \
    --s3-prefix=.forge/${VERSION} \
    --output-template=dist/packaged.yaml

aws cloudformation deploy \
    --template-file dist/packaged.yaml \
    --stack-name ForgeResources \
    --capabilities CAPABILITY_NAMED_IAM \
    --role-arn ${CLOUDFORMATION_ROLE_ARN} \
    --no-fail-on-empty-changeset \
    --parameter-overrides \
        Version=${VERSION}

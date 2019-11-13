#!/usr/bin/env bash

set -eu -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null && pwd)"
VERSION=$(git describe --match "v*" --dirty=--DIRTY-- --always | sed 's:^v\(.*\)$:\1:')

echo "Bootstrap in region: ${AWS_REGION}"
echo "Version: ${VERSION}"

CLOUDFORMATION_ROLE_ARN=$(aws --region us-east-1 cloudformation describe-stacks --stack-name ForgeBootstrap | jq -r '.Stacks[0].Outputs | map(select(.OutputKey=="BootstrapCloudFormationRoleArn"))[0].OutputValue')
FORGE_DOMAIN_NAME=$(aws --region us-east-1 cloudformation describe-stacks --stack-name ForgeBootstrap | jq -r '.Stacks[0].Outputs | map(select(.OutputKey=="ForgeDomainName"))[0].OutputValue')
PYTHON_LAMBDA_LAYER_NAME=ForgeResourcesLayer

function deployForgeIam() {
    echo "# Deploying ForgeIam in the ${1} region"
    cd ${SCRIPT_DIR}
    aws --region ${1} cloudformation deploy \
        --template-file templates/ForgeIam.yaml \
        --stack-name ForgeIam \
        --capabilities CAPABILITY_NAMED_IAM \
        --role-arn ${CLOUDFORMATION_ROLE_ARN} \
        --no-fail-on-empty-changeset
    echo ""
}

function deployForgeBuckets() {
    echo "# Deploying ForgeBuckets in the ${1} region"
    cd ${SCRIPT_DIR}
    aws --region ${1} cloudformation deploy \
        --template-file templates/ForgeBuckets.yaml \
        --stack-name ForgeBuckets \
        --capabilities CAPABILITY_NAMED_IAM \
        --role-arn ${CLOUDFORMATION_ROLE_ARN} \
        --no-fail-on-empty-changeset \
        --parameter-overrides \
        ForgeDomainName=${FORGE_DOMAIN_NAME}
    echo ""
}

function deployForgeResources() {
    echo "# Create Python lambda layer in the ${1} region"
    cd ${SCRIPT_DIR}/..
    S3_BUCKET=$(aws --region ${1} cloudformation describe-stacks --stack-name ForgeBuckets | jq -r '.Stacks[0].Outputs | map(select(.OutputKey=="ArtifactsBucketName"))[0].OutputValue')
    PIPLOCK_HASH=$(shasum -a 256 Pipfile.lock | cut -d " " -f 1)
    COMMON_CODE_HASH=$(find codesmith/common -type f -print0 | xargs -0 shasum -a 256 | sort -k 2 | shasum -a 256 | cut -d " " -f 1)
    PYTHON_LAMBDA_LAYER_HASH="${PIPLOCK_HASH}${COMMON_CODE_HASH}"
    echo "Python Layer Hash: ${PYTHON_LAMBDA_LAYER_HASH}"

    rm -fr dist
    mkdir dist

    PYTHON_LAMBDA_LAYER_KEY=.forgeResources/lambdaLayer/${PYTHON_LAMBDA_LAYER_HASH}
    set +e
    aws --region ${1} s3api head-object --bucket ${S3_BUCKET} --key ${PYTHON_LAMBDA_LAYER_KEY}
    LAMBDA_LAYER_EXISTS="$?"
    set -e
    echo $LAMBDA_LAYER_EXISTS
    if [[ "$LAMBDA_LAYER_EXISTS" == "0" ]]; then
        echo "# reusing the previous python layer"
        PYTHON_LAMBDA_LAYER_ARN=$(aws lambda --region ${1} list-layer-versions --layer-name ${PYTHON_LAMBDA_LAYER_NAME} | jq -r ".LayerVersions[0].LayerVersionArn")
    else
        mkdir -p dist/layer/python/codesmith/
        pipenv lock -r >dist/requirements.txt
        pipenv run pip install -t dist/layer/python -r dist/requirements.txt
        cp -R codesmith/common dist/layer/python/codesmith/

        cd dist/layer
        echo "# creating a new python layer"
        zip -r -X layer.zip .
        aws s3 cp layer.zip s3://${S3_BUCKET}/${PYTHON_LAMBDA_LAYER_KEY}
        PYTHON_LAMBDA_LAYER_ARN=$(aws --region ${1} lambda publish-layer-version --layer-name ${PYTHON_LAMBDA_LAYER_NAME} --content S3Bucket=${S3_BUCKET},S3Key=${PYTHON_LAMBDA_LAYER_KEY} --compatible-runtimes python3.7 | jq -r '.LayerVersionArn')
    fi

    echo "# Deploying ForgeResources in the ${1} region"
    cd ${SCRIPT_DIR}/..
    SSM_KMS_KEY_ARN=$(aws --region ${1} kms describe-key --key-id alias/aws/ssm | jq -r ".KeyMetadata.Arn")

    aws --region ${1} cloudformation package \
        --template-file=${SCRIPT_DIR}/templates/ForgeResources.yaml \
        --s3-bucket=${S3_BUCKET} \
        --s3-prefix=.forgeResources/${VERSION} \
        --output-template=dist/ForgeResources${VERSION}.yaml

    aws --region ${1} cloudformation deploy \
        --template-file dist/ForgeResources${VERSION}.yaml \
        --stack-name ForgeResources \
        --capabilities CAPABILITY_NAMED_IAM \
        --role-arn ${CLOUDFORMATION_ROLE_ARN} \
        --no-fail-on-empty-changeset \
        --parameter-overrides \
        Version=${VERSION} \
        SsmKmsKeyArn=${SSM_KMS_KEY_ARN} \
        PythonLambdaLayerArn=${PYTHON_LAMBDA_LAYER_ARN} \
        PythonLambdaLayerHash=${PYTHON_LAMBDA_LAYER_HASH}
}

function deployForgeLogsMaintenance() {
    echo "# Deploying ForgeLogsMaintenance in the ${1} region"
    cd ${SCRIPT_DIR}/..
    S3_BUCKET=$(aws --region ${1} cloudformation describe-stacks --stack-name ForgeBuckets | jq -r '.Stacks[0].Outputs | map(select(.OutputKey=="ArtifactsBucketName"))[0].OutputValue')

    aws --region ${1} cloudformation package \
        --template-file=${SCRIPT_DIR}/templates/ForgeLogsMaintenance.yaml \
        --s3-bucket=${S3_BUCKET} \
        --s3-prefix=.forgeLogsMaintenance/${VERSION} \
        --output-template=dist/ForgeLogsMaintenance${VERSION}.yaml

    aws --region ${1} cloudformation deploy \
        --template-file dist/ForgeLogsMaintenance${VERSION}.yaml \
        --stack-name ForgeLogsMaintenance \
        --capabilities CAPABILITY_NAMED_IAM \
        --role-arn ${CLOUDFORMATION_ROLE_ARN} \
        --no-fail-on-empty-changeset \
        --parameter-overrides \
        Version=${VERSION}
    echo ""
}

function main() {
    echo "# Deploying the forge in the us-east-1 region"

    deployForgeIam "us-east-1"
    deployForgeBuckets "us-east-1"
    deployForgeResources "us-east-1"
    deployForgeLogsMaintenance "us-east-1"

    if [[ "${AWS_REGION}" != "us-east-1" ]]; then
        echo "# Deploying the forge in the ${AWS_REGION} region"

        deployForgeIam "${AWS_REGION}"
        deployForgeBuckets "${AWS_REGION}"
        deployForgeResources "${AWS_REGION}"
    fi
}

function dev() {
    deployForgeResources "eu-central-1"
}

case $1 in
"main") main ;;
"dev") dev ;;
*)
    echo "Unknown command: ${1}"
    exit 1
    ;;
esac

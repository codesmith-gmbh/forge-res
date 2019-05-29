#!/usr/bin/env bash

DATABASE_INSTANCE_NAME=codesmith
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"
VERSION=$(git describe --match "v*" --dirty=--DIRTY-- --always | sed 's:^v\(.*\)$:\1:')

VPC_ID=$(aws ec2 describe-vpcs | jq -r '.Vpcs | map(select(.IsDefault == true))[0].VpcId')
SUBNET_IDS=$(aws ec2 describe-subnets --filters Name=vpc-id,Values=${VPC_ID} | jq -r '.Subnets | map(.SubnetId) | reduce .[1:][] as $i ("\(.[0])"; . + ",\($i)")')

echo "Database instance name: ${DATABASE_INSTANCE_NAME}"
echo "VPC Id: ${VPC_ID}"
echo "Subnet Ids: ${SUBNET_IDS}"
echo "Version: ${VERSION}"

CLOUDFORMATION_ROLE_ARN=$(aws --region us-east-1 cloudformation describe-stacks --stack-name ForgeBootstrap | jq -r '.Stacks[0].Outputs | map(select(.OutputKey=="BootstrapCloudFormationRoleArn"))[0].OutputValue')
FORGE_DOMAIN_NAME=$(aws --region us-east-1 cloudformation describe-stacks --stack-name ForgeBootstrap | jq -r '.Stacks[0].Outputs | map(select(.OutputKey=="ForgeDomainName"))[0].OutputValue')

function checkVPCEndpoints () {
    SERVICES_COUNT=$(aws ec2 describe-vpc-endpoints --filters Name=vpc-id,Values=${VPC_ID} | jq '.VpcEndpoints | map(select(.ServiceName=="com.amazonaws.eu-west-1.s3" or .ServiceName=="com.amazonaws.eu-west-1.secretsmanager")) | reduce .[] as $item (0; . + 1)' )
    if [[ "${SERVICES_COUNT}" == "2" ]]; then
        echo "Endpoints ok"
    else
        echo "Create Endpoints in your VPC for the following services:"
        echo " - com.amazonaws.eu-west-1.s3"
        echo " - com.amazonaws.eu-west-1.secretsmanager"
    fi
}

function goArtefacts () {
    cd ${SCRIPT_DIR}
    goreleaser --rm-dist --snapshot
    cp PostgresDatabase/rds-ca-2015-root.pem dist/linux_amd64/PostgresDatabase
}

function deployInstanceStack () {
    echo "# Deploying Instance Stack"
    cd ${SCRIPT_DIR}/..
    S3_BUCKET=$(aws cloudformation describe-stacks --stack-name ForgeBuckets | jq -r '.Stacks[0].Outputs | map(select(.OutputKey=="ArtifactsBucketName"))[0].OutputValue')

    aws cloudformation package \
        --template-file=${SCRIPT_DIR}/postgresInstance.yaml \
        --s3-bucket=${S3_BUCKET} \
        --s3-prefix=.postgresInstance/${DATABASE_INSTANCE_NAME}/${VERSION} \
        --output-template=dist/postgresInstance${VERSION}.yaml

    aws cloudformation deploy \
        --template-file dist/postgresInstance${VERSION}.yaml \
        --stack-name PostgresInstance-${DATABASE_INSTANCE_NAME} \
        --capabilities CAPABILITY_NAMED_IAM \
        --no-fail-on-empty-changeset \
        --parameter-overrides \
            Version=${VERSION} \
            DBInstanceName=${DATABASE_INSTANCE_NAME} \
            VpcId=${VPC_ID} \
            SubnetIds=${SUBNET_IDS}
    echo ""
}

function main () {
    echo "# Check S3 and SecretManager"

    checkVPCEndpoints

    echo "# Building go artefacts"

    goArtefacts

    echo "# Deploying the instance stack"

    deployInstanceStack
}

function dev () {
    #goArtefacts
    deployInstanceStack
}

case $1 in
	"main") main;;
	"dev") dev;;
	*)
		echo "Unknown command: ${1}"
		exit 1
		;;
esac
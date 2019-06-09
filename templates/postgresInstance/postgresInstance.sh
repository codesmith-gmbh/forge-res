#!/usr/bin/env bash

DATABASE_INSTANCE_IDENTIFIER=codesmith
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"
VERSION=$(git describe --match "v*" --dirty=--DIRTY-- --always | sed 's:^v\(.*\)$:\1:')

VPC_ID=$(aws ec2 describe-vpcs | jq -r '.Vpcs | map(select(.IsDefault == true))[0].VpcId')
SUBNET_IDS=$(aws ec2 describe-subnets --filters Name=vpc-id,Values=${VPC_ID} | jq -r '.Subnets | map(.SubnetId) | reduce .[1:][] as $i ("\(.[0])"; . + ",\($i)")')

echo "Database instance identifier: ${DATABASE_INSTANCE_IDENTIFIER}"
echo "VPC Id: ${VPC_ID}"
echo "Subnet Ids: ${SUBNET_IDS}"
echo "Version: ${VERSION}"

CLOUDFORMATION_ROLE_ARN=$(aws --region us-east-1 cloudformation describe-stacks --stack-name ForgeBootstrap | jq -r '.Stacks[0].Outputs | map(select(.OutputKey=="BootstrapCloudFormationRoleArn"))[0].OutputValue')
FORGE_DOMAIN_NAME=$(aws --region us-east-1 cloudformation describe-stacks --stack-name ForgeBootstrap | jq -r '.Stacks[0].Outputs | map(select(.OutputKey=="ForgeDomainName"))[0].OutputValue')

function goArtefacts () {
    echo "# Building go artefacts"
    cd ${SCRIPT_DIR}
    goreleaser --rm-dist --snapshot
}

function deployInstanceStack () {
    cd ${SCRIPT_DIR}
    echo "# Deploying Instance Stack"
    S3_BUCKET=$(aws cloudformation describe-stacks --stack-name ForgeBuckets | jq -r '.Stacks[0].Outputs | map(select(.OutputKey=="ArtifactsBucketName"))[0].OutputValue')

    aws cloudformation package \
        --template-file=postgresInstance.yaml \
        --s3-bucket=${S3_BUCKET} \
        --s3-prefix=.postgresInstance/${DATABASE_INSTANCE_IDENTIFIER}/${VERSION} \
        --output-template=dist/postgresInstance${VERSION}.yaml

    aws cloudformation deploy \
        --template-file dist/postgresInstance${VERSION}.yaml \
        --stack-name PostgresInstance-${DATABASE_INSTANCE_IDENTIFIER} \
        --capabilities CAPABILITY_NAMED_IAM \
        --no-fail-on-empty-changeset \
        --parameter-overrides \
            Version=${VERSION} \
            DbInstanceIdentifier=${DATABASE_INSTANCE_IDENTIFIER} \
            VpcId=${VPC_ID} \
            SubnetIds=${SUBNET_IDS}
    echo ""
}

function main () {
    goArtefacts
    deployInstanceStack
}

function dev () {
    goArtefacts
}

case $1 in
	"main") main;;
	"dev") dev;;
	*)
		echo "Unknown command: ${1}"
		exit 1
		;;
esac
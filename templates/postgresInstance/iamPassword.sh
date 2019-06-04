#!/usr/bin/env bash

DB_INSTANCE_IDENTIFIER=${1}
USERNAME=${2}



DB_INSTANCE=$(aws rds describe-db-instances --filter Name=db-instance-id,Values=${DB_INSTANCE_IDENTIFIER})
HOST=$(echo "${DB_INSTANCE}" | jq -r ".DBInstances[0].Endpoint.Address")
PORT=$(echo "${DB_INSTANCE}" | jq -r ".DBInstances[0].Endpoint.Port")


echo "DB_INSTANCE_IDENTIFIER=${DB_INSTANCE_IDENTIFIER}"
echo "HOST=${HOST}"
echo "PORT=${PORT}"
echo "USERNAME=${USERNAME}"

aws rds generate-db-auth-token --hostname ${HOST} --port ${PORT} --username ${USERNAME} | tr -d '\n' | pbcopy

echo "Password in clipboard"

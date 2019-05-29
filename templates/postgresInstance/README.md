# Postgres Instance template

The Postgres Instance template is a collection of files to create an RDS instance. You shoud copy it, edit it and
check it in a separate repository.

It also install a cloudformation custom resource to manage individual databases on the instance itself.

Here is the list of configurable options:

- Instance Identifier: the Instance Identifier is configured in the `postgresInstance.sh` shell script via
  the env var `DATABASE_INSTANCE_NAME`
- VPC: the VPC for the RDS instance is configured in the `postgresInstance.sh` shell script via the env var `VPC_ID`.
  Per default, the RDS is created in the default VPC.
- Storage/Machine/etc.. : in the template `postgresInstance.yaml` on the resource `Instance` directly.

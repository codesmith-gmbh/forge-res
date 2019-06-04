# PostgresDatabase

The PostgresDatabase resource is used to create/delete a database on a PostgreSQL instance/cluster. Together with
the database a new admin user is created for the database, it has the same name as the database. Its password is
stored in an encrypted SSM parameter.

To access the database, this procedure reads the password from an encrypted SSM parameter (that
have been used to create the PostgreSQL instance).

WARNING: The database is *always* deleted when the resource is deleted. To avoid loss of data, the resource will
kick off a snapshot the instance.
It is *strongly* recommended to use `Retain` for `DeletionPolicy` to avoid loss of data.

## Syntax

To create an `PostgresDatabase` resource, add the following resource to your cloudformation
template (yaml notation, json is similar)

```yaml
MyPostgresDatabase:
  Type: Custom::PostgresDatabase
  DeletionPolicy: Retain
  Properties:
    ServiceToken: !ImportValue <instance stack name>-PostgresDatabase
    DatabaseName: <database name>
```

## Properties

`DatabaseName`

> The name of the database to create on the instance. The resource also creates an admin user with the same name.
> The user logs in via the IAM login method as explained in this AWS article: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAM.html
>
> _Type_: String
> 
> _Required_: Yes
>
> _Update Requires_: no interruption


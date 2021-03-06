# S3 Cleanup: custom CloudFormation resource

**WARNING**: Use the S3ReleaseCleanup to regularly cleanup resources; this resource is meant
to be used when deleting an application.

The `s3Cleanup` CloudFormation custom resource cleans up S3 buckets.

The resource either deletes all objects of an S3 buckets (useful when deleting a stack containing an S3
Bucket that should be deleted as well) or it deletes objects under a given prefix (useful when deleting
a stack that uses an S3 bucket shared among many stacks).

The `s3Cleanup` custom resource only deletes objects
when the stack itself is being deleted. It also safe to remove the resource from an existing stack.

## Syntax

To create an `s3Cleanup` resource, add the following resource to your cloudformation
template (yaml notation, json is similar)

```yaml
MyS3Cleanup:
  Type: Custom::S3Cleanup
  Properties:
    ServiceToken: !ImportValue ForgeResources-S3Cleanup
    Bucket: <bucket name>
    Prefix: <prefix>
```

## Properties

`Bucket`

> The name of the S3 Bucket to cleanup when the `s3Cleanup` resource is deleted while its stack
> itself is deleted.
>
> _Type_: Bucket Name
>
> _Required_: Yes
>
> _Update Requires_: replacement

`Prefix`

> A prefix to delete objects. If the prefix is omitted or is empty, then all objects are deleted.
>
> _Type_: String
>
> _Required_: No
>
> _Update Requires_: replacement
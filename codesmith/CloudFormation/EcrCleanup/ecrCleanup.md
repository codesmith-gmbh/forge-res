# ECR Cleanup

The `ecrCleanup` custom resource cleans up ECR repositories.

The AWS ECR resource cannot be deleted if images still exsists. If you integrate an `ecrCleanup` custom resource
in the same stack as an AWS ECR resouce and letting it depend on the AWS ECR resource, then, on stack deletion,
the `ecrCleanup` resource will delete all images before the AWS ECR itself can be deleted.

It is not dangerous to delete the resource itself when updating the stack as the `ecrCleanup` custom resource only
cleanup the content when the stack itself is getting deleted.

## Syntax

To create an ecrCleanup resource, add the following resource to your cloudformation
template (yaml notation, json is similar)

```yaml
MyEcrCleanup:
  Type: Custom::EcrCleanup
  Properties:
    ServiceToken: !ImportValue ForgeResources-EcrCleanup
    Repository: !Ref Repository
```

## Properties

`Repository`

> The name of the repository to clean when the resource is deleted while its stack
> itself is deleted
>
> _Type_: Repository Name
>
> _Required_: Yes
>
> _Update Requires_: no interruption
